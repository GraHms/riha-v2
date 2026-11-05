package domain

import "time"

const (
	RoleMerchant = "merchant"
	RoleAdmin    = "admin"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	Name         string
	Phone        string
	Role         string
	AdminTitle   string
	MerchantID   *string
	LastLoginAt  *time.Time
	CreatedAt    time.Time
}

type AuthSession struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

const (
	MerchantLead          = "lead"
	MerchantInReview      = "in_review"
	MerchantInfoRequested = "info_requested"
	MerchantActive        = "active"
	MerchantRejected      = "rejected"
	MerchantSuspended     = "suspended"
)

type Merchant struct {
	ID                  string
	Slug                string
	Name                string
	LegalName           string
	BusinessType        string
	NUIT                string
	Phone               string
	Email               string
	City                string
	OwnerName           string
	ExpectedVolume      string
	StatementDescriptor string
	AvatarColor         string
	Status              string
	FeeBps              *int
	SuccessURL          string
	CancelURL           string
	SigningSecretTest   string
	SigningSecretLive   string
	SMSNotifications    bool
	EmailNotifications  bool
	KycSubmittedAt      *time.Time
	ApprovedAt          *time.Time
	ReviewNote          string
	ReviewedBy          string
	CreatedAt           time.Time
}

func (m *Merchant) SigningSecret(live bool) string {
	if live {
		return m.SigningSecretLive
	}
	return m.SigningSecretTest
}

type Document struct {
	ID          string
	MerchantID  string
	Kind        string
	FileName    string
	StoredPath  string
	Size        int64
	ContentType string
	CreatedAt   time.Time
}

type BankAccount struct {
	ID         string
	MerchantID string
	BankName   string
	Holder     string
	NIB        string
	DocumentID *string
	Verified   bool
	UpdatedAt  time.Time
}

const (
	KeyPublishable = "publishable"
	KeySecret      = "secret"
)

type APIKey struct {
	ID         string
	MerchantID string
	Livemode   bool
	Kind       string
	Token      string
	Hash       string
	Last4      string
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedBy  string
	CreatedAt  time.Time
}

type PaymentLink struct {
	ID           string
	MerchantID   string
	Slug         string
	Livemode     bool
	Name         string
	Amount       int64
	CustomAmount bool
	Reusable     bool
	Methods      []string
	ExpiresAt    *time.Time
	Active       bool
	Kind         string
	CreatedBy    string
	CreatedVia   string
	Location     string
	CreatedAt    time.Time
}

const (
	SessionOpen     = "open"
	SessionComplete = "complete"
	SessionExpired  = "expired"
)

const (
	SourceAPI       = "api_checkout"
	SourceLink      = "payment_link"
	SourceQR        = "qr"
	SourceDashboard = "dashboard"
)

type LineItem struct {
	Name       string `json:"name"`
	Quantity   int64  `json:"quantity"`
	UnitAmount int64  `json:"unit_amount"`
}

type Origin struct {
	SourceType    string
	PaymentLinkID *string
	APIKeyID      *string
	Domain        string
	IP            string
	UserAgent     string
	Channel       string
	CreatedBy     string
}

type CheckoutSession struct {
	ID                 string
	MerchantID         string
	Livemode           bool
	Status             string
	Amount             int64
	Currency           string
	Description        string
	SuccessURL         string
	CancelURL          string
	CallbackURL        string
	ClientReferenceID  string
	CustomerName       string
	CustomerPhone      string
	CustomerEmail      string
	LineItems          []LineItem
	Metadata           map[string]string
	PaymentMethodTypes []string
	Locale             string
	ExpiresAt          time.Time
	CompletedAt        *time.Time
	PaymentID          *string
	Origin
	CreatedAt time.Time
}

func (cs *CheckoutSession) IsOpen(now time.Time) bool {
	return cs.Status == SessionOpen && cs.ExpiresAt.After(now)
}

