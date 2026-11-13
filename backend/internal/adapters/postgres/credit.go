package postgres

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type TimeList []time.Time

func (l TimeList) Value() (driver.Value, error) {
	b, err := json.Marshal([]time.Time(l))
	return string(b), err
}
func (l *TimeList) Scan(v any) error { return scanJSON(v, (*[]time.Time)(l)) }

type Int64List []int64

func (l Int64List) Value() (driver.Value, error) {
	b, err := json.Marshal([]int64(l))
	return string(b), err
}
func (l *Int64List) Scan(v any) error { return scanJSON(v, (*[]int64)(l)) }

type creditProductRecord struct {
	ID           string `gorm:"primaryKey;size:40"`
	MerchantID   string `gorm:"size:40;not null;index:credit_product_merchant"`
	Livemode     bool   `gorm:"index:credit_product_merchant"`
	Name         string `gorm:"size:120"`
	Min, Max     int64
	Freq         string `gorm:"size:12"`
	TermMin      int
	TermMax      int
	RateBP       int
	DayMode      string `gorm:"size:12"`
	Mora         bool
	MoraBP       int
	Grace        int
	MoraMaxWeeks int
	EffortPct    int
	Fields       StringList `gorm:"type:jsonb"`
	Status       string     `gorm:"size:12"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (creditProductRecord) TableName() string { return "credit_products" }

type creditRequestRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	MerchantID  string `gorm:"size:40;not null;index:credit_request_merchant"`
	Livemode    bool   `gorm:"index:credit_request_merchant"`
	ProductID   string `gorm:"size:40;index"`
	Token       string `gorm:"size:60;uniqueIndex"`
	Name        string `gorm:"size:120"`
	Phone       string `gorm:"size:20;index"`
	Wallet      string `gorm:"size:12"`
	IDNumber    string `gorm:"size:30"`
	NUIT        string `gorm:"size:20"`
	Address     string `gorm:"size:200"`
	Activity    string `gorm:"size:200"`
	Purpose     string `gorm:"size:300"`
	Income      int64
	Extra       StringMap `gorm:"type:jsonb"`
	Amount      int64
	Term        int
	Day         int
	Instalment  int64
	EffortPct   int
	Status      string `gorm:"size:12;index"`
	Source      string `gorm:"size:12"`
	PhoneOKAt   *time.Time
	DeclineNote string `gorm:"size:300"`
	DecidedBy   string `gorm:"size:40"`
	DecidedAt   *time.Time
	LoanID      string    `gorm:"size:40"`
	IP          string    `gorm:"size:64"`
	CreatedAt   time.Time `gorm:"index"`
	UpdatedAt   time.Time
}

func (creditRequestRecord) TableName() string { return "credit_requests" }

type creditDocumentRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	RequestID   string `gorm:"size:40;index"`
	MerchantID  string `gorm:"size:40;index"`
	Kind        string `gorm:"size:20"`
	FileName    string `gorm:"size:200"`
	StoredPath  string `gorm:"size:300"`
	ContentType string `gorm:"size:60"`
	Size        int64
	CreatedAt   time.Time
}

func (creditDocumentRecord) TableName() string { return "credit_documents" }

type loanRecord struct {
	ID             string `gorm:"primaryKey;size:40"`
	MerchantID     string `gorm:"size:40;not null;index:loan_merchant"`
	Livemode       bool   `gorm:"index:loan_merchant"`
	ProductID      string `gorm:"size:40;index"`
	RequestID      string `gorm:"size:40"`
	Token          string `gorm:"size:60;uniqueIndex"`
	Reference      string `gorm:"size:20;index"`
	Name           string `gorm:"size:120"`
	Phone          string `gorm:"size:20;index"`
	Wallet         string `gorm:"size:12"`
	Amount         int64
	Term           int
	Day            int
	Freq           string `gorm:"size:12"`
	RateBP         int
	MoraBP         int
	Grace          int
	MoraMax        int
	Status         string `gorm:"size:12;index"`
	Delivery       string `gorm:"size:10"`
	DeliveryStatus string `gorm:"size:12"`
	DeliveryError  string `gorm:"size:200"`
	DeliveredAt    *time.Time
	PayoutRef      string `gorm:"size:80"`
	Version        int
	Proposed       int
	AskedAt        *time.Time
	Manual         bool
	SettleOffer    int64
	ApprovedBy     string `gorm:"size:40"`
	SecondBy       string `gorm:"size:40"`
	CreatedBy      string `gorm:"size:40"`
	AcceptedAt     *time.Time
	ClosedAt       *time.Time
	CloseNote      string `gorm:"size:300"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (loanRecord) TableName() string { return "loans" }

type instalmentRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	LoanID     string `gorm:"size:40;index"`
	MerchantID string `gorm:"size:40;index:instalment_merchant"`
	Livemode   bool   `gorm:"index:instalment_merchant"`
	Version    int
	Seq        int
	DueAt      time.Time `gorm:"index"`
	Amount     int64
	Status     string `gorm:"size:14;index"`
	MoraPaid   int64
	MoraWaived bool
	Charged    int64
	SessionID  *string `gorm:"size:60;index"`
	PaymentID  *string `gorm:"size:60"`
	Method     string  `gorm:"size:20"`
	PaidAt     *time.Time
	MarkedBy   string `gorm:"size:120"`
	Note       string `gorm:"size:200"`
	LinkSentAt *time.Time
	OpenedAt   *time.Time
	Steps      string `gorm:"size:200"`
	Failure    string `gorm:"size:200"`
	UpdatedAt  time.Time
}

