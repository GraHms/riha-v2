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
	Partner     string `gorm:"size:120"`
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
	return []any{&integrationRecord{}, &merchantRouteRecord{}, &merchantDealRecord{}, &partnerSettlementRecord{}}
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
	err := r.conn(ctx).Model(&payoutRecord{}).Where("livemode AND status = ?", domain.PayoutPaid).Select("COALESCE(SUM(amount),0)").Scan(&m.PaidOut).Error
	return m, err
}

func (r Integrations) Unpriced(ctx context.Context, limit int) ([]domain.Payment, error) {
	return find[paymentRecord, domain.Payment](r.conn(ctx).Where("NOT priced AND integration <> ''").Order("created_at").Limit(limit))
}

func (r Integrations) SetPrice(ctx context.Context, paymentID string, p domain.Price) error {
	return r.conn(ctx).Model(&paymentRecord{}).Where("id = ?", paymentID).
		Updates(map[string]any{"platform_fee": p.PlatformFee, "partner_fee": p.PartnerFee, "priced": true}).Error
}
