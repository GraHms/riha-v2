package ports

import (
	"context"
	"time"
)

// Insights answers the admin's questions about payments: find one, and how it is going.
type Insights interface {
	FindThreads(ctx context.Context, q string, limit int) ([]ThreadHit, error)
	Metrics(ctx context.Context, q MetricsQuery) (Metrics, error)
	Health(ctx context.Context, since time.Time) (Health, error)
}

type ThreadHit struct {
	PaymentID   string    `json:"payment_id"`
	SessionID   string    `json:"session_id"`
	Reference   string    `json:"reference"`
	Operation   string    `json:"operation"`
	Method      string    `json:"method"`
	Status      string    `json:"status"`
	Amount      int64     `json:"amount"`
	Livemode    bool      `json:"livemode"`
	CreatedAt   time.Time `json:"created_at"`
	Merchant    string    `json:"merchant"`
	PhonePrefix string    `json:"phone_prefix"`
	PhoneLast4  string    `json:"phone_last4"`
	FailureCode string    `json:"failure_code"`
}

type MetricsQuery struct {
	From, To   time.Time
	Live       bool
	MerchantID string
}

type Metrics struct {
	Daily []struct {
		Day    string `json:"day"`
		Method string `json:"method"`
		Total  int64  `json:"total"`
		Paid   int64  `json:"paid"`
		Failed int64  `json:"failed"`
		Amount int64  `json:"amount"`
	} `json:"daily"`
	Failures []struct {
		Code   string `json:"code"`
		Method string `json:"method"`
		N      int64  `json:"n"`
	} `json:"failures"`
	Methods []struct {
		Method     string  `json:"method"`
		Paid       int64   `json:"paid"`
		Failed     int64   `json:"failed"`
		P50Seconds float64 `json:"p50_seconds"`
		P90Seconds float64 `json:"p90_seconds"`
	} `json:"methods"`
	Operator []struct {
		Method string  `json:"method"`
		N      int64   `json:"n"`
		P50Ms  float64 `json:"p50_ms"`
		P90Ms  float64 `json:"p90_ms"`
		Errors int64   `json:"errors"`
	} `json:"operator"`
	Networks []struct {
		Network string `json:"network"`
		Total   int64  `json:"total"`
		Paid    int64  `json:"paid"`
		Failed  int64  `json:"failed"`
	} `json:"networks"`
	Commission []struct {
		Wallet     string  `json:"wallet"`
		Payments   int64   `json:"payments"`
		Commission float64 `json:"commission"`
		Amount     float64 `json:"amount"`
	} `json:"commission"`
	Funnel struct {
		Opened int64 `json:"opened"`
		Tried  int64 `json:"tried"`
		Paid   int64 `json:"paid"`
	} `json:"funnel"`
	Webhooks struct {
		Total     int64   `json:"total"`
		Delivered int64   `json:"delivered"`
		Failed    int64   `json:"failed"`
		Pending   int64   `json:"pending"`
		P90Ms     float64 `json:"p90_ms"`
	} `json:"webhooks"`
	API struct {
		Total        int64   `json:"total"`
		ClientErrors int64   `json:"client_errors"`
		ServerErrors int64   `json:"server_errors"`
		P90Ms        float64 `json:"p90_ms"`
	} `json:"api"`
	Stuck int64 `json:"stuck"`
}

// Health is what went wrong lately, for the alerts.
type Health struct {
	StuckPayments   int64
	FailedWebhooks  int64
	OperatorErrors  int64
	APIServerErrors int64
	ErrorLogs       int64
	LastTrail       time.Time
}