func (instalmentRecord) TableName() string { return "loan_instalments" }

type loanAgreementRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	LoanID     string `gorm:"size:40;index"`
	MerchantID string `gorm:"size:40"`
	Version    int
	Status     string `gorm:"size:12"`
	Text       string `gorm:"type:text"`
	Total      int64
	Dates      TimeList  `gorm:"type:jsonb"`
	Amounts    Int64List `gorm:"type:jsonb"`
	MoraWaived int64
	SentAt     time.Time
	AcceptedAt *time.Time
	Phone      string `gorm:"size:20"`
	IP         string `gorm:"size:64"`
	CreatedBy  string `gorm:"size:40"`
}

func (loanAgreementRecord) TableName() string { return "loan_agreements" }

type creditCodeRecord struct {
	ID        string `gorm:"primaryKey;size:40"`
	Subject   string `gorm:"size:40;index:credit_code_subject"`
	Purpose   string `gorm:"size:12;index:credit_code_subject"`
	Phone     string `gorm:"size:20;index"`
	Hash      string `gorm:"size:100"`
	Tries     int
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time `gorm:"index"`
}

func (creditCodeRecord) TableName() string { return "credit_codes" }

type creditSettingsRecord struct {
	MerchantID string `gorm:"primaryKey;size:40"`
	Data       string `gorm:"type:text"`
	UpdatedAt  time.Time
}

func (creditSettingsRecord) TableName() string { return "credit_settings" }

func creditRecords() []any {
	return []any{&creditProductRecord{}, &creditRequestRecord{}, &creditDocumentRecord{}, &loanRecord{}, &instalmentRecord{},
		&loanAgreementRecord{}, &creditCodeRecord{}, &creditSettingsRecord{}}
}

type Credit struct{ *Store }

var _ ports.Credit = Credit{}

func like(s string) string {
	return "%" + strings.NewReplacer("%", `\%`, "_", `\_`).Replace(strings.ToLower(strings.TrimSpace(s))) + "%"
}

