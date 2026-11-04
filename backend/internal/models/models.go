// Package models holds the GORM persistence models for Riha.
//
// Money is always stored as int64 centavos of metical (125000 = 1 250,00 MT).
// Every merchant-scoped record carries Livemode so that test and production
// data never mix.
package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// StringList is a []string persisted as JSON text.
type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(s))
	return string(b), err
}

func (s *StringList) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*s = nil
		return nil
	case string:
		return json.Unmarshal([]byte(t), (*[]string)(s))
	case []byte:
		return json.Unmarshal(t, (*[]string)(s))
	}
	return errors.New("StringList: unsupported type")
}

func (s StringList) Has(v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// JSONMap is a map persisted as JSON text (metadata, customer, etc).
type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]any(m))
	return string(b), err
}

func (m *JSONMap) Scan(v any) error {
	var raw []byte
	switch t := v.(type) {
	case nil:
		*m = JSONMap{}
		return nil
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	default:
		return errors.New("JSONMap: unsupported type")
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	*m = out
	return nil
}

// LineItem shown on the checkout and receipt.
type LineItem struct {
	Name       string `json:"name"`
	Quantity   int64  `json:"quantity"`
	UnitAmount int64  `json:"unit_amount"`
}

type LineItems []LineItem

func (l LineItems) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]LineItem(l))
	return string(b), err
}

func (l *LineItems) Scan(v any) error {
	switch t := v.(type) {
	case nil:
		*l = nil
		return nil
	case string:
		return json.Unmarshal([]byte(t), (*[]LineItem)(l))
	case []byte:
		return json.Unmarshal(t, (*[]LineItem)(l))
	}
	return errors.New("LineItems: unsupported type")
}

// ---------------------------------------------------------------- accounts

const (
	RoleMerchant = "merchant"
	RoleAdmin    = "admin"
)

type User struct {
	ID           string  `gorm:"primaryKey;size:40"`
	Email        string  `gorm:"uniqueIndex;size:200;not null"`
	PasswordHash string  `gorm:"not null" json:"-"`
	Name         string  `gorm:"size:200"`
	Phone        string  `gorm:"size:40"`
	Role         string  `gorm:"size:20;not null;default:merchant"`
	AdminTitle   string  `gorm:"size:80"`
	MerchantID   *string `gorm:"size:40;index"`
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        string `gorm:"primaryKey;size:80"` // sha256 of the cookie token
	UserID    string `gorm:"size:40;index;not null"`
	ExpiresAt time.Time
	IP        string `gorm:"size:64"`
	UserAgent string `gorm:"size:300"`
	CreatedAt time.Time
}

// Merchant statuses.
const (
	MerchantOnboarding    = "onboarding"     // signed up, KYC not submitted
	MerchantInReview      = "in_review"      // KYC submitted, waiting for admin
	MerchantInfoRequested = "info_requested" // admin asked for more documents
	MerchantActive        = "active"
	MerchantRejected      = "rejected"
	MerchantSuspended     = "suspended"
)

