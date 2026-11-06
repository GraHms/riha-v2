package domain

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The trail is what happened to a checkout and its payments, in order, in one
// place: what the customer did and on what network, what RIHA asked the operator
// and what it answered, the operator's callback, the webhooks and the API calls.
// It is for finding out what happened, so it keeps what was said, with secrets
// taken out and phone numbers masked.

const (
	TrailCheckoutCreated = "checkout.created"
	TrailClient          = "client"
	TrailSubmitted       = "payment.submitted"
	TrailRailRequest     = "rail.request"
	TrailRailResponse    = "rail.response"
	TrailRailStatus      = "rail.status"
	TrailRailCallback    = "rail.callback"
	TrailPaymentStatus   = "payment.status"
	TrailWebhook         = "webhook.delivery"
	TrailAPI             = "api.request"
	TrailLog             = "log"

	TrailInfo  = "info"
	TrailWarn  = "warn"
	TrailError = "error"
)

type TrailEvent struct {
	ID         string
	MerchantID string
	Livemode   bool
	SessionID  string
	PaymentID  string
	RequestID  string
	Kind       string
	// Source says who acted: customer, merchant, api, rail, operator or riha.
	Source     string
	Level      string
	Title      string
	Method     string
	Path       string
	Status     int
	DurationMs int64
	IP         string
	UserAgent  string
	Network    string
	Request    string
	Response   string
	Data       map[string]any
	CreatedAt  time.Time
}

// RequestInfo is who made the request being served, carried in its context so
// what happens because of it can say so.
type RequestInfo struct {
	ID        string
	IP        string
	UserAgent string
}

type requestKey struct{}

func WithRequest(ctx context.Context, r RequestInfo) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}

func RequestFrom(ctx context.Context) RequestInfo {
	if ctx == nil {
		return RequestInfo{}
	}
	r, _ := ctx.Value(requestKey{}).(RequestInfo)
	return r
}

// TrailBodyLimit is the most of a body kept.
const TrailBodyLimit = 16 << 10

var (
	secretKey  = regexp.MustCompile(`(?i)(secret|password|passwd|token|authorization|api[_-]?key|access[_-]?key|private|pin|cvc|cvv|card_number|signature)`)
	cardNumber = regexp.MustCompile(`\b\d{13,19}\b`)
	mzPhone    = regexp.MustCompile(`\b(258 ?)?(8[2-7]) ?(\d{3}) ?(\d{4})\b`)
)

// MaskPhones hides the middle of Mozambican phone numbers: 84•••4567.
func MaskPhones(s string) string {
	return mzPhone.ReplaceAllString(s, "$2•••$4")
}

// RedactBody keeps a body fit for the trail: in JSON, values under secret names
// are hidden and numbers that look like cards or phones are masked; anything else
// is masked the same way as text. It is cut at TrailBodyLimit.
func RedactBody(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if json.Unmarshal(raw, &v) == nil {
		v = redactValue("", v)
		if b, err := json.MarshalIndent(v, "", "  "); err == nil {
			return cut(string(b))
		}
	}
	return cut(MaskPhones(cardNumber.ReplaceAllString(string(raw), "[cartão]")))
}

func redactValue(key string, v any) any {
	if key != "" && secretKey.MatchString(key) && !strings.EqualFold(key, "pin_incorrect") {
		if v == nil || v == "" {
			return v
		}
		return "[oculto]"
	}
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = redactValue(k, x)
		}
		return t
	case []any:
		for i, x := range t {
			t[i] = redactValue("", x)
		}
		return t
	case float64:
		// A phone number sent as a JSON number.
		if s := strconv.FormatFloat(t, 'f', -1, 64); mzPhone.MatchString(s) {
			return MaskPhones(s)
		}
		return t
	case string:
		if cardNumber.MatchString(t) && !mzPhone.MatchString(t) {
			return cardNumber.ReplaceAllString(t, "[cartão]")
		}
		return MaskPhones(t)
	}
	return v
}

func cut(s string) string {
	if len(s) <= TrailBodyLimit {
		return s
	}
	return s[:TrailBodyLimit] + "\n… (cortado)"
}
