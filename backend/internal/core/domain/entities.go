package domain

import (
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

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
	// Permissions are a staff or partner user's; PartnerCode is the partner a
	// partner's user belongs to. DisabledAt closes the account.
	Permissions []string
	PartnerCode string
	DisabledAt  *time.Time
	LastLoginAt *time.Time
	// FailedLogins counts wrong passwords in a row; LockedUntil stops
	// guessing for a while after too many.
	FailedLogins int
	LockedUntil  *time.Time
	CreatedAt    time.Time
}

// LoginCode is the code emailed to staff and partners signing in on a device
// RIHA doesn't know; TrustedDevice is one that entered it.
type LoginCode struct {
	UserID    string
	CodeHash  string
	Attempts  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

type TrustedDevice struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time
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
	// Beta accounts may charge from 1 MT, plans included, to test with real money.
	Beta bool
	// Microcredit is granted by a RIHA super admin; without it the account has no
	// access to anything in the Microcrédito service.
	MicrocreditAt   *time.Time
	MicrocreditBy   string
	MicrocreditNote string
	// ImportedFrom is "v1" for an account brought over from the first RIHA, and
	// V1ID is its id there, so an import run twice doesn't bring it in twice.
	ImportedFrom       string
	V1ID               string
	SuccessURL         string
	CancelURL          string
	SigningSecretTest  string
	SigningSecretLive  string
	SMSNotifications   bool
	EmailNotifications bool
	KycSubmittedAt     *time.Time
	ApprovedAt         *time.Time
	ReviewNote         string
	ReviewedBy         string
	CreatedAt          time.Time
}

func (m *Merchant) HasMicrocredit() bool { return m != nil && m.MicrocreditAt != nil }

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
	// ChangedAt is when an approved merchant last replaced the account: live
	// withdrawals wait BankChangeHold, so a stolen session can't empty the
	// balance into an account of its own before the owner sees the email.
	ChangedAt *time.Time
	UpdatedAt time.Time
}

const BankChangeHold = 48 * time.Hour

// SameHolder tells whether an account holder is one of names, ignoring case,
// accents, punctuation and company suffixes.
func SameHolder(holder string, names ...string) bool {
	h := holderKey(holder)
	for _, n := range names {
		if k := holderKey(n); k != "" && (k == h || strings.Contains(h, k) || strings.Contains(k, h)) {
			return true
		}
	}
	return h == ""
}

func holderKey(s string) string {
	s = strings.ToLower(norm.NFD.String(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '.' || r == ',' || r == '-' || r == '&':
			b.WriteRune(' ')
		}
	}
	words := []string{}
	for _, w := range strings.Fields(b.String()) {
		switch w {
		case "lda", "limitada", "sa", "s", "a", "e", "de", "da", "do", "sociedade", "unipessoal", "su", "ei":
			continue
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// PayableFrom is when a replaced account can take live withdrawals.
func (b *BankAccount) PayableFrom() *time.Time {
	if b.ChangedAt == nil {
		return nil
	}
	t := b.ChangedAt.Add(BankChangeHold)
	return &t
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
	// What the customer reads and sees before paying.
	Details string
	Image   string
	// For a link where the customer chooses the amount. 0 means not set.
	SuggestedAmount int64
	MinCustom       int64
	MaxCustom       int64
	// After paying, the customer reads AfterMessage, or goes to RedirectURL.
	AfterMessage string
	RedirectURL  string
	// SubmitType names the checkout button: pay, book or donate.
	SubmitType string
	// Archived links are off and out of the way, but kept: payments point to them.
	Archived  bool
	CreatedAt time.Time
}

// CustomRange is the lowest and highest amount a customer may choose; min is the
// merchant's least (MinAmountFor) when the link sets none.
func (l *PaymentLink) CustomRange(min int64) (int64, int64) {
	lo, hi := min, int64(MaxAmount)
	if l.MinCustom > 0 {
		lo = l.MinCustom
	}
	if l.MaxCustom > 0 {
		hi = l.MaxCustom
	}
	return lo, hi
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
	ID          string
	MerchantID  string
	Livemode    bool
	Status      string
	Amount      int64
	Currency    string
	Description string
	SuccessURL  string
	CancelURL   string
	CallbackURL string
	// SuccessMessage is shown once the customer has paid, from a payment link.
	SuccessMessage     string
	SubmitType         string
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
	PaymentProcessing = "processing"
	// PaymentUnconfirmed: the operator gave no final answer in time. The customer
	// may still have paid, so it isn't failed: RIHA keeps asking and settles it
	// either way when the answer comes.
	PaymentUnconfirmed       = "unconfirmed"
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
	// ContactPhone is the customer's full number, kept only until the payment
	// is final, to tell them by SMS how an unconfirmed payment ended.
	ContactPhone string
	// RecheckAt is when an unconfirmed payment is next asked about, and
	// Rechecks how often it has been.
	RecheckAt *time.Time
	Rechecks  int
	// PaidLate: the operator confirmed it only after it had left processing.
	PaidLate bool
	// DuplicateOf is the payment the same customer already paid for the same
	// thing, when this one came in late on top of it: a refund to look at.
	DuplicateOf string
	// Retry counts the automatic tries before this one, after the operator
	// failed the transaction: 0 when the customer pressed pay.
	Retry int
	// Integration is the code of the integration that took the payment, which
	// holds its money until it settles with RIHA. PlatformFee and PartnerFee are
	// how RIHA's fee divided on that integration's deal when the payment began;
	// Priced says they were worked out, since both can be zero.
	Integration string
	PlatformFee int64
	PartnerFee  int64
	Priced      bool
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
	LedgerPayment         = "payment"
	LedgerPayout          = "payout"
	LedgerPayoutReversal  = "payout_reversal"
	LedgerTransferFee     = "transfer_fee"
	LedgerRefund          = "refund"
	LedgerLoanOut         = "loan_out"
	LedgerLoanOutReversal = "loan_out_reversal"
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

// EventPing is the test event: every active endpoint takes it, and no
// integration can mistake it for money.
const EventPing = "ping"

func (e *WebhookEndpoint) Wants(event string) bool {
	return e.Active && (event == EventPing || contains(e.Events, event) || contains(e.Events, "*"))
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

const (
	SubmitPay    = "pay"
	SubmitBook   = "book"
	SubmitDonate = "donate"
)

// CleanSubmitType checks the word on a checkout's button. Empty means pay, and
// so do Stripe's auto and subscribe, so that code written for Stripe keeps working.
func CleanSubmitType(s string) (string, error) {
	switch s = strings.ToLower(strings.TrimSpace(s)); s {
	case "", "auto", "subscribe":
		return SubmitPay, nil
	case SubmitPay, SubmitBook, SubmitDonate:
		return s, nil
	}
	return s, Invalid("parameter_invalid", "submit_type", "submit_type tem de ser pay, book ou donate.")
}

func (l *PaymentLink) SubmitTypeOrPay() string { return FirstNonEmpty(l.SubmitType, SubmitPay) }
