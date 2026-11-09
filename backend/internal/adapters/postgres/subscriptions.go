package postgres

import (
	"context"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type planRecord struct {
	ID           string `gorm:"primaryKey;size:40"`
	MerchantID   string `gorm:"size:40;not null;uniqueIndex:plan_slug"`
	Livemode     bool   `gorm:"index;uniqueIndex:plan_slug"`
	Slug         string `gorm:"size:80;uniqueIndex:plan_slug"`
	Name         string `gorm:"size:120"`
	Status       string `gorm:"size:20;index"`
	Interval     string `gorm:"size:10"`
	Anchor       string `gorm:"size:10"`
	BillingDay   int
	Months       int
	Missed       string     `gorm:"size:10"`
	Fields       StringList `gorm:"type:jsonb"`
	Highlights   StringList `gorm:"type:jsonb"`
	Cycles       int
	EndsAt       *time.Time
	Amount       int64
	Version      int
	InCatalog    bool
	CatalogOrder int
	CreatedBy    string `gorm:"size:40"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (planRecord) TableName() string { return "plans" }

type planVersionRecord struct {
	ID        string `gorm:"primaryKey;size:40"`
	PlanID    string `gorm:"size:40;uniqueIndex:plan_version"`
	Version   int    `gorm:"uniqueIndex:plan_version"`
	Amount    int64
	Scope     string `gorm:"size:20"`
	CreatedBy string `gorm:"size:40"`
	CreatedAt time.Time
}

func (planVersionRecord) TableName() string { return "plan_versions" }

type memberRecord struct {
	ID          string    `gorm:"primaryKey;size:40"`
	MerchantID  string    `gorm:"size:40;not null;index:member_identity"`
	Livemode    bool      `gorm:"index:member_identity"`
	Name        string    `gorm:"size:200"`
	NameKey     string    `gorm:"size:200;index:member_identity"`
	PayerName   string    `gorm:"size:200"`
	Phone       string    `gorm:"size:20;index:member_identity"`
	Email       string    `gorm:"size:200"`
	Channel     string    `gorm:"size:10"`
	Fields      StringMap `gorm:"type:jsonb"`
	Status      string    `gorm:"size:20;index"`
	Debt        int64
	DebtCount   int
	NextDueAt   *time.Time
	LastPaidAt  *time.Time
	LastPaidVia string `gorm:"size:40"`
	PlanNames   string `gorm:"size:400"`
	LastMethod  string `gorm:"size:10"`
	LastPhone   string `gorm:"size:20"`
	Token       string `gorm:"size:40;uniqueIndex"`
	CreatedVia  string `gorm:"size:20"`
	CreatedBy   string `gorm:"size:40"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (memberRecord) TableName() string { return "members" }

type subscriptionRecord struct {
	ID             string `gorm:"primaryKey;size:40"`
	MerchantID     string `gorm:"size:40;index;not null"`
	Livemode       bool   `gorm:"index"`
	MemberID       string `gorm:"size:40;index"`
	PlanID         string `gorm:"size:40;index"`
	PlanVersion    int
	Amount         int64
	PendingAmount  int64
	PendingVersion int
	PendingFrom    *time.Time
	Status         string `gorm:"size:20;index"`
	AnchorAt       time.Time
	NextDueAt      *time.Time `gorm:"index"`
	PausedUntil    *time.Time
	SkipPeriods    StringList `gorm:"type:jsonb"`
	ConsentText    string     `gorm:"size:1000"`
	ConsentAt      *time.Time
	ConsentIP      string `gorm:"size:64"`
	CreatedVia     string `gorm:"size:20"`
	CreatedBy      string `gorm:"size:40"`
	ActivatedAt    *time.Time
	CanceledAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (subscriptionRecord) TableName() string { return "subscriptions" }

type chargeRecord struct {
	ID               string    `gorm:"primaryKey;size:40"`
	MerchantID       string    `gorm:"size:40;index;not null"`
	Livemode         bool      `gorm:"index"`
	SubscriptionID   string    `gorm:"size:40;uniqueIndex:charge_period"`
	MemberID         string    `gorm:"size:40;index"`
	PlanID           string    `gorm:"size:40;index"`
	Period           string    `gorm:"size:10;uniqueIndex:charge_period"`
	Label            string    `gorm:"size:60"`
	DueAt            time.Time `gorm:"index"`
	Amount           int64
	Status           string `gorm:"size:20;index"`
	First            bool
	Token            string  `gorm:"size:40;uniqueIndex"`
	SessionID        *string `gorm:"size:60;index"`
	PaymentID        *string `gorm:"size:40"`
	PaidAt           *time.Time
	Method           string     `gorm:"size:10"`
	OutsideMethod    string     `gorm:"size:20"`
	OutsideRef       string     `gorm:"size:100"`
	OutsideProofPath string     `gorm:"size:500"`
	OutsideProofName string     `gorm:"size:200"`
	OutsideProofHash string     `gorm:"size:64"`
	MarkedBy         string     `gorm:"size:200"`
	Steps            StringList `gorm:"type:jsonb"`
	Resends          int
	LastFailure      string `gorm:"size:200"`
	LinkSentAt       *time.Time
	OpenedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (chargeRecord) TableName() string { return "subscription_charges" }

type messageRecord struct {
	ID            string `gorm:"primaryKey;size:40"`
	MerchantID    string `gorm:"size:40;index"`
	Livemode      bool
	MemberID      string `gorm:"size:40;index"`
	ChargeID      string `gorm:"size:40"`
	Channel       string `gorm:"size:10"`
	To            string `gorm:"column:recipient;size:20"`
	Sender        string `gorm:"size:20"`
	Kind          string `gorm:"size:20"`
	Body          string `gorm:"size:1000"`
	Status        string `gorm:"size:10;index"`
	Error         string `gorm:"size:200"`
	ProviderID    string `gorm:"size:100;index"`
	Attempts      int
	NextAttemptAt *time.Time
	SentAt        *time.Time
	DeliveredAt   *time.Time
	CreatedAt     time.Time `gorm:"index"`
}

func (messageRecord) TableName() string { return "messages" }

type subSettingsRecord struct {
	MerchantID    string     `gorm:"primaryKey;size:40"`
	DisabledSteps StringList `gorm:"type:jsonb"`
	Channels      StringList `gorm:"type:jsonb"`
	SenderID      string     `gorm:"size:20"`
	ToleranceDays int
	FinalAction   string `gorm:"size:10"`
	RequireProof  bool
	ReceiptSMS    bool `gorm:"column:receipt_sms"`
	CatalogMulti  bool
	// Unique when set: see Store.Migrate.
	CatalogSlug        string `gorm:"size:40"`
	CatalogTitle       string `gorm:"size:80"`
	CatalogDescription string `gorm:"size:400"`
	CatalogCover       string `gorm:"size:300"`
	CatalogAccent      string `gorm:"size:7"`
	CatalogBackground  string `gorm:"size:7"`
	UpdatedAt          time.Time
}

func (subSettingsRecord) TableName() string { return "subscription_settings" }

func subscriptionRecords() []any {
	return []any{&planRecord{}, &planVersionRecord{}, &memberRecord{}, &subscriptionRecord{}, &chargeRecord{}, &messageRecord{}, &subSettingsRecord{}}
}

type Plans struct{ *Store }

func (r Plans) Create(ctx context.Context, p *domain.Plan) error {
	return create[planRecord](r.conn(ctx), p)
}
func (r Plans) Save(ctx context.Context, p *domain.Plan) error {
	return save[planRecord](r.conn(ctx), p)
}
func (r Plans) ByID(ctx context.Context, merchantID, id string) (*domain.Plan, error) {
	return first[planRecord, domain.Plan](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}
func (r Plans) Get(ctx context.Context, id string) (*domain.Plan, error) {
	return first[planRecord, domain.Plan](r.conn(ctx).Where("id = ?", id))
}
func (r Plans) BySlug(ctx context.Context, merchantID, slug string) (*domain.Plan, error) {
	return first[planRecord, domain.Plan](r.conn(ctx).Where("merchant_id = ? AND slug = ? AND status = ?", merchantID, slug, domain.PlanActive).Order("livemode DESC"))
}
func (r Plans) SlugTaken(ctx context.Context, merchantID, slug string) (bool, error) {
	return exists(r.conn(ctx).Model(&planRecord{}).Where("merchant_id = ? AND slug = ?", merchantID, slug))
}
func (r Plans) List(ctx context.Context, merchantID string, live bool) ([]domain.Plan, error) {
	return find[planRecord, domain.Plan](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status <> ?", merchantID, live, domain.PlanArchived).Order("catalog_order, created_at"))
}

// CatalogMerchants lists the merchants with a live plan in their catalog.
func (r Plans) CatalogMerchants(ctx context.Context, limit int) ([]string, error) {
	var ids []string
	err := r.conn(ctx).Model(&planRecord{}).Distinct("merchant_id").Where("livemode AND in_catalog AND status = ?", domain.PlanActive).Limit(limit).Pluck("merchant_id", &ids).Error
	return ids, err
}
func (r Plans) AddVersion(ctx context.Context, v *domain.PlanVersion) error {
	return create[planVersionRecord](r.conn(ctx), v)
}
func (r Plans) Versions(ctx context.Context, planID string) ([]domain.PlanVersion, error) {
	return find[planVersionRecord, domain.PlanVersion](r.conn(ctx).Where("plan_id = ?", planID).Order("version DESC"))
}

type Members struct{ *Store }

func (r Members) Create(ctx context.Context, m *domain.Member) error {
	return create[memberRecord](r.conn(ctx), m)
}
func (r Members) Save(ctx context.Context, m *domain.Member) error {
	return save[memberRecord](r.conn(ctx), m)
}
func (r Members) ByID(ctx context.Context, merchantID, id string) (*domain.Member, error) {
	return first[memberRecord, domain.Member](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}
func (r Members) Get(ctx context.Context, id string) (*domain.Member, error) {
	return first[memberRecord, domain.Member](r.conn(ctx).Where("id = ?", id))
}
func (r Members) Lock(ctx context.Context, id string) (*domain.Member, error) {
	return first[memberRecord, domain.Member](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Members) Identity(ctx context.Context, merchantID string, live bool, phone, nameKey string) (*domain.Member, error) {
	return first[memberRecord, domain.Member](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND phone = ? AND name_key = ?", merchantID, live, phone, nameKey))
}
func (r Members) query(ctx context.Context, q ports.MemberQuery) *gormDB {
	db := r.conn(ctx).Model(&memberRecord{}).Where("merchant_id = ? AND livemode = ?", q.MerchantID, q.Live)
	if len(q.Statuses) > 0 {
		db = db.Where("status IN ?", q.Statuses)
	} else {
		db = db.Where("status NOT IN ?", []string{domain.SubIncomplete})
	}
	if q.PlanID != "" {
		db = db.Where("id IN (?)", r.conn(ctx).Model(&subscriptionRecord{}).Select("member_id").Where("plan_id = ? AND status <> ?", q.PlanID, domain.SubIncomplete))
	}
	if q.FieldKey != "" && q.FieldValue != "" {
		db = db.Where("fields ->> ? = ?", q.FieldKey, q.FieldValue)
	}
	if len(q.IDs) > 0 {
		db = db.Where("id IN ?", q.IDs)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		like := "%" + strings.NewReplacer("%", `\%`, "_", `\_`).Replace(strings.ToLower(s)) + "%"
		digits := domain.Digits(s)
		if len(digits) >= 3 {
			db = db.Where("name_key LIKE ? OR LOWER(payer_name) LIKE ? OR phone LIKE ?", like, like, "%"+digits+"%")
		} else {
			db = db.Where("name_key LIKE ? OR LOWER(payer_name) LIKE ? OR LOWER(fields::text) LIKE ?", like, like, like)
		}
	}
	return db
}
func (r Members) Find(ctx context.Context, q ports.MemberQuery) ([]domain.Member, error) {
	db := r.query(ctx, q).Order("CASE status WHEN 'past_due' THEN 0 WHEN 'suspended' THEN 1 ELSE 2 END, name_key, id")
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	if q.Offset > 0 {
		db = db.Offset(q.Offset)
	}
	return find[memberRecord, domain.Member](db)
}
func (r Members) Count(ctx context.Context, q ports.MemberQuery) (int64, error) {
	var n int64
	err := r.query(ctx, q).Count(&n).Error
	return n, err
}
func (r Members) CountByStatus(ctx context.Context, merchantID string, live bool) (map[string]int64, error) {
	var rows []struct {
		Status string
		N      int64
	}
	err := r.conn(ctx).Model(&memberRecord{}).Where("merchant_id = ? AND livemode = ?", merchantID, live).Select("status, COUNT(*) AS n").Group("status").Scan(&rows).Error
	out := map[string]int64{}
	for _, x := range rows {
		out[x.Status] = x.N
	}
	return out, err
}
func (r Members) FieldValues(ctx context.Context, merchantID string, live bool, key string) ([]string, error) {
	var out []string
	err := r.conn(ctx).Model(&memberRecord{}).Where("merchant_id = ? AND livemode = ? AND status <> ? AND fields ->> ? <> ''", merchantID, live, domain.SubIncomplete, key).
		Distinct().Order("1").Limit(200).Pluck("fields ->> '"+strings.ReplaceAll(key, "'", "''")+"'", &out).Error
	return out, err
}

type Subscriptions struct{ *Store }

func (r Subscriptions) Create(ctx context.Context, s *domain.Subscription) error {
	return create[subscriptionRecord](r.conn(ctx), s)
}
func (r Subscriptions) Save(ctx context.Context, s *domain.Subscription) error {
	return save[subscriptionRecord](r.conn(ctx), s)
}
func (r Subscriptions) Get(ctx context.Context, id string) (*domain.Subscription, error) {
	return first[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("id = ?", id))
}
func (r Subscriptions) Lock(ctx context.Context, id string) (*domain.Subscription, error) {
	return first[subscriptionRecord, domain.Subscription](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Subscriptions) ByMember(ctx context.Context, memberID string) ([]domain.Subscription, error) {
	return find[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("member_id = ?", memberID).Order("created_at"))
}
func (r Subscriptions) ByPlan(ctx context.Context, planID string, statuses []string) ([]domain.Subscription, error) {
	return find[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("plan_id = ? AND status IN ?", planID, statuses).Order("created_at"))
}
func (r Subscriptions) Live(ctx context.Context, merchantID string, live bool) ([]domain.Subscription, error) {
	return find[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status IN ?", merchantID, live, domain.LiveSubStatuses))
}
func (r Subscriptions) List(ctx context.Context, merchantID string, live bool, memberID string, statuses []string, limit int) ([]domain.Subscription, error) {
	db := r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status <> ?", merchantID, live, domain.SubIncomplete)
	if memberID != "" {
		db = db.Where("member_id = ?", memberID)
	}
	if len(statuses) > 0 {
		db = db.Where("status IN ?", statuses)
	}
	return find[subscriptionRecord, domain.Subscription](db.Order("created_at DESC").Limit(limit))
}
func (r Subscriptions) CountByPlan(ctx context.Context, merchantID string, live bool) (map[string]map[string]int64, error) {
	var rows []struct {
		PlanID string
		Status string
		N      int64
	}
	err := r.conn(ctx).Model(&subscriptionRecord{}).Where("merchant_id = ? AND livemode = ?", merchantID, live).
		Select("plan_id, status, COUNT(*) AS n").Group("plan_id, status").Scan(&rows).Error
	out := map[string]map[string]int64{}
	for _, x := range rows {
		if out[x.PlanID] == nil {
			out[x.PlanID] = map[string]int64{}
		}
		out[x.PlanID][x.Status] = x.N
	}
	return out, err
}
func (r Subscriptions) DueForIssue(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error) {
	return find[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("status IN ? AND next_due_at IS NOT NULL AND next_due_at <= ?",
		[]string{domain.SubActive, domain.SubPastDue, domain.SubPaused}, before).Order("next_due_at").Limit(limit))
}
func (r Subscriptions) StaleIncomplete(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error) {
	return find[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("status = ? AND created_at < ?", domain.SubIncomplete, before).Limit(limit))
}
func (r Subscriptions) PausedUntil(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error) {
	return find[subscriptionRecord, domain.Subscription](r.conn(ctx).Where("status = ? AND paused_until IS NOT NULL AND paused_until <= ?", domain.SubPaused, before).Limit(limit))
}

type Charges struct{ *Store }

func (r Charges) Create(ctx context.Context, c *domain.Charge) error {
	return create[chargeRecord](r.conn(ctx), c)
}
func (r Charges) Save(ctx context.Context, c *domain.Charge) error {
	return save[chargeRecord](r.conn(ctx), c)
}
func (r Charges) Lock(ctx context.Context, id string) (*domain.Charge, error) {
	return first[chargeRecord, domain.Charge](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Charges) ByToken(ctx context.Context, token string) (*domain.Charge, error) {
	return first[chargeRecord, domain.Charge](r.conn(ctx).Where("token = ?", token))
}
func (r Charges) query(ctx context.Context, q ports.ChargeQuery) *gormDB {
	db := r.conn(ctx).Model(&chargeRecord{})
	if q.MerchantID != "" {
		db = db.Where("merchant_id = ?", q.MerchantID)
	}
	if q.Live != nil {
		db = db.Where("livemode = ?", *q.Live)
	}
	if q.MemberID != "" {
		db = db.Where("member_id = ?", q.MemberID)
	}
	if len(q.SubscriptionIDs) > 0 {
		db = db.Where("subscription_id IN ?", q.SubscriptionIDs)
	}
	if len(q.Statuses) > 0 {
		db = db.Where("status IN ?", q.Statuses)
	}
	if q.SessionID != "" {
		db = db.Where("session_id = ?", q.SessionID)
	}
	if len(q.IDs) > 0 {
		db = db.Where("id IN ?", q.IDs)
	}
	if q.DueFrom != nil {
		db = db.Where("due_at >= ?", *q.DueFrom)
	}
	if q.DueTo != nil {
		db = db.Where("due_at < ?", *q.DueTo)
	}
	if q.PaidFrom != nil {
		db = db.Where("paid_at >= ?", *q.PaidFrom)
	}
	if q.PaidTo != nil {
		db = db.Where("paid_at < ?", *q.PaidTo)
	}
	if q.ExcludeInvites {
		db = db.Where("NOT (first AND status = ?)", domain.ChargeOpen)
	}
	return db
}
func (r Charges) Find(ctx context.Context, q ports.ChargeQuery) ([]domain.Charge, error) {
	db := r.query(ctx, q)
	if q.OrderBy != "" {
		db = db.Order(q.OrderBy)
	} else {
		db = db.Order("due_at")
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	return find[chargeRecord, domain.Charge](db)
}
func (r Charges) Sums(ctx context.Context, q ports.ChargeQuery) ([]ports.ChargeSum, error) {
	var out []ports.ChargeSum
	err := r.query(ctx, q).Select("status, COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").Group("status").Scan(&out).Error
	return out, err
}
func (r Charges) Exists(ctx context.Context, subscriptionID, period string) (bool, error) {
	return exists(r.conn(ctx).Model(&chargeRecord{}).Where("subscription_id = ? AND period = ?", subscriptionID, period))
}
func (r Charges) Active(ctx context.Context, before time.Time, limit int) ([]domain.Charge, error) {
	return find[chargeRecord, domain.Charge](r.conn(ctx).Where("status IN ? AND due_at <= ?",
		[]string{domain.ChargeScheduled, domain.ChargeOpen, domain.ChargePastDue}, before).Order("updated_at").Limit(limit))
}

type Messages struct{ *Store }

func (r Messages) Create(ctx context.Context, m *domain.Message) error {
	return create[messageRecord](r.conn(ctx), m)
}
func (r Messages) Save(ctx context.Context, m *domain.Message) error {
	return save[messageRecord](r.conn(ctx), m)
}
func (r Messages) ByMember(ctx context.Context, memberID string, limit int) ([]domain.Message, error) {
	return find[messageRecord, domain.Message](r.conn(ctx).Where("member_id = ?", memberID).Order("created_at DESC").Limit(limit))
}
func (r Messages) Queued(ctx context.Context, now time.Time, limit int) ([]domain.Message, error) {
	return find[messageRecord, domain.Message](r.conn(ctx).Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", domain.MessageQueued, now).Order("created_at").Limit(limit))
}
func (r Messages) ByProviderID(ctx context.Context, providerID string) (*domain.Message, error) {
	return first[messageRecord, domain.Message](r.conn(ctx).Where("provider_id = ?", providerID))
}
func (r Messages) Failed(ctx context.Context, merchantID string, live bool, since time.Time, limit int) ([]domain.Message, error) {
	return find[messageRecord, domain.Message](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status = ? AND created_at >= ?", merchantID, live, "failed", since).Order("created_at DESC").Limit(limit))
}
func (r Messages) CountRecent(ctx context.Context, memberID, kind string, since time.Time) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&messageRecord{}).Where("member_id = ? AND kind = ? AND created_at >= ?", memberID, kind, since).Count(&n).Error
	return n, err
}

type SubscriptionSettings struct{ *Store }

func (r SubscriptionSettings) Get(ctx context.Context, merchantID string) (*domain.SubscriptionSettings, error) {
	return first[subSettingsRecord, domain.SubscriptionSettings](r.conn(ctx).Where("merchant_id = ?", merchantID))
}
func (r SubscriptionSettings) Save(ctx context.Context, s *domain.SubscriptionSettings) error {
	return save[subSettingsRecord](r.conn(ctx), s)
}
func (r SubscriptionSettings) ByCatalogSlug(ctx context.Context, slug string) (*domain.SubscriptionSettings, error) {
	return first[subSettingsRecord, domain.SubscriptionSettings](r.conn(ctx).Where("catalog_slug = ? AND catalog_slug <> ''", slug))
}
func (r SubscriptionSettings) WithCatalog(ctx context.Context) ([]domain.SubscriptionSettings, error) {
	return find[subSettingsRecord, domain.SubscriptionSettings](r.conn(ctx).Where("catalog_slug <> ''").Order("updated_at DESC").Limit(5000))
}