const (
	PaymentProcessing        = "processing"
	PaymentSucceeded         = "succeeded"
	PaymentFailed            = "failed"
	PaymentRefunded          = "refunded"
	PaymentPartiallyRefunded = "partially_refunded"
)

var SettledStatuses = []string{PaymentSucceeded, PaymentRefunded, PaymentPartiallyRefunded}

type Payment struct {
	ID                string
	Reference         string
	MerchantID        string
	Livemode          bool
	SessionID         string
	Status            string
	Amount            int64
	FeeBps            int
	Fee               int64
	Net               int64
	AmountRefunded    int64
	Method            string
	PhonePrefix       string
	PhoneLast4        string
	CardBrand         string
	CardLast4         string
	OperatorReference string
	RailReference     string
	RailID            string
	AttemptID         string
	FailureCode       string
	FailureMessage    string
	CustomerName      string
	CustomerEmail     string
	Description       string
	ClientReferenceID string
	Metadata          map[string]string
	Origin
	PayerIP        string
	PayerUserAgent string
	SucceededAt    *time.Time
	FailedAt       *time.Time
	CreatedAt      time.Time
}

func (p *Payment) Refundable() int64 {
	if p.Status != PaymentSucceeded && p.Status != PaymentPartiallyRefunded {
		return 0
	}
	return p.Amount - p.AmountRefunded
}

type Refund struct {
	ID         string
	PaymentID  string
	MerchantID string
	Livemode   bool
	Amount     int64
	Reason     string
	Status     string
	CreatedBy  string
	CreatedAt  time.Time
}

const (
	LedgerPayment        = "payment"
	LedgerPayout         = "payout"
	LedgerPayoutReversal = "payout_reversal"
	LedgerRefund         = "refund"
)

type LedgerEntry struct {
	ID          string
	MerchantID  string
	Livemode    bool
	Type        string
	Amount      int64
	ReferenceID string
	Description string
	CreatedAt   time.Time
}

type WebhookEndpoint struct {
	ID         string
	MerchantID string
	Livemode   bool
	URL        string
	Events     []string
	Active     bool
	CreatedAt  time.Time
}

func (e *WebhookEndpoint) Wants(event string) bool {
	return e.Active && (contains(e.Events, event) || contains(e.Events, "*"))
}

type AllowedDomain struct {
	ID         string
	MerchantID string
	Domain     string
	Token      string
	VerifiedAt *time.Time
	CreatedAt  time.Time
}

type Event struct {
	ID         string
	MerchantID string
	Livemode   bool
	Type       string
	ObjectID   string
	Data       []byte
	CreatedAt  time.Time
}

const (
	TargetEndpoint = "endpoint"
	TargetCallback = "callback_url"

	DeliveryPending   = "pending"
	DeliverySucceeded = "succeeded"
	DeliveryFailed    = "failed"
)

type Delivery struct {
	ID             string
	EventID        string
	MerchantID     string
	Livemode       bool
	EventType      string
	Target         string
	EndpointID     *string
	URL            string
	Status         string
	Attempts       int
	NextAttemptAt  *time.Time
	LastStatusCode int
	LastError      string
	LastDurationMs int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RequestLog struct {
	ID         string
	MerchantID string
	Livemode   bool
	Method     string
	Path       string
	Status     int
	APIKeyID   string
	IP         string
	DurationMs int64
	RequestID  string
	CreatedAt  time.Time
}

type IdempotencyRecord struct {
	ID           string
	MerchantID   string
	Livemode     bool
	Key          string
	Path         string
	RequestHash  string
	StatusCode   int
	ResponseBody string
	CreatedAt    time.Time
}

type AuditEntry struct {
	ID         string
	MerchantID string
	ActorID    string
	ActorRole  string
	Action     string
	Target     string
	Details    string
	IP         string
	CreatedAt  time.Time
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func Contains(list []string, v string) bool { return contains(list, v) }
