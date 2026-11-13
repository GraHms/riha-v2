package postgres

import "time"

type userRecord struct {
	ID           string     `gorm:"primaryKey;size:40"`
	Email        string     `gorm:"uniqueIndex;size:200;not null"`
	PasswordHash string     `gorm:"not null"`
	Name         string     `gorm:"size:200"`
	Phone        string     `gorm:"size:40"`
	Role         string     `gorm:"size:20;not null;default:merchant"`
	AdminTitle   string     `gorm:"size:80"`
	MerchantID   *string    `gorm:"size:40;index"`
	Permissions  StringList `gorm:"type:text"`
	PartnerCode  string     `gorm:"size:30;not null;default:'';index"`
	DisabledAt   *time.Time
	FailedLogins int
	LockedUntil  *time.Time
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (userRecord) TableName() string { return "users" }

type sessionRecord struct {
	TokenHash string `gorm:"column:id;primaryKey;size:80"`
	UserID    string `gorm:"size:40;index;not null"`
	ExpiresAt time.Time
	IP        string `gorm:"size:64"`
	UserAgent string `gorm:"size:300"`
	CreatedAt time.Time
}

func (sessionRecord) TableName() string { return "sessions" }

type merchantRecord struct {
	ID                  string `gorm:"primaryKey;size:40"`
	Slug                string `gorm:"uniqueIndex;size:80;not null"`
	Name                string `gorm:"size:200;not null"`
	LegalName           string `gorm:"size:200"`
	BusinessType        string `gorm:"size:20"`
	NUIT                string `gorm:"column:nuit;size:20"`
	Phone               string `gorm:"size:40"`
	Email               string `gorm:"size:200"`
	City                string `gorm:"size:80"`
	OwnerName           string `gorm:"size:200"`
	ExpectedVolume      string `gorm:"size:80"`
	StatementDescriptor string `gorm:"size:40"`
	AvatarColor         string `gorm:"size:12"`
	Status              string `gorm:"size:20;index;not null"`
	FeeBps              *int
	Beta                bool
	ImportedFrom        string `gorm:"size:10;not null;default:''"`
	V1ID                string `gorm:"column:v1_id;size:40;not null;default:'';index"`
	SuccessURL          string `gorm:"size:500"`
	CancelURL           string `gorm:"size:500"`
	SigningSecretTest   string `gorm:"size:80"`
	SigningSecretLive   string `gorm:"size:80"`
	SMSNotifications    bool   `gorm:"default:true"`
	EmailNotifications  bool   `gorm:"default:true"`
	KycSubmittedAt      *time.Time
	ApprovedAt          *time.Time
	ReviewNote          string `gorm:"size:1000"`
	ReviewedBy          string `gorm:"size:40"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (merchantRecord) TableName() string { return "merchants" }

type documentRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	MerchantID  string `gorm:"size:40;index;not null"`
	Kind        string `gorm:"size:30;not null"`
	FileName    string `gorm:"size:200"`
	StoredPath  string `gorm:"size:500"`
	Size        int64
	ContentType string `gorm:"size:100"`
	CreatedAt   time.Time
}

func (documentRecord) TableName() string { return "kyc_documents" }

type bankRecord struct {
	ID         string  `gorm:"primaryKey;size:40"`
	MerchantID string  `gorm:"uniqueIndex;size:40;not null"`
	BankName   string  `gorm:"size:80"`
	Holder     string  `gorm:"size:200"`
	NIB        string  `gorm:"column:nib;size:21"`
	DocumentID *string `gorm:"size:40"`
	Verified   bool
	ChangedAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (bankRecord) TableName() string { return "bank_accounts" }

type keyRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	Kind       string `gorm:"size:20"`
	Token      string `gorm:"size:120;index"`
	Hash       string `gorm:"size:80;index"`
	Last4      string `gorm:"size:8"`
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedBy  string `gorm:"size:40"`
	CreatedAt  time.Time
}

func (keyRecord) TableName() string { return "api_keys" }

type linkRecord struct {
	ID              string `gorm:"primaryKey;size:40"`
	MerchantID      string `gorm:"size:40;uniqueIndex:idx_link_slug;not null"`
	Slug            string `gorm:"size:80;uniqueIndex:idx_link_slug;not null"`
	Livemode        bool
	Name            string `gorm:"size:200"`
	Amount          int64
	CustomAmount    bool
	Reusable        bool
	Methods         StringList `gorm:"type:text"`
	ExpiresAt       *time.Time
	Active          bool
	Kind            string `gorm:"size:20;default:link"`
	CreatedBy       string `gorm:"size:40"`
	CreatedVia      string `gorm:"size:20"`
	Location        string `gorm:"size:120"`
	Details         string `gorm:"size:500"`
	Image           string `gorm:"size:300"`
	SuggestedAmount int64
	MinCustom       int64
	MaxCustom       int64
	AfterMessage    string `gorm:"size:300"`
	RedirectURL     string `gorm:"size:1000"`
	SubmitType      string `gorm:"size:10"`
	Archived        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (linkRecord) TableName() string { return "payment_links" }

type originCols struct {
	SourceType    string  `gorm:"size:20"`
	PaymentLinkID *string `gorm:"size:40;index"`
	APIKeyID      *string `gorm:"column:api_key_id;size:40"`
	Domain        string  `gorm:"column:origin_domain;size:200"`
	IP            string  `gorm:"column:origin_ip;size:64"`
	UserAgent     string  `gorm:"column:origin_user_agent;size:300"`
	Channel       string  `gorm:"size:20"`
	CreatedBy     string  `gorm:"size:40"`
}

type sessionCheckoutRecord struct {
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
	SuccessMessage     string     `gorm:"size:300"`
	SubmitType         string     `gorm:"size:10"`
	ClientReferenceID  string     `gorm:"size:200;index"`
	CustomerName       string     `gorm:"size:200"`
	CustomerPhone      string     `gorm:"size:40"`
	CustomerEmail      string     `gorm:"size:200"`
	LineItems          LineItems  `gorm:"type:text"`
	Metadata           StringMap  `gorm:"type:text"`
	PaymentMethodTypes StringList `gorm:"type:text"`
	Locale             string     `gorm:"size:10"`
	ExpiresAt          time.Time  `gorm:"index"`
	CompletedAt        *time.Time
	PaymentID          *string    `gorm:"size:40"`
	Origin             originCols `gorm:"embedded"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (sessionCheckoutRecord) TableName() string { return "checkout_sessions" }

type paymentRecord struct {
	ID                string `gorm:"primaryKey;size:40"`
	Reference         string `gorm:"size:20;uniqueIndex"`
	MerchantID        string `gorm:"size:40;index;not null"`
	Livemode          bool   `gorm:"index"`
	SessionID         string `gorm:"size:60;index;uniqueIndex:idx_payment_attempt,priority:1"`
	Status            string `gorm:"size:24;index"`
	Amount            int64
	FeeBps            int
	Fee               int64
	Net               int64
	AmountRefunded    int64
	Method            string     `gorm:"size:10"`
	PhonePrefix       string     `gorm:"size:4"`
	PhoneLast4        string     `gorm:"size:4"`
	CardBrand         string     `gorm:"size:20"`
	CardLast4         string     `gorm:"size:4"`
	OperatorReference string     `gorm:"size:40"`
	RailReference     string     `gorm:"size:20;index"`
	RailID            string     `gorm:"size:64"`
	AttemptID         string     `gorm:"size:20;uniqueIndex:idx_payment_attempt,priority:2,where:attempt_id <> ''"`
	FailureCode       string     `gorm:"size:40"`
	FailureMessage    string     `gorm:"size:300"`
	CustomerName      string     `gorm:"size:200"`
	CustomerEmail     string     `gorm:"size:200"`
	Description       string     `gorm:"size:500"`
	ClientReferenceID string     `gorm:"size:200"`
	Metadata          StringMap  `gorm:"type:text"`
	Origin            originCols `gorm:"embedded"`
	PayerIP           string     `gorm:"size:64"`
	PayerUserAgent    string     `gorm:"size:300"`
	SucceededAt       *time.Time
	FailedAt          *time.Time
	CreatedAt         time.Time `gorm:"index"`
	UpdatedAt         time.Time
	ContactPhone      string     `gorm:"size:20;not null;default:''"`
	RecheckAt         *time.Time `gorm:"index"`
	Rechecks          int        `gorm:"not null;default:0"`
	PaidLate          bool       `gorm:"not null;default:false"`
	DuplicateOf       string     `gorm:"size:40;not null;default:''"`
	Retry             int        `gorm:"not null;default:0"`
	Integration       string     `gorm:"size:30;not null;default:'';index"`
	PlatformFee       int64      `gorm:"not null;default:0"`
	PartnerFee        int64      `gorm:"not null;default:0"`
	Priced            bool       `gorm:"not null;default:false"`
}

func (paymentRecord) TableName() string { return "payments" }

type refundRecord struct {
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

func (refundRecord) TableName() string { return "refunds" }

type ledgerRecord struct {
	ID          string `gorm:"primaryKey;size:40"`
	MerchantID  string `gorm:"size:40;index:idx_ledger_m;not null"`
	Livemode    bool   `gorm:"index:idx_ledger_m"`
	Type        string `gorm:"size:20"`
	Amount      int64
	ReferenceID string `gorm:"size:60;index"`
	Description string `gorm:"size:300"`
	CreatedAt   time.Time
}

func (ledgerRecord) TableName() string { return "ledger_entries" }

type payoutRecord struct {
	ID            string `gorm:"primaryKey;size:40"`
	Number        int64  `gorm:"uniqueIndex"`
	MerchantID    string `gorm:"size:40;index;not null"`
	Livemode      bool   `gorm:"index"`
	Amount        int64
	Fee           int64
	Status        string `gorm:"size:20;index"`
	BankName      string `gorm:"size:80"`
	Holder        string `gorm:"size:200"`
	NIB           string `gorm:"column:nib;size:21"`
	BankChangedAt *time.Time
	RequestedBy   string `gorm:"size:40"`
	RequestedAt   time.Time
	DueAt         time.Time
	ReviewedAt    *time.Time
	SentAt        *time.Time
	PaidAt        *time.Time
	ClosedAt      *time.Time
	FailureReason string `gorm:"size:300"`
	ReviewedBy    string `gorm:"size:40"`
	ReceiptPath   string `gorm:"size:500"`
	ReceiptName   string `gorm:"size:200"`
	ReceiptType   string `gorm:"size:60"`
	ReceiptSize   int64
	ReceiptHash   string `gorm:"size:64"`
	EmailedAt     *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (payoutRecord) TableName() string { return "payouts" }

type endpointRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	URL        string     `gorm:"size:1000"`
	Events     StringList `gorm:"type:text"`
	Active     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (endpointRecord) TableName() string { return "webhook_endpoints" }

type domainRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;uniqueIndex:idx_domain;not null"`
	Domain     string `gorm:"size:200;uniqueIndex:idx_domain;not null"`
	Token      string `gorm:"size:80"`
	VerifiedAt *time.Time
	CreatedAt  time.Time
}

func (domainRecord) TableName() string { return "allowed_domains" }

type eventRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index;not null"`
	Livemode   bool
	Type       string `gorm:"size:60;index"`
	ObjectID   string `gorm:"size:60;index"`
	Data       string `gorm:"type:text"`
	CreatedAt  time.Time
}

func (eventRecord) TableName() string { return "events" }

type deliveryRecord struct {
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

func (deliveryRecord) TableName() string { return "webhook_deliveries" }

type requestLogRecord struct {
	ID         string `gorm:"primaryKey;size:40"`
	MerchantID string `gorm:"size:40;index"`
	Livemode   bool
	Method     string `gorm:"size:10"`
	Path       string `gorm:"size:300"`
	Status     int
	APIKeyID   string `gorm:"column:api_key_id;size:40"`
	IP         string `gorm:"size:64"`
	DurationMs int64
	RequestID  string    `gorm:"size:40"`
	CreatedAt  time.Time `gorm:"index"`
}

func (requestLogRecord) TableName() string { return "api_request_logs" }

type idempotencyRecord struct {
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

func (idempotencyRecord) TableName() string { return "idempotency_records" }

type auditRecord struct {
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

func (auditRecord) TableName() string { return "audit_logs" }

type settingRecord struct {
	Key       string `gorm:"primaryKey;size:80"`
	Value     string `gorm:"size:500"`
	UpdatedAt time.Time
}

func (settingRecord) TableName() string { return "platform_settings" }

type emailRecord struct {
	ID            string `gorm:"primaryKey;size:40"`
	Kind          string `gorm:"size:60;index"`
	To            string `gorm:"column:recipient;size:320"`
	Subject       string `gorm:"size:500"`
	Text          string
	HTML          string `gorm:"column:html"`
	Status        string `gorm:"size:20;index"`
	Attempts      int
	NextAttemptAt *time.Time `gorm:"index"`
	LastError     string     `gorm:"size:500"`
	SentAt        *time.Time
	CreatedAt     time.Time `gorm:"index"`
	UpdatedAt     time.Time
}

func (emailRecord) TableName() string { return "email_outbox" }

type passwordResetRecord struct {
	TokenHash string    `gorm:"primaryKey;size:64"`
	UserID    string    `gorm:"size:40;index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (passwordResetRecord) TableName() string { return "password_resets" }

type loginCodeRecord struct {
	UserID    string `gorm:"primaryKey;size:40"`
	CodeHash  string `gorm:"size:64;not null"`
	Attempts  int
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (loginCodeRecord) TableName() string { return "login_codes" }

type trustedDeviceRecord struct {
	TokenHash string    `gorm:"primaryKey;size:64"`
	UserID    string    `gorm:"size:40;index;not null"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

func (trustedDeviceRecord) TableName() string { return "trusted_devices" }

func allRecords() []any {
	return append([]any{
		&userRecord{}, &sessionRecord{}, &merchantRecord{}, &documentRecord{}, &bankRecord{}, &keyRecord{},
		&linkRecord{}, &sessionCheckoutRecord{}, &paymentRecord{}, &refundRecord{}, &ledgerRecord{}, &payoutRecord{},
		&endpointRecord{}, &domainRecord{}, &eventRecord{}, &deliveryRecord{}, &requestLogRecord{}, &idempotencyRecord{},
		&auditRecord{}, &settingRecord{}, &emailRecord{}, &passwordResetRecord{}, &trailRecord{},
		&loginCodeRecord{}, &trustedDeviceRecord{},
	}, append(append(subscriptionRecords(), integrationRecords()...), rollupRecords()...)...)
}
