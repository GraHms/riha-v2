package ports

import (
	"context"
	"errors"
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
	LoginFailed(ctx context.Context, id string, now time.Time, max int, lock time.Duration) error
	// Team lists staff (role admin, no partner) or one partner's users.
	Team(ctx context.Context, role, partnerCode string) ([]domain.User, error)
	// SetAccess saves a user's name, title, permissions and whether it's closed.
	SetAccess(ctx context.Context, u *domain.User) error
}

type AuthSessions interface {
	Create(ctx context.Context, s *domain.AuthSession) error
	UserID(ctx context.Context, tokenHash string, now time.Time) (string, error)
	Delete(ctx context.Context, tokenHash string) error
	DeleteForUser(ctx context.Context, userID, exceptHash string) error
	Purge(ctx context.Context, before time.Time) error
}

type Emails interface {
	Create(ctx context.Context, e *domain.QueuedEmail) error
	ClaimDue(ctx context.Context, now, lease time.Time, limit int) ([]domain.QueuedEmail, error)
	Save(ctx context.Context, e *domain.QueuedEmail) error
	Purge(ctx context.Context, before time.Time) error
}

type PasswordResets interface {
	Create(ctx context.Context, r *domain.PasswordReset) error
	CountSince(ctx context.Context, userID string, since time.Time) (int64, error)
	// Take spends a valid token and returns its user, or domain.ErrNotFound.
	Take(ctx context.Context, tokenHash string, now time.Time) (string, error)
	SpendAll(ctx context.Context, userID string, now time.Time) error
	Purge(ctx context.Context, before time.Time) error
	// Codes emailed to staff and partners on a device RIHA doesn't know.
	SaveLoginCode(ctx context.Context, c *domain.LoginCode) error
	LoginCode(ctx context.Context, userID string) (*domain.LoginCode, error)
	TryLoginCode(ctx context.Context, userID string) error
	SpendLoginCode(ctx context.Context, userID string) error
	TrustDevice(ctx context.Context, d *domain.TrustedDevice) error
	DeviceTrusted(ctx context.Context, userID, tokenHash string, now time.Time) (bool, error)
}

type MerchantFilter struct {
	Statuses []string
	Query    string
	Limit    int
	Offset   int
	OrderBy  string
}

