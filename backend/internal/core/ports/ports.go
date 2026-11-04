package ports

import (
	"context"
	"io"
	"time"

	"riha/internal/core/domain"
)

type Tx interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Users interface {
	Create(ctx context.Context, u *domain.User) error
	ByID(ctx context.Context, id string) (*domain.User, error)
	ByEmail(ctx context.Context, email string) (*domain.User, error)
	EmailTaken(ctx context.Context, email string) (bool, error)
	SetPassword(ctx context.Context, id, hash string) error
	TouchLogin(ctx context.Context, id string, at time.Time) error
}

type AuthSessions interface {
	Create(ctx context.Context, s *domain.AuthSession) error
	UserID(ctx context.Context, tokenHash string, now time.Time) (string, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteForUser(ctx context.Context, userID, exceptHash string) error
	Purge(ctx context.Context, before time.Time) error
}

type MerchantFilter struct {
	Statuses []string
	Query    string
	Limit    int
	OrderBy  string
}

type Merchants interface {
	Create(ctx context.Context, m *domain.Merchant) error
	ByID(ctx context.Context, id string) (*domain.Merchant, error)
	BySlug(ctx context.Context, slug string) (*domain.Merchant, error)
	Lock(ctx context.Context, id string) (*domain.Merchant, error)
	SlugTaken(ctx context.Context, slug string) (bool, error)
	Save(ctx context.Context, m *domain.Merchant) error
	SetFee(ctx context.Context, id string, bps *int) error
	List(ctx context.Context, f MerchantFilter) ([]domain.Merchant, error)
	Count(ctx context.Context, statuses []string) (int64, error)
}

type Documents interface {
	Create(ctx context.Context, d *domain.Document) error
	ByID(ctx context.Context, id string) (*domain.Document, error)
	ByMerchant(ctx context.Context, merchantID string) ([]domain.Document, error)
	RemoveKind(ctx context.Context, merchantID, kind string) ([]domain.Document, error)
	CountKinds(ctx context.Context, merchantID string, kinds []string) (int64, error)
}

type BankAccounts interface {
	ByMerchant(ctx context.Context, merchantID string) (*domain.BankAccount, error)
	Save(ctx context.Context, b *domain.BankAccount) error
	SetDocument(ctx context.Context, merchantID, documentID string) error
	MarkVerified(ctx context.Context, merchantID string) error
}

type APIKeys interface {
	Create(ctx context.Context, k *domain.APIKey) error
	ByID(ctx context.Context, id string) (*domain.APIKey, error)
	Active(ctx context.Context, merchantID string, live bool, kind string) (*domain.APIKey, error)
	BySecretHash(ctx context.Context, hash string) (*domain.APIKey, error)
	RevokeActive(ctx context.Context, merchantID string, live bool, kind string, at time.Time) error
	Touch(ctx context.Context, id string, at time.Time) error
}

type LinkStats struct {
	Count int64
	Total int64
}

type Links interface {
	Create(ctx context.Context, l *domain.PaymentLink) error
	ByID(ctx context.Context, id string) (*domain.PaymentLink, error)
	BySlug(ctx context.Context, merchantID, slug string) (*domain.PaymentLink, error)
	SlugTaken(ctx context.Context, merchantID, slug string) (bool, error)
	Save(ctx context.Context, l *domain.PaymentLink) error
	List(ctx context.Context, merchantID string, live bool, limit int) ([]domain.PaymentLink, error)
	Stats(ctx context.Context, linkID string) (LinkStats, error)
}

type CheckoutSessions interface {
	Create(ctx context.Context, cs *domain.CheckoutSession) error
	ByID(ctx context.Context, id string) (*domain.CheckoutSession, error)
	Lock(ctx context.Context, id string) (*domain.CheckoutSession, error)
	Save(ctx context.Context, cs *domain.CheckoutSession) error
	List(ctx context.Context, merchantID string, live bool, clientRef string, limit int) ([]domain.CheckoutSession, error)
	ExpiredOpen(ctx context.Context, now time.Time, limit int) ([]domain.CheckoutSession, error)
}

type PaymentQuery struct {
	MerchantID    string
	Live          *bool
	Statuses      []string
	Source        string
	Search        string
	From, To      *time.Time
	StartingAfter *domain.Payment
	Limit, Offset int
}

type Payments interface {
	Create(ctx context.Context, p *domain.Payment) error
	Delete(ctx context.Context, id string) error
	Save(ctx context.Context, p *domain.Payment) error
	ByID(ctx context.Context, id string) (*domain.Payment, error)
	Lock(ctx context.Context, id string) (*domain.Payment, error)
	ByReference(ctx context.Context, ref string) (*domain.Payment, error)
	LatestBySession(ctx context.Context, sessionID string) (*domain.Payment, error)
	CountBySession(ctx context.Context, sessionID, status string) (int64, error)
	CountSettledByLink(ctx context.Context, linkID string) (int64, error)
	Find(ctx context.Context, q PaymentQuery) ([]domain.Payment, error)
	Count(ctx context.Context, q PaymentQuery) (int64, error)
	StaleProcessing(ctx context.Context, before time.Time, limit int) ([]domain.Payment, error)
}

type Refunds interface {
	Create(ctx context.Context, r *domain.Refund) error
	ByPayment(ctx context.Context, paymentID string) ([]domain.Refund, error)
}

type Ledger interface {
	Add(ctx context.Context, e *domain.LedgerEntry) error
	Balance(ctx context.Context, merchantID string, live bool) (int64, error)
}

type PayoutQuery struct {
	MerchantID string
	Live       *bool
	Statuses   []string
	IDs        []string
	DueBefore  *time.Time
	OrderBy    string
	Limit      int
}

type Payouts interface {
	NextNumber(ctx context.Context) (int64, error)
	Create(ctx context.Context, p *domain.Payout) error
	Save(ctx context.Context, p *domain.Payout) error
	Lock(ctx context.Context, id string) (*domain.Payout, error)
	Find(ctx context.Context, q PayoutQuery) ([]domain.Payout, error)
	Sum(ctx context.Context, q PayoutQuery) (count, total int64, err error)
	PaidBetween(ctx context.Context, merchantID string, live bool, from, to time.Time) (count, total int64, err error)
	TestModeToSettle(ctx context.Context, before time.Time, limit int) ([]domain.Payout, error)
}

type Endpoints interface {
	Create(ctx context.Context, e *domain.WebhookEndpoint) error
	ByID(ctx context.Context, merchantID, id string) (*domain.WebhookEndpoint, error)
	Save(ctx context.Context, e *domain.WebhookEndpoint) error
	Delete(ctx context.Context, merchantID string, live *bool, id string) (bool, error)
	List(ctx context.Context, merchantID string, live bool) ([]domain.WebhookEndpoint, error)
}

type Domains interface {
	Create(ctx context.Context, d *domain.AllowedDomain) error
	List(ctx context.Context, merchantID string) ([]domain.AllowedDomain, error)
	Delete(ctx context.Context, merchantID, id string) error
}

type Events interface {
	Create(ctx context.Context, e *domain.Event) error
	ByID(ctx context.Context, id string) (*domain.Event, error)
	ByObjects(ctx context.Context, merchantID string, objectIDs []string) ([]domain.Event, error)
}

type Deliveries interface {
	Create(ctx context.Context, d *domain.Delivery) error
	ClaimDue(ctx context.Context, now, lease time.Time, limit int) ([]domain.Delivery, error)
	Save(ctx context.Context, d *domain.Delivery) error
	Requeue(ctx context.Context, merchantID, id string, at time.Time) (bool, error)
	Recent(ctx context.Context, merchantID string, live bool, limit int) ([]domain.Delivery, error)
	ByEvents(ctx context.Context, eventIDs []string) ([]domain.Delivery, error)
}

type RequestLogs interface {
	Create(ctx context.Context, l *domain.RequestLog) error
	Recent(ctx context.Context, merchantID string, live bool, limit int) ([]domain.RequestLog, error)
}

type Idempotency interface {
	Get(ctx context.Context, merchantID string, live bool, key string) (*domain.IdempotencyRecord, error)
	Save(ctx context.Context, r *domain.IdempotencyRecord) error
	Purge(ctx context.Context, before time.Time) error
}

type Audit interface {
	Record(ctx context.Context, e *domain.AuditEntry) error
	ByMerchant(ctx context.Context, merchantID string, limit int) ([]domain.AuditEntry, error)
}

type Settings interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type Scope struct {
	MerchantID string
	Live       bool
	From, To   *time.Time
}

type Bucket struct {
	Date   time.Time
	Method string
	Amount int64
}

type MethodHealth struct {
	Method      string
	Total       int64
	Succeeded   int64
	OperatorOff int64
}

type Reports interface {
	SettledVolume(ctx context.Context, s Scope) (int64, error)
	SettledCount(ctx context.Context, s Scope) (int64, error)
	Attempts(ctx context.Context, s Scope) (int64, error)
	VolumeBuckets(ctx context.Context, s Scope, unit string) ([]Bucket, error)
	VolumeByMethod(ctx context.Context, s Scope) (map[string]int64, error)
	VolumeByDayAndMethod(ctx context.Context, s Scope) ([]Bucket, error)
	Health(ctx context.Context, since time.Time) ([]MethodHealth, error)
	MainMethod(ctx context.Context, merchantID string) (string, error)
}

type Rail interface {
	Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error)
}