type Merchant struct {
	ID                  string `gorm:"primaryKey;size:40"`
	Slug                string `gorm:"uniqueIndex;size:80;not null"`
	Name                string `gorm:"size:200;not null"` // trading name
	LegalName           string `gorm:"size:200"`
	BusinessType        string `gorm:"size:20"` // individual | company
	NUIT                string `gorm:"column:nuit;size:20"`
	Phone               string `gorm:"size:40"`
	Email               string `gorm:"size:200"`
	City                string `gorm:"size:80"`
	OwnerName           string `gorm:"size:200"`
	ExpectedVolume      string `gorm:"size:80"`
	StatementDescriptor string `gorm:"size:40"`
	AvatarColor         string `gorm:"size:12"`
	Status              string `gorm:"size:20;index;not null"`
	// FeeBps is the negotiated fee in basis points (600 = 6%). NULL means the
	// platform default applies.
	FeeBps            *int
	SuccessURL        string `gorm:"size:500"`
	CancelURL         string `gorm:"size:500"`
	SigningSecretTest string `gorm:"size:80" json:"-"`
	SigningSecretLive string `gorm:"size:80" json:"-"`
	SMSNotifications  bool   `gorm:"default:true"`
	KycSubmittedAt    *time.Time
	ApprovedAt        *time.Time
	ReviewNote        string `gorm:"size:1000"`
	ReviewedBy        string `gorm:"size:40"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (m *Merchant) SigningSecret(live bool) string {
	if live {
		return m.SigningSecretLive
	}
	return m.SigningSecretTest
}

type KycDocument struct {
	ID          string `gorm:"primaryKey;size:40"`
	MerchantID  string `gorm:"size:40;index;not null"`
	Kind        string `gorm:"size:30;not null"` // id_front | id_back | registration | nib_proof
	FileName    string `gorm:"size:200"`
	StoredPath  string `gorm:"size:500" json:"-"`
	Size        int64
	ContentType string `gorm:"size:100"`
	CreatedAt   time.Time
}

type BankAccount struct {
	ID         string  `gorm:"primaryKey;size:40"`
	MerchantID string  `gorm:"uniqueIndex;size:40;not null"`
	BankName   string  `gorm:"size:80"`
	Holder     string  `gorm:"size:200"`
	NIB        string  `gorm:"column:nib;size:21"`
	DocumentID *string `gorm:"size:40"`
	Verified   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

const (
	KeyPublishable = "publishable"
	KeySecret      = "secret"
)

type APIKey struct {
	ID         string `gorm:"primaryKey;size:40"` // key_xxxx
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	Kind       string `gorm:"size:20"`
	// Token is stored in clear only for publishable keys (they are public).
	Token      string `gorm:"size:120;index"`
	Hash       string `gorm:"size:80;index"` // sha256 hex of secret keys
	Last4      string `gorm:"size:8"`
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedBy  string `gorm:"size:40"`
	CreatedAt  time.Time
}

// ---------------------------------------------------------------- payments

type PaymentLink struct {
	ID           string `gorm:"primaryKey;size:40"`
	MerchantID   string `gorm:"size:40;uniqueIndex:idx_link_slug;not null"`
	Slug         string `gorm:"size:80;uniqueIndex:idx_link_slug;not null"`
	Livemode     bool
	Name         string `gorm:"size:200"`
	Amount       int64  // 0 when CustomAmount
	CustomAmount bool
	Reusable     bool
	Methods      StringList `gorm:"type:text"`
	ExpiresAt    *time.Time
	Active       bool
	Kind         string `gorm:"size:20;default:link"` // link | qr
	CreatedBy    string `gorm:"size:40"`
	CreatedVia   string `gorm:"size:20"` // dashboard | api
	Location     string `gorm:"size:120"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

const (
	SessionOpen     = "open"
	SessionComplete = "complete"
	SessionExpired  = "expired"
)

// Payment source types (where did the money come from).
const (
	SourceAPI       = "api_checkout"
	SourceLink      = "payment_link"
	SourceQR        = "qr"
	SourceDashboard = "dashboard"
)

type CheckoutSession struct {
	ID                 string `gorm:"primaryKey;size:60"`
	MerchantID         string `gorm:"size:40;index;not null"`
	Livemode           bool
	Status             string `gorm:"size:20;index"`
	Amount             int64
	Currency           string     `gorm:"size:3"`
	Description        string     `gorm:"size:500"`
	SuccessURL         string     `gorm:"size:1000"`
	CancelURL          string     `gorm:"size:1000"`
	CallbackURL        string     `gorm:"size:1000"`
	ClientReferenceID  string     `gorm:"size:200;index"`
	CustomerName       string     `gorm:"size:200"`
	CustomerPhone      string     `gorm:"size:40"`
	CustomerEmail      string     `gorm:"size:200"`
	LineItems          LineItems  `gorm:"type:text"`
	Metadata           JSONMap    `gorm:"type:text"`
	PaymentMethodTypes StringList `gorm:"type:text"`
	Locale             string     `gorm:"size:10"`
	ExpiresAt          time.Time  `gorm:"index"`
	CompletedAt        *time.Time
	PaymentID          *string `gorm:"size:40"`
	// Source / origin
	SourceType      string  `gorm:"size:20"`
	PaymentLinkID   *string `gorm:"size:40;index"`
	APIKeyID        *string `gorm:"size:40"`
	OriginDomain    string  `gorm:"size:200"`
	OriginIP        string  `gorm:"size:64"`
	OriginUserAgent string  `gorm:"size:300"`
	Channel         string  `gorm:"size:20"`
	CreatedBy       string  `gorm:"size:40"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

const (
	PaymentProcessing        = "processing"
	PaymentSucceeded         = "succeeded"
	PaymentFailed            = "failed"
	PaymentRefunded          = "refunded"
	PaymentPartiallyRefunded = "partially_refunded"
)

type Payment struct {
	ID                string `gorm:"primaryKey;size:40"`
	Reference         string `gorm:"size:20;uniqueIndex"`
	MerchantID        string `gorm:"size:40;index;not null"`
	Livemode          bool   `gorm:"index"`
	SessionID         string `gorm:"size:60;index"`
	Status            string `gorm:"size:24;index"`
	Amount            int64
	FeeBps            int
	Fee               int64
	Net               int64
	AmountRefunded    int64
	Method            string  `gorm:"size:10"` // mpesa | emola | card
	PhonePrefix       string  `gorm:"size:4"`
	PhoneLast4        string  `gorm:"size:4"`
	CardBrand         string  `gorm:"size:20"`
	CardLast4         string  `gorm:"size:4"`
	OperatorReference string  `gorm:"size:40"`
	FailureCode       string  `gorm:"size:40"`
	FailureMessage    string  `gorm:"size:300"`
	CustomerName      string  `gorm:"size:200"`
	CustomerEmail     string  `gorm:"size:200"`
	Description       string  `gorm:"size:500"`
	ClientReferenceID string  `gorm:"size:200"`
	Metadata          JSONMap `gorm:"type:text"`
	SourceType        string  `gorm:"size:20;index"`
	PaymentLinkID     *string `gorm:"size:40;index"`
	APIKeyID          *string `gorm:"size:40"`
	OriginDomain      string  `gorm:"size:200"`
	OriginIP          string  `gorm:"size:64"`
	OriginUserAgent   string  `gorm:"size:300"`
	Channel           string  `gorm:"size:20"`
	CreatedBy         string  `gorm:"size:40"`
	PayerIP           string  `gorm:"size:64"`
	PayerUserAgent    string  `gorm:"size:300"`
	SucceededAt       *time.Time
	FailedAt          *time.Time
	CreatedAt         time.Time `gorm:"index"`
	UpdatedAt         time.Time
}

type Refund struct {
	ID         string `gorm:"primaryKey;size:40"`
	PaymentID  string `gorm:"size:40;index;not null"`
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	Amount     int64
	Reason     string `gorm:"size:200"`
	Status     string `gorm:"size:20"`
	CreatedBy  string `gorm:"size:40"`
	CreatedAt  time.Time
}

// Ledger entry types.
const (
	LedgerPayment        = "payment"
	LedgerPayout         = "payout"
	LedgerPayoutReversal = "payout_reversal"
	LedgerRefund         = "refund"
	LedgerAdjustment     = "adjustment"
)

// LedgerEntry is the single source of truth for balances: the available
// balance of a merchant in a mode is SUM(amount).
type LedgerEntry struct {
	ID          string `gorm:"primaryKey;size:40"`
	MerchantID  string `gorm:"size:40;index:idx_ledger_m;not null"`
	Livemode    bool   `gorm:"index:idx_ledger_m"`
	Type        string `gorm:"size:20"`
	Amount      int64
	ReferenceID string `gorm:"size:60;index"`
	Description string `gorm:"size:300"`
	CreatedAt   time.Time
}

// Payout statuses. new → review → sent → paid; or rejected/failed (reversed).
const (
	PayoutNew      = "new"
	PayoutReview   = "review"
	PayoutSent     = "sent"
	PayoutPaid     = "paid"
	PayoutRejected = "rejected"
	PayoutFailed   = "failed"
)

type Payout struct {
	ID            string `gorm:"primaryKey;size:40"`
	Number        int64  `gorm:"uniqueIndex"`
	MerchantID    string `gorm:"size:40;index;not null"`
	Livemode      bool   `gorm:"index"`
	Amount        int64
	Status        string `gorm:"size:20;index"`
	BankName      string `gorm:"size:80"`
	Holder        string `gorm:"size:200"`
	NIB           string `gorm:"column:nib;size:21"`
	RequestedBy   string `gorm:"size:40"`
	RequestedAt   time.Time
	DueAt         time.Time
	ReviewedAt    *time.Time
	SentAt        *time.Time
	PaidAt        *time.Time
	ClosedAt      *time.Time
	FailureReason string `gorm:"size:300"`
	ReviewedBy    string `gorm:"size:40"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ---------------------------------------------------------------- webhooks

type WebhookEndpoint struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	URL        string     `gorm:"size:1000"`
	Events     StringList `gorm:"type:text"`
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type AllowedDomain struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;uniqueIndex:idx_domain;not null"`
	Domain     string `gorm:"size:200;uniqueIndex:idx_domain;not null"`
	Token      string `gorm:"size:80"`
	VerifiedAt *time.Time
	CreatedAt  time.Time
}

type Event struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	Type       string `gorm:"size:60;index"`
	ObjectID   string `gorm:"size:60;index"`
	Data       string `gorm:"type:text"` // JSON of data.object
	CreatedAt  time.Time
}