type Merchants interface {
	Create(ctx context.Context, m *domain.Merchant) error
	ByID(ctx context.Context, id string) (*domain.Merchant, error)
	// Many reads several merchants in one query.
	Many(ctx context.Context, ids []string) ([]domain.Merchant, error)
	BySlug(ctx context.Context, slug string) (*domain.Merchant, error)
	Lock(ctx context.Context, id string) (*domain.Merchant, error)
	SlugTaken(ctx context.Context, slug string) (bool, error)
	Save(ctx context.Context, m *domain.Merchant) error
	SetFee(ctx context.Context, id string, bps *int) error
	SetBeta(ctx context.Context, id string, beta bool) error
	List(ctx context.Context, f MerchantFilter) ([]domain.Merchant, error)
	Count(ctx context.Context, statuses []string) (int64, error)
	// CountMatching counts what List would return without its limit.
	CountMatching(ctx context.Context, f MerchantFilter) (int64, error)
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
	Last  *time.Time
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
	SessionID     string
	Live          *bool
	Statuses      []string
	Source        string
	LinkID        string
	Search        string
	Method        string
	PaidLate      bool // only those paid after they were left unconfirmed
	Duplicates    bool // only those that may duplicate another payment
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
	// CountCustomerFailures counts the failed payments a customer started in a
	// session, leaving out the tries RIHA made by itself.
	CountCustomerFailures(ctx context.Context, sessionID string) (int64, error)
	CountSettledByLink(ctx context.Context, linkID string) (int64, error)
	Find(ctx context.Context, q PaymentQuery) ([]domain.Payment, error)
	Count(ctx context.Context, q PaymentQuery) (int64, error)
	StaleProcessing(ctx context.Context, before time.Time, limit int) ([]domain.Payment, error)
	ByRailReference(ctx context.Context, ref string) (*domain.Payment, error)
	BySessionAttempt(ctx context.Context, sessionID, attemptID string) (*domain.Payment, error)
	// RecentlyTimedOut lists live payments that timed out since a time.
	RecentlyTimedOut(ctx context.Context, since time.Time, limit int) ([]domain.Payment, error)
	// DueRechecks lists unconfirmed payments due to be asked about again.
	DueRechecks(ctx context.Context, now time.Time, limit int) ([]domain.Payment, error)
	SetRecheck(ctx context.Context, id string, at time.Time, rechecks int) error
	// LaterTwin finds a payment the same customer made to the same merchant for
	// the same amount after this one, and that succeeded: the retry this one
	// duplicates if it turns out paid.
	LaterTwin(ctx context.Context, p *domain.Payment) (*domain.Payment, error)
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
	MarkVerified(ctx context.Context, merchantID, id string, at time.Time) error
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
	// Claim records a key before its request runs; false when it is taken.
	Claim(ctx context.Context, r *domain.IdempotencyRecord) (bool, error)
	Finish(ctx context.Context, id string, status int, body string) error
	// Release frees a claim whose request failed, for the client to retry.
	Release(ctx context.Context, id string) error
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

// MerchantSummary is what a merchant list shows of each: live volume paid
// since a day, its main method and its live balance.
type MerchantSummary struct {
	MerchantID string
	Volume     int64
	MainMethod string
	Balance    int64
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
	// RollupDrift counts rollup rows that differ from adding the rows up
	// again; RebuildRollups adds them up again.
	// PaymentTotal counts payments from the payment_days rollup: live or
	// test, of a merchant and a method ("" for any), in these statuses (any
	// when empty).
	PaymentTotal(ctx context.Context, live bool, merchantID, method string, statuses []string) (int64, error)
	// MerchantSummaries reads the summaries of many merchants at once, from
	// the rollups.
	MerchantSummaries(ctx context.Context, ids []string, since time.Time) (map[string]MerchantSummary, error)
	RollupDrift(ctx context.Context) (int64, error)
	RebuildRollups(ctx context.Context) error
	// Prune deletes request logs, and events whose deliveries are over,
	// older than before.
	Prune(ctx context.Context, before time.Time) (int64, error)
}

type Rail interface {
	Initiate(ctx context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error)
}

// ErrSheetTooWide is a spreadsheet with data far to the right, refused
// before it is read.
var ErrSheetTooWide = errors.New("sheet has data past the widest column read")

// ErrNoStatusCheck means this rail can't be asked about a payment, as with the simulator.
var ErrNoStatusCheck = errors.New("rail has no status check")

// ErrStillPending means the operator has the payment and no final answer yet.
var ErrStillPending = errors.New("payment still pending at the operator")

// RailQuery is implemented by rails that can say what became of a payment whose
// answer never arrived. It returns false when the operator has no record of it.
type RailQuery interface {
	Query(ctx context.Context, q domain.StatusQuery) (domain.Outcome, bool, error)
}

// RailReferencer is implemented by rails with their own rules for a payment's reference.
type RailReferencer interface {
	NewReference(method string, live bool) string
}

// RailRouting is implemented by a rail made of integrations. RouteFor names the
// one a payment by method goes through when it asks for want (the default when
// want doesn't take the method, "" when none does); Routes lists, by method,
// those that take it, the default first; ReferenceVia is a new reference by the
// rules of the integration a payment goes through.
type RailRouting interface {
	RouteFor(want, method string, live bool) string
	Routes(live bool) map[string][]string
	ReferenceVia(integration, method string, live bool) string
}

type WebhookSender interface {
	Send(ctx context.Context, url string, headers map[string]string, body []byte) (status int, err error)
}

// WebhookReplier also returns the start of the merchant's reply, for the trail.
type WebhookReplier interface {
	SendReply(ctx context.Context, url string, headers map[string]string, body []byte) (status int, reply []byte, err error)
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
	CatalogMerchants(ctx context.Context, limit int) ([]string, error)
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
	ByToken(ctx context.Context, token string) (*domain.Member, error)
	Identity(ctx context.Context, merchantID string, live bool, phone, nameKey string) (*domain.Member, error)
	// Identities are every member with this phone and name, oldest first.
	Identities(ctx context.Context, merchantID string, live bool, phone, nameKey string) ([]domain.Member, error)
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
	Queued(ctx context.Context, now time.Time, limit int) ([]domain.Message, error)
	ByProviderID(ctx context.Context, providerID string) (*domain.Message, error)
	Failed(ctx context.Context, merchantID string, live bool, since time.Time, limit int) ([]domain.Message, error)
	CountRecent(ctx context.Context, memberID, kind string, since time.Time) (int64, error)
	CountRecentToPhone(ctx context.Context, merchantID, phone, kind string, since time.Time) (int64, error)
}

type SubscriptionSettingsRepo interface {
	Get(ctx context.Context, merchantID string) (*domain.SubscriptionSettings, error)
	Save(ctx context.Context, s *domain.SubscriptionSettings) error
	ByCatalogSlug(ctx context.Context, slug string) (*domain.SubscriptionSettings, error)
	WithCatalog(ctx context.Context) ([]domain.SubscriptionSettings, error)
}

type OutboundMessage struct {
	Channel string
	To      string
	Sender  string
	Body    string
	// Live is a message to a real customer. A test-mode message never leaves RIHA.
	Live bool
}

// SentMessage is what the provider said when it took a message: its id, which its
// delivery reports carry, and the channel it actually went on.
type SentMessage struct {
	ProviderID string
	Channel    string
}

// ErrRetryLater says the message wasn't sent but may be later: the provider is down,
// busy, or out of credit.
var ErrRetryLater = errors.New("retry later")

type Messenger interface {
	Send(ctx context.Context, m OutboundMessage) (SentMessage, error)
}

// MessageStatusChecker is a messenger that can ask its provider what became of a
// message it took: delivered, failed, or "" while it isn't final.
type MessageStatusChecker interface {
	MessageStatus(ctx context.Context, providerID string) (status, why string, err error)
}

// HealthReporter is a messenger that can say how its provider account is.
type HealthReporter interface {
	Health() (MessengerHealth, bool)
}

// MessengerHealth is what an SMS provider last said about the account, for the alerts.
type MessengerHealth struct {
	Provider string
	// Balance is the credit left in meticais, as of BalanceAt; -1 when not known yet.
	Balance    float64
	BalanceAt  time.Time
	LowBalance float64
	// AccountError is the last refusal that isn't about one message (no credit, a bad
	// key), and when; cleared by the next message that goes through.
	AccountError   string
	AccountErrorAt time.Time
}

type Sheet interface {
	Rows(name string, data []byte) ([][]string, error)
}

// Trail keeps what happened to checkouts and payments, step by step.
type Trail interface {
	Add(ev domain.TrailEvent)
	Thread(ctx context.Context, sessionID string, paymentIDs []string) ([]domain.TrailEvent, error)
	ByRequest(ctx context.Context, requestID string) ([]domain.TrailEvent, error)
	Recent(ctx context.Context, q TrailQuery) ([]domain.TrailEvent, error)
	Prune(ctx context.Context, bodiesBefore, eventsBefore time.Time) (int64, int64, error)
	Dropped() int64
	Written() int64
}

type TrailQuery struct {
	MerchantID string
	Kinds      []string
	Level      string
	Search     string
	Since      *time.Time
	Limit      int
}

// CollectedRow sums one merchant's live payments through one integration.
type CollectedRow struct {
	Integration string
	MerchantID  string
	Count       int64
	Amount      int64
	Fee         int64
	PlatformFee int64
	PartnerFee  int64
	Refunded    int64
}

// MerchantMoney is what RIHA owes merchants in live mode: their Balances, and
// the withdrawals InTransit, already off their balances; and what RIHA's own
// account has PaidOut to them (partners' parts of withdrawals aside).
type MerchantMoney struct {
	Balances, InTransit, PaidOut int64
}

// Integrations keeps the integrations and their deals, which merchants go
// through which, and what partners have paid RIHA.
type Integrations interface {
	List(ctx context.Context) ([]domain.Integration, error)
	Get(ctx context.Context, code string) (*domain.Integration, error)
	Save(ctx context.Context, i *domain.Integration) error
	// AddMissing adds those whose code isn't there yet, leaving the others as edited.
	AddMissing(ctx context.Context, is []domain.Integration) error
	// Routes and Deals are one merchant's, or with "" everyone's.
	Routes(ctx context.Context, merchantID string) ([]domain.MerchantRoute, error)
	SetRoute(ctx context.Context, r *domain.MerchantRoute) error
	DeleteRoute(ctx context.Context, merchantID, method string) error
	Deals(ctx context.Context, merchantID string) ([]domain.MerchantDeal, error)
	SetDeal(ctx context.Context, d *domain.MerchantDeal) error
	DeleteDeal(ctx context.Context, merchantID, integration string) error
	AddSettlement(ctx context.Context, s *domain.PartnerSettlement) error
	Settlement(ctx context.Context, id string) (*domain.PartnerSettlement, error)
	Settlements(ctx context.Context, integration string, limit int) ([]domain.PartnerSettlement, error)
	DeleteSettlement(ctx context.Context, id string) error
	SettledTotals(ctx context.Context) (map[string]int64, error)
	// Collected sums live paid payments by integration and merchant, paid in [from, to).
	Collected(ctx context.Context, from, to *time.Time) ([]CollectedRow, error)
	MerchantMoney(ctx context.Context) (MerchantMoney, error)
	// Unpriced are payments with an integration whose fee hasn't been divided yet.
	Unpriced(ctx context.Context, limit int) ([]domain.Payment, error)
	SetPrice(ctx context.Context, paymentID string, p domain.Price) error

	Partners(ctx context.Context) ([]domain.Partner, error)
	Partner(ctx context.Context, code string) (*domain.Partner, error)
	CreatePartner(ctx context.Context, p *domain.Partner) error
	SavePartner(ctx context.Context, p *domain.Partner) error
	AddMissingPartners(ctx context.Context, ps []domain.Partner) error

	CreateFunding(ctx context.Context, f *domain.PayoutFunding) error
	SaveFunding(ctx context.Context, f *domain.PayoutFunding) error
	LockFunding(ctx context.Context, id string) (*domain.PayoutFunding, error)
	// LockRoom serializes giving partners withdrawals to pay, until the
	// transaction ends.
	LockRoom(ctx context.Context) error
	// ReceiptUsed tells whether a receipt already paid another part or withdrawal.
	ReceiptUsed(ctx context.Context, hash, exceptFunding string) (bool, error)
	FundingOf(ctx context.Context, payoutIDs []string) ([]domain.PayoutFunding, error)
	// FundingFrom is parts paid from these sources, in these statuses, of
	// withdrawals in payoutStatuses (any, when empty), newest first.
	FundingFrom(ctx context.Context, sources, statuses, payoutStatuses []string, limit int) ([]domain.PayoutFunding, error)
	// HeldFor is what each integration still holds of a merchant's live money.
	HeldFor(ctx context.Context, merchantID string) (map[string]int64, error)
	// FundingTotals sums withdrawal parts by source, then status.
	FundingTotals(ctx context.Context) (map[string]map[string]int64, error)
	UnfundedPayouts(ctx context.Context, statuses []string) ([]domain.Payout, error)
	// CollectedDaily sums live paid payments through these integrations by day (Maputo), paid in [from, to).
	CollectedDaily(ctx context.Context, integrations []string, from, to time.Time) ([]DayRow, error)
}

// DayRow sums a day's live paid payments through some integrations.
type DayRow struct {
	Day        string // YYYY-MM-DD
	Count      int64
	Amount     int64
	PartnerFee int64
}