type WebhookSender interface {
	Send(ctx context.Context, url string, headers map[string]string, body []byte) (status int, err error)
}

type FileStore interface {
	Save(ctx context.Context, dir, name, contentType string, r io.Reader) (path string, err error)
	Read(ctx context.Context, path string) ([]byte, error)
	Remove(ctx context.Context, path string) error
}

type Renderer interface {
	PayoutInstructions(sheets []domain.PayoutSheet) ([]byte, error)
	PayoutStatement(sheet domain.PayoutSheet) ([]byte, error)
}

type Mailer interface {
	Send(ctx context.Context, e domain.Email) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Matches(hash, password string) bool
}

type MemberQuery struct {
	MerchantID string
	Live       bool
	PlanID     string
	Statuses   []string
	Search     string
	FieldKey   string
	FieldValue string
	IDs        []string
	Limit      int
	Offset     int
}

type ChargeQuery struct {
	MerchantID      string
	Live            *bool
	MemberID        string
	SubscriptionIDs []string
	Statuses        []string
	SessionID       string
	IDs             []string
	DueFrom, DueTo  *time.Time
	PaidFrom        *time.Time
	PaidTo          *time.Time
	ExcludeInvites  bool
	OrderBy         string
	Limit           int
}

type ChargeSum struct {
	Status string
	Count  int64
	Total  int64
}