const (
	TargetEndpoint = "endpoint"
	TargetCallback = "callback_url"

	DeliveryPending   = "pending"
	DeliverySucceeded = "succeeded"
	DeliveryFailed    = "failed"
)

type WebhookDelivery struct {
	ID             string `gorm:"primaryKey;size:40"`
	EventID        string `gorm:"size:40;index;not null"`
	MerchantID     string `gorm:"size:40;index;not null"`
	Livemode       bool
	EventType      string  `gorm:"size:60"`
	Target         string  `gorm:"size:20"`
	EndpointID     *string `gorm:"size:40"`
	URL            string  `gorm:"size:1000"`
	Status         string  `gorm:"size:20;index"`
	Attempts       int
	NextAttemptAt  *time.Time `gorm:"index"`
	LastStatusCode int
	LastError      string `gorm:"size:500"`
	LastDurationMs int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ---------------------------------------------------------------- platform

type APIRequestLog struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index"`
	Livemode   bool
	Method     string `gorm:"size:10"`
	Path       string `gorm:"size:300"`
	Status     int
	APIKeyID   string `gorm:"size:40"`
	IP         string `gorm:"size:64"`
	DurationMs int64
	RequestID  string    `gorm:"size:40"`
	CreatedAt  time.Time `gorm:"index"`
}

