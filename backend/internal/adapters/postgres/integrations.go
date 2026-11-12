package postgres

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm/clause"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type IntMap map[string]int

func (m IntMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]int(m))
	return string(b), err
}

func (m *IntMap) Scan(v any) error {
	out := map[string]int{}
	if err := scanJSON(v, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

type integrationRecord struct {
	Code        string `gorm:"primaryKey;size:30"`
	Name        string `gorm:"size:80"`
	PartnerCode string `gorm:"size:30;not null;default:'';index"`
	Account     string `gorm:"size:200"`
	PlatformBps IntMap `gorm:"type:text"`
	ShareBps    int
	Note        string `gorm:"size:500"`
	UpdatedAt   time.Time
}

func (integrationRecord) TableName() string { return "integrations" }

type merchantRouteRecord struct {
	MerchantID  string `gorm:"primaryKey;size:40"`
	Method      string `gorm:"primaryKey;size:10"`
	Integration string `gorm:"size:30;index"`
	UpdatedBy   string `gorm:"size:40"`
	UpdatedAt   time.Time
}

func (merchantRouteRecord) TableName() string { return "merchant_routes" }

type merchantDealRecord struct {
	MerchantID  string `gorm:"primaryKey;size:40"`
	Integration string `gorm:"primaryKey;size:30"`
	ShareBps    int
	UpdatedBy   string `gorm:"size:40"`
	UpdatedAt   time.Time
}

func (merchantDealRecord) TableName() string { return "merchant_deals" }

type partnerSettlementRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	Integration string `gorm:"size:30;index;not null"`
	Amount      int64
	Reference   string    `gorm:"size:120"`
	Note        string    `gorm:"size:500"`
	ReceivedOn  time.Time `gorm:"index"`
	CreatedBy   string    `gorm:"size:40"`
	CreatedAt   time.Time
}

func (partnerSettlementRecord) TableName() string { return "partner_settlements" }

func integrationRecords() []any {
	return []any{&integrationRecord{}, &merchantRouteRecord{}, &merchantDealRecord{}, &partnerSettlementRecord{}, &partnerRecord{}, &payoutFundingRecord{}}
}

// tagPaymentsByIntegration names the integration of the payments from before
// payments recorded it. mtoca's operation numbers are 12 characters and the
// M-Pesa API's references 20, so a live M-Pesa payment with a 20-character
// reference went direct, through 900511. Test payments were simulated, except
// those mtoca's dev API took.
const tagPaymentsByIntegration = `UPDATE payments SET integration = CASE
	WHEN method IN ('mpesa','emola','bim') AND length(rail_reference) = 12 THEN 'mtoca'
	WHEN livemode AND method = 'mpesa' AND length(rail_reference) = 20 THEN 'mpesa_direct'
	WHEN NOT livemode THEN 'simulator'
	ELSE integration END
WHERE integration = ''`

type Integrations struct{ *Store }

func (r Integrations) List(ctx context.Context) ([]domain.Integration, error) {
	return find[integrationRecord, domain.Integration](r.conn(ctx).Order("code"))
}

func (r Integrations) Get(ctx context.Context, code string) (*domain.Integration, error) {
	return first[integrationRecord, domain.Integration](r.conn(ctx).Where("code = ?", code))
}

func (r Integrations) Save(ctx context.Context, i *domain.Integration) error {
	return r.conn(ctx).Save(toRecord[integrationRecord](i)).Error
}

func (r Integrations) AddMissing(ctx context.Context, is []domain.Integration) error {
	for i := range is {
		if err := r.conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(toRecord[integrationRecord](&is[i])).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r Integrations) Routes(ctx context.Context, merchantID string) ([]domain.MerchantRoute, error) {
	db := r.conn(ctx).Order("merchant_id, method")
	if merchantID != "" {
		db = db.Where("merchant_id = ?", merchantID)
	}
	return find[merchantRouteRecord, domain.MerchantRoute](db)
}

func (r Integrations) SetRoute(ctx context.Context, x *domain.MerchantRoute) error {
	return r.conn(ctx).Save(toRecord[merchantRouteRecord](x)).Error
}

func (r Integrations) DeleteRoute(ctx context.Context, merchantID, method string) error {
	return r.conn(ctx).Where("merchant_id = ? AND method = ?", merchantID, method).Delete(&merchantRouteRecord{}).Error
}

func (r Integrations) Deals(ctx context.Context, merchantID string) ([]domain.MerchantDeal, error) {
	db := r.conn(ctx).Order("merchant_id, integration")
	if merchantID != "" {
		db = db.Where("merchant_id = ?", merchantID)
	}
	return find[merchantDealRecord, domain.MerchantDeal](db)
}

func (r Integrations) SetDeal(ctx context.Context, x *domain.MerchantDeal) error {
	return r.conn(ctx).Save(toRecord[merchantDealRecord](x)).Error
}

func (r Integrations) DeleteDeal(ctx context.Context, merchantID, integration string) error {
	return r.conn(ctx).Where("merchant_id = ? AND integration = ?", merchantID, integration).Delete(&merchantDealRecord{}).Error
}

func (r Integrations) AddSettlement(ctx context.Context, s *domain.PartnerSettlement) error {
	return create[partnerSettlementRecord](r.conn(ctx), s)
}

func (r Integrations) Settlement(ctx context.Context, id string) (*domain.PartnerSettlement, error) {
	return first[partnerSettlementRecord, domain.PartnerSettlement](r.conn(ctx).Where("id = ?", id))
}

func (r Integrations) Settlements(ctx context.Context, integration string, limit int) ([]domain.PartnerSettlement, error) {
	db := r.conn(ctx).Order("received_on DESC, created_at DESC")
	if integration != "" {
		db = db.Where("integration = ?", integration)
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	return find[partnerSettlementRecord, domain.PartnerSettlement](db)
}

func (r Integrations) DeleteSettlement(ctx context.Context, id string) error {
	return r.conn(ctx).Where("id = ?", id).Delete(&partnerSettlementRecord{}).Error
}

func (r Integrations) SettledTotals(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Integration string
		Total       int64
	}
	err := r.conn(ctx).Model(&partnerSettlementRecord{}).Select("integration, COALESCE(SUM(amount),0) AS total").Group("integration").Scan(&rows).Error
	out := map[string]int64{}
	for _, x := range rows {
		out[x.Integration] = x.Total
	}
	return out, err
}

func (r Integrations) Collected(ctx context.Context, from, to *time.Time) ([]ports.CollectedRow, error) {
	db := r.conn(ctx).Model(&paymentRecord{}).Where("livemode AND status IN ?", domain.SettledStatuses)
	if from != nil {
		db = db.Where("succeeded_at >= ?", *from)
	}
	if to != nil {
		db = db.Where("succeeded_at < ?", *to)
	}
	var rows []ports.CollectedRow
	err := db.Select(`integration, merchant_id, COUNT(*) AS count, COALESCE(SUM(amount),0) AS amount, COALESCE(SUM(fee),0) AS fee,
		COALESCE(SUM(platform_fee),0) AS platform_fee, COALESCE(SUM(partner_fee),0) AS partner_fee, COALESCE(SUM(amount_refunded),0) AS refunded`).
		Group("integration, merchant_id").Order("integration, amount DESC").Scan(&rows).Error
	return rows, err
}

func (r Integrations) MerchantMoney(ctx context.Context) (ports.MerchantMoney, error) {
	var m ports.MerchantMoney
	if err := r.conn(ctx).Model(&ledgerRecord{}).Where("livemode").Select("COALESCE(SUM(amount),0)").Scan(&m.Balances).Error; err != nil {
		return m, err
	}
	if err := r.conn(ctx).Model(&payoutRecord{}).Where("livemode AND status IN ?", domain.OpenPayoutStatuses).Select("COALESCE(SUM(amount),0)").Scan(&m.InTransit).Error; err != nil {
		return m, err
	}
	err := r.conn(ctx).Raw(`SELECT COALESCE((SELECT SUM(amount) FROM payouts WHERE livemode AND status = ? AND NOT EXISTS (SELECT 1 FROM payout_funding f WHERE f.payout_id = payouts.id)),0)
		+ COALESCE((SELECT SUM(amount) FROM payout_funding WHERE source = ? AND status = ?),0)`, domain.PayoutPaid, domain.FundRIHA, domain.FundingPaid).Scan(&m.PaidOut).Error
	return m, err
}

func (r Integrations) Unpriced(ctx context.Context, limit int) ([]domain.Payment, error) {
	return find[paymentRecord, domain.Payment](r.conn(ctx).Where("NOT priced AND integration <> ''").Order("created_at").Limit(limit))
}

func (r Integrations) SetPrice(ctx context.Context, paymentID string, p domain.Price) error {
	return r.conn(ctx).Model(&paymentRecord{}).Where("id = ?", paymentID).
		Updates(map[string]any{"platform_fee": p.PlatformFee, "partner_fee": p.PartnerFee, "priced": true}).Error
}

type partnerRecord struct {
	Code      string `gorm:"primaryKey;size:30"`
	Name      string `gorm:"size:120;not null"`
	Email     string `gorm:"size:200"`
	CreatedAt time.Time
}

func (partnerRecord) TableName() string { return "partners" }

type payoutFundingRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	PayoutID    string `gorm:"size:40;index;not null"`
	MerchantID  string `gorm:"size:40;index;not null"`
	Source      string `gorm:"size:30;index;not null"`
	Amount      int64
	Send        int64
	Status      string `gorm:"size:20;index"`
	Reference   string `gorm:"size:120"`
	ReceiptPath string `gorm:"size:500"`
	ReceiptName string `gorm:"size:200"`
	ReceiptType string `gorm:"size:60"`
	ReceiptSize int64
	ReceiptHash string `gorm:"size:64"`
	PaidAt      *time.Time
	PaidBy      string `gorm:"size:40"`
	CreatedAt   time.Time
}

func (payoutFundingRecord) TableName() string { return "payout_funding" }

func (r Integrations) Partners(ctx context.Context) ([]domain.Partner, error) {
	return find[partnerRecord, domain.Partner](r.conn(ctx).Order("name"))
}

func (r Integrations) Partner(ctx context.Context, code string) (*domain.Partner, error) {
	return first[partnerRecord, domain.Partner](r.conn(ctx).Where("code = ?", code))
}

func (r Integrations) CreatePartner(ctx context.Context, p *domain.Partner) error {
	return create[partnerRecord](r.conn(ctx), p)
}

func (r Integrations) SavePartner(ctx context.Context, p *domain.Partner) error {
	return r.conn(ctx).Omit("created_at").Save(toRecord[partnerRecord](p)).Error
}

func (r Integrations) AddMissingPartners(ctx context.Context, ps []domain.Partner) error {
	for i := range ps {
		if err := r.conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(toRecord[partnerRecord](&ps[i])).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r Integrations) CreateFunding(ctx context.Context, f *domain.PayoutFunding) error {
	return create[payoutFundingRecord](r.conn(ctx), f)
}

func (r Integrations) SaveFunding(ctx context.Context, f *domain.PayoutFunding) error {
	return save[payoutFundingRecord](r.conn(ctx), f)
}

func (r Integrations) LockFunding(ctx context.Context, id string) (*domain.PayoutFunding, error) {
	return first[payoutFundingRecord, domain.PayoutFunding](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}

func (r Integrations) FundingOf(ctx context.Context, payoutIDs []string) ([]domain.PayoutFunding, error) {
	if len(payoutIDs) == 0 {
		return nil, nil
	}
	return find[payoutFundingRecord, domain.PayoutFunding](r.conn(ctx).Where("payout_id IN ?", payoutIDs).Order("created_at, amount DESC"))
}

func (r Integrations) FundingFrom(ctx context.Context, sources, statuses []string, limit int) ([]domain.PayoutFunding, error) {
	db := r.conn(ctx).Where("source IN ?", sources).Order("created_at DESC")
	if len(statuses) > 0 {
		db = db.Where("status IN ?", statuses)
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	return find[payoutFundingRecord, domain.PayoutFunding](db)
}

// HeldFor is what each integration still holds of one merchant's live money:
// what its paid payments left the merchant, less refunds and the withdrawals it
// was given to pay.
func (r Integrations) HeldFor(ctx context.Context, merchantID string) (map[string]int64, error) {
	var rows []struct {
		Source string
		Total  int64
	}
	err := r.conn(ctx).Raw(`SELECT source, SUM(total) AS total FROM (
		SELECT integration AS source, COALESCE(SUM(net - amount_refunded),0) AS total FROM payments
			WHERE merchant_id = ? AND livemode AND status IN ? AND integration <> '' GROUP BY integration
		UNION ALL
		SELECT source, -COALESCE(SUM(amount),0) FROM payout_funding WHERE merchant_id = ? AND status <> ? GROUP BY source
	) t GROUP BY source`, merchantID, domain.SettledStatuses, merchantID, domain.FundingCancelled).Scan(&rows).Error
	out := map[string]int64{}
	for _, x := range rows {
		out[x.Source] = x.Total
	}
	return out, err
}

// FundingTotals sums withdrawal parts by source and status.
func (r Integrations) FundingTotals(ctx context.Context) (map[string]map[string]int64, error) {
	var rows []struct {
		Source, Status string
		Total          int64
	}
	err := r.conn(ctx).Model(&payoutFundingRecord{}).Select("source, status, COALESCE(SUM(amount),0) AS total").Group("source, status").Scan(&rows).Error
	out := map[string]map[string]int64{}
	for _, x := range rows {
		if out[x.Source] == nil {
			out[x.Source] = map[string]int64{}
		}
		out[x.Source][x.Status] = x.Total
	}
	return out, err
}

// UnfundedPayouts are live withdrawals, in these statuses, not yet split by source.
func (r Integrations) UnfundedPayouts(ctx context.Context, statuses []string) ([]domain.Payout, error) {
	return find[payoutRecord, domain.Payout](r.conn(ctx).Where("livemode AND status IN ? AND NOT EXISTS (SELECT 1 FROM payout_funding f WHERE f.payout_id = payouts.id)", statuses).Order("requested_at"))
}

func (r Integrations) CollectedDaily(ctx context.Context, integrations []string, from, to time.Time) ([]ports.DayRow, error) {
	var rows []ports.DayRow
	err := r.conn(ctx).Model(&paymentRecord{}).
		Where("livemode AND status IN ? AND integration IN ? AND succeeded_at >= ? AND succeeded_at < ?", domain.SettledStatuses, integrations, from, to).
		Select(`to_char(succeeded_at AT TIME ZONE 'Africa/Maputo', 'YYYY-MM-DD') AS day, COUNT(*) AS count, COALESCE(SUM(amount),0) AS amount, COALESCE(SUM(partner_fee),0) AS partner_fee`).
		Group("day").Order("day").Scan(&rows).Error
	return rows, err
}