type Plans interface {
	Create(ctx context.Context, p *domain.Plan) error
	Save(ctx context.Context, p *domain.Plan) error
	ByID(ctx context.Context, merchantID, id string) (*domain.Plan, error)
	Get(ctx context.Context, id string) (*domain.Plan, error)
	BySlug(ctx context.Context, merchantID, slug string) (*domain.Plan, error)
	SlugTaken(ctx context.Context, merchantID, slug string) (bool, error)
	List(ctx context.Context, merchantID string, live bool) ([]domain.Plan, error)
	AddVersion(ctx context.Context, v *domain.PlanVersion) error
	Versions(ctx context.Context, planID string) ([]domain.PlanVersion, error)
}

type Members interface {
	Create(ctx context.Context, m *domain.Member) error
	Save(ctx context.Context, m *domain.Member) error
	ByID(ctx context.Context, merchantID, id string) (*domain.Member, error)
	Get(ctx context.Context, id string) (*domain.Member, error)
	Lock(ctx context.Context, id string) (*domain.Member, error)
	Identity(ctx context.Context, merchantID string, live bool, phone, nameKey string) (*domain.Member, error)
	Find(ctx context.Context, q MemberQuery) ([]domain.Member, error)
	Count(ctx context.Context, q MemberQuery) (int64, error)
	CountByStatus(ctx context.Context, merchantID string, live bool) (map[string]int64, error)
	FieldValues(ctx context.Context, merchantID string, live bool, key string) ([]string, error)
}

type Subscriptions interface {
	Create(ctx context.Context, s *domain.Subscription) error
	Save(ctx context.Context, s *domain.Subscription) error
	Get(ctx context.Context, id string) (*domain.Subscription, error)
	Lock(ctx context.Context, id string) (*domain.Subscription, error)
	ByMember(ctx context.Context, memberID string) ([]domain.Subscription, error)
	ByPlan(ctx context.Context, planID string, statuses []string) ([]domain.Subscription, error)
	Live(ctx context.Context, merchantID string, live bool) ([]domain.Subscription, error)
	List(ctx context.Context, merchantID string, live bool, memberID string, statuses []string, limit int) ([]domain.Subscription, error)
	CountByPlan(ctx context.Context, merchantID string, live bool) (map[string]map[string]int64, error)
	DueForIssue(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error)
	StaleIncomplete(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error)
	PausedUntil(ctx context.Context, before time.Time, limit int) ([]domain.Subscription, error)
}

type Charges interface {
	Create(ctx context.Context, c *domain.Charge) error
	Save(ctx context.Context, c *domain.Charge) error
	Lock(ctx context.Context, id string) (*domain.Charge, error)
	ByToken(ctx context.Context, token string) (*domain.Charge, error)
	Find(ctx context.Context, q ChargeQuery) ([]domain.Charge, error)
	Sums(ctx context.Context, q ChargeQuery) ([]ChargeSum, error)
	Exists(ctx context.Context, subscriptionID, period string) (bool, error)
	Active(ctx context.Context, before time.Time, limit int) ([]domain.Charge, error)
}

type Messages interface {
	Create(ctx context.Context, m *domain.Message) error
	Save(ctx context.Context, m *domain.Message) error
	ByMember(ctx context.Context, memberID string, limit int) ([]domain.Message, error)
	Queued(ctx context.Context, limit int) ([]domain.Message, error)
	Failed(ctx context.Context, merchantID string, live bool, since time.Time, limit int) ([]domain.Message, error)
	CountRecent(ctx context.Context, memberID, kind string, since time.Time) (int64, error)
}

type SubscriptionSettingsRepo interface {
	Get(ctx context.Context, merchantID string) (*domain.SubscriptionSettings, error)
	Save(ctx context.Context, s *domain.SubscriptionSettings) error
}

type OutboundMessage struct {
	Channel string
	To      string
	Sender  string
	Body    string
}

type Messenger interface {
	Send(ctx context.Context, m OutboundMessage) error
}

type Sheet interface {
	Rows(name string, data []byte) ([][]string, error)
}