func (r Credit) CreateProduct(ctx context.Context, p *domain.CreditProduct) error {
	return create[creditProductRecord](r.conn(ctx), p)
}
func (r Credit) SaveProduct(ctx context.Context, p *domain.CreditProduct) error {
	return save[creditProductRecord](r.conn(ctx), p)
}
func (r Credit) Product(ctx context.Context, merchantID, id string) (*domain.CreditProduct, error) {
	return first[creditProductRecord, domain.CreditProduct](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}
func (r Credit) Products(ctx context.Context, merchantID string, live bool) ([]domain.CreditProduct, error) {
	return find[creditProductRecord, domain.CreditProduct](r.conn(ctx).Where("merchant_id = ? AND livemode = ?", merchantID, live).Order("created_at, id"))
}

func (r Credit) CreateRequest(ctx context.Context, q *domain.CreditRequest) error {
	return create[creditRequestRecord](r.conn(ctx), q)
}
func (r Credit) SaveRequest(ctx context.Context, q *domain.CreditRequest) error {
	return save[creditRequestRecord](r.conn(ctx), q)
}
func (r Credit) LockRequest(ctx context.Context, id string) (*domain.CreditRequest, error) {
	return first[creditRequestRecord, domain.CreditRequest](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Credit) Request(ctx context.Context, merchantID, id string) (*domain.CreditRequest, error) {
	return first[creditRequestRecord, domain.CreditRequest](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}
func (r Credit) RequestByToken(ctx context.Context, token string) (*domain.CreditRequest, error) {
	if token == "" {
		return nil, domain.ErrNotFound
	}
	return first[creditRequestRecord, domain.CreditRequest](r.conn(ctx).Where("token = ?", token))
}
func (r Credit) Requests(ctx context.Context, q ports.CreditRequestQuery) ([]domain.CreditRequest, error) {
	db := r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status <> ?", q.MerchantID, q.Live, domain.RequestVerifying)
	if len(q.Statuses) > 0 {
		db = db.Where("status IN ?", q.Statuses)
	}
	if q.ProductID != "" {
		db = db.Where("product_id = ?", q.ProductID)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		if d := domain.Digits(s); len(d) >= 3 {
			db = db.Where("phone LIKE ?", "%"+d+"%")
		} else {
			db = db.Where("LOWER(name) LIKE ?", like(s))
		}
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	return find[creditRequestRecord, domain.CreditRequest](db.Order("created_at DESC, id"))
}
func (r Credit) CountRequests(ctx context.Context, merchantID string, live bool) (map[string]int64, error) {
	var rows []struct {
		Status string
		N      int64
	}
	err := r.conn(ctx).Model(&creditRequestRecord{}).Where("merchant_id = ? AND livemode = ?", merchantID, live).Select("status, COUNT(*) AS n").Group("status").Scan(&rows).Error
	out := map[string]int64{}
	for _, x := range rows {
		out[x.Status] = x.N
	}
	return out, err
}
func (r Credit) RecentRequests(ctx context.Context, phone string, since time.Time) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&creditRequestRecord{}).Where("phone = ? AND created_at > ?", phone, since).Count(&n).Error
	return n, err
}

func (r Credit) AddDocument(ctx context.Context, d *domain.CreditDocument) error {
	return create[creditDocumentRecord](r.conn(ctx), d)
}
func (r Credit) Documents(ctx context.Context, requestID string) ([]domain.CreditDocument, error) {
	return find[creditDocumentRecord, domain.CreditDocument](r.conn(ctx).Where("request_id = ?", requestID).Order("created_at"))
}
func (r Credit) Document(ctx context.Context, merchantID, id string) (*domain.CreditDocument, error) {
	return first[creditDocumentRecord, domain.CreditDocument](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}

func (r Credit) CreateLoan(ctx context.Context, l *domain.Loan) error {
	return create[loanRecord](r.conn(ctx), l)
}
func (r Credit) SaveLoan(ctx context.Context, l *domain.Loan) error {
	return save[loanRecord](r.conn(ctx), l)
}
func (r Credit) LockLoan(ctx context.Context, id string) (*domain.Loan, error) {
	return first[loanRecord, domain.Loan](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Credit) Loan(ctx context.Context, merchantID, id string) (*domain.Loan, error) {
	return first[loanRecord, domain.Loan](r.conn(ctx).Where("id = ? AND merchant_id = ?", id, merchantID))
}
func (r Credit) LoanByID(ctx context.Context, id string) (*domain.Loan, error) {
	return first[loanRecord, domain.Loan](r.conn(ctx).Where("id = ?", id))
}
func (r Credit) LoanByToken(ctx context.Context, token string) (*domain.Loan, error) {
	if token == "" {
		return nil, domain.ErrNotFound
	}
	return first[loanRecord, domain.Loan](r.conn(ctx).Where("token = ?", token))
}
func (r Credit) Loans(ctx context.Context, q ports.LoanQuery) ([]domain.Loan, error) {
	db := r.conn(ctx).Where("merchant_id = ? AND livemode = ?", q.MerchantID, q.Live)
	if len(q.Statuses) > 0 {
		db = db.Where("status IN ?", q.Statuses)
	}
	if len(q.IDs) > 0 {
		db = db.Where("id IN ?", q.IDs)
	}
	if q.ProductID != "" {
		db = db.Where("product_id = ?", q.ProductID)
	}
	if s := strings.TrimSpace(q.Search); s != "" {
		if d := domain.Digits(s); len(d) >= 3 {
			db = db.Where("phone LIKE ?", "%"+d+"%")
		} else {
			db = db.Where("LOWER(name) LIKE ? OR LOWER(reference) LIKE ?", like(s), like(s))
		}
	}
	if q.Limit > 0 {
		db = db.Limit(q.Limit)
	}
	return find[loanRecord, domain.Loan](db.Order("created_at DESC, id"))
}
func (r Credit) LoansByPhone(ctx context.Context, merchantID string, live bool, phone string) ([]domain.Loan, error) {
	return find[loanRecord, domain.Loan](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND phone = ?", merchantID, live, phone).Order("created_at DESC").Limit(20))
}

func (r Credit) CreateInstalment(ctx context.Context, i *domain.Instalment) error {
	return create[instalmentRecord](r.conn(ctx), i)
}
func (r Credit) SaveInstalment(ctx context.Context, i *domain.Instalment) error {
	return save[instalmentRecord](r.conn(ctx), i)
}
func (r Credit) LockInstalment(ctx context.Context, id string) (*domain.Instalment, error) {
	return first[instalmentRecord, domain.Instalment](r.conn(ctx).Clauses(forUpdate).Where("id = ?", id))
}
func (r Credit) Instalments(ctx context.Context, loanID string) ([]domain.Instalment, error) {
	return find[instalmentRecord, domain.Instalment](r.conn(ctx).Where("loan_id = ?", loanID).Order("version, seq"))
}
func (r Credit) InstalmentsOf(ctx context.Context, merchantID string, live bool, statuses []string) ([]domain.Instalment, error) {
	return find[instalmentRecord, domain.Instalment](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status IN ?", merchantID, live, statuses).Order("due_at"))
}
func (r Credit) InstalmentsBySession(ctx context.Context, sessionID string) ([]domain.Instalment, error) {
	return find[instalmentRecord, domain.Instalment](r.conn(ctx).Where("session_id = ?", sessionID).Order("seq"))
}
func (r Credit) OpenInstalmentsDue(ctx context.Context, before time.Time, limit int) ([]domain.Instalment, error) {
	return find[instalmentRecord, domain.Instalment](r.conn(ctx).Where("status = ? AND due_at < ? AND loan_id IN (?)", domain.InstScheduled, before,
		r.conn(ctx).Model(&loanRecord{}).Select("id").Where("status = ?", domain.LoanActive)).Order("due_at").Limit(limit))
}
func (r Credit) PaidInstalments(ctx context.Context, merchantID string, live bool, from, to time.Time) ([]domain.Instalment, error) {
	return find[instalmentRecord, domain.Instalment](r.conn(ctx).Where("merchant_id = ? AND livemode = ? AND status IN ? AND paid_at >= ? AND paid_at < ?",
		merchantID, live, []string{domain.InstPaid, domain.InstPaidOut}, from, to))
}

func (r Credit) CreateAgreement(ctx context.Context, g *domain.LoanAgreement) error {
	return create[loanAgreementRecord](r.conn(ctx), g)
}
func (r Credit) SaveAgreement(ctx context.Context, g *domain.LoanAgreement) error {
	return save[loanAgreementRecord](r.conn(ctx), g)
}
func (r Credit) Agreements(ctx context.Context, loanID string) ([]domain.LoanAgreement, error) {
	return find[loanAgreementRecord, domain.LoanAgreement](r.conn(ctx).Where("loan_id = ?", loanID).Order("version"))
}

func (r Credit) CreateCode(ctx context.Context, c *domain.CreditCode) error {
	return create[creditCodeRecord](r.conn(ctx), c)
}
func (r Credit) SaveCode(ctx context.Context, c *domain.CreditCode) error {
	return save[creditCodeRecord](r.conn(ctx), c)
}
func (r Credit) LatestCode(ctx context.Context, subject, purpose string) (*domain.CreditCode, error) {
	return first[creditCodeRecord, domain.CreditCode](r.conn(ctx).Clauses(forUpdate).Where("subject = ? AND purpose = ?", subject, purpose).Order("created_at DESC"))
}
func (r Credit) CodesSent(ctx context.Context, phone string, since time.Time) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&creditCodeRecord{}).Where("phone = ? AND created_at > ?", phone, since).Count(&n).Error
	return n, err
}

func (r Credit) Settings(ctx context.Context, merchantID string) (*domain.CreditSettings, error) {
	var rec creditSettingsRecord
	err := r.conn(ctx).Where("merchant_id = ?", merchantID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s := domain.DefaultCreditSettings()
	if err := json.Unmarshal([]byte(rec.Data), &s); err != nil {
		return nil, err
	}
	return &s, nil
}
func (r Credit) SaveSettings(ctx context.Context, merchantID string, s domain.CreditSettings) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return r.conn(ctx).Save(&creditSettingsRecord{MerchantID: merchantID, Data: string(b), UpdatedAt: time.Now().UTC()}).Error
}