type IdempotencyRecord struct {
	ID           string `gorm:"primaryKey;size:40"`
	MerchantID   string `gorm:"size:40;uniqueIndex:idx_idem;not null"`
	Livemode     bool   `gorm:"uniqueIndex:idx_idem"`
	Key          string `gorm:"size:255;uniqueIndex:idx_idem;not null"`
	Path         string `gorm:"size:300"`
	RequestHash  string `gorm:"size:80"`
	StatusCode   int
	ResponseBody string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"index"`
}

type AuditLog struct {
	ID         string    `gorm:"primaryKey;size:40"`
	MerchantID string    `gorm:"size:40;index"`
	ActorID    string    `gorm:"size:40"`
	ActorRole  string    `gorm:"size:20"`
	Action     string    `gorm:"size:80"`
	Target     string    `gorm:"size:80"`
	Details    string    `gorm:"size:1000"`
	IP         string    `gorm:"size:64"`
	CreatedAt  time.Time `gorm:"index"`
}

type PlatformSetting struct {
	Key       string `gorm:"primaryKey;size:80"`
	Value     string `gorm:"size:500"`
	UpdatedAt time.Time
}

// All returns every model for AutoMigrate.
func All() []any {
	return []any{
		&User{}, &Session{}, &Merchant{}, &KycDocument{}, &BankAccount{}, &APIKey{},
		&PaymentLink{}, &CheckoutSession{}, &Payment{}, &Refund{}, &LedgerEntry{}, &Payout{},
		&WebhookEndpoint{}, &AllowedDomain{}, &Event{}, &WebhookDelivery{},
		&APIRequestLog{}, &IdempotencyRecord{}, &AuditLog{}, &PlatformSetting{},
	}
}
