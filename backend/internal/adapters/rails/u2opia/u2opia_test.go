package u2opia

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// fake stands in for U2opia, as its integration guide describes it.
type fake struct {
	mu        sync.Mutex
	tokens    int
	refreshes int
	reject    int // how many charges to refuse with a 401 first
	charges   []map[string]any
	headers   []http.Header
	answer    map[string]any
	status    int
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var b map[string]any
	_ = json.NewDecoder(r.Body).Decode(&b)
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/api-manager/api/auth/token":
		f.tokens++
		if b["partnerCode"] != "RIH-TEST" {
			w.WriteHeader(403)
			_ = json.NewEncoder(w).Encode(map[string]any{"responseCode": "403", "responseMessage": "Partner is inactive"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"token": "tok-" + b["countryId"].(string), "refreshToken": "ref", "expiresIn": 86400, "refreshTokenExpiresIn": 604800}})
	case "/api-manager/api/auth/refresh":
		f.refreshes++
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"token": "tok-refreshed", "refreshToken": "ref2", "expiresIn": 86400, "refreshTokenExpiresIn": 604800}})
	case "/api-manager/v1/initate-subscription":
		f.charges = append(f.charges, b)
		f.headers = append(f.headers, r.Header.Clone())
		if f.reject > 0 {
			f.reject--
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"responseCode": "401", "responseMessage": "Authentication processing failed"})
			return
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
		}
		_ = json.NewEncoder(w).Encode(f.answer)
	default:
		w.WriteHeader(404)
	}
}

func newClient(t *testing.T, f *fake, now func() time.Time) *Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	za := Known["ZA"]
	za.PerMZN = 0.28
	ng := Known["NG"]
	ng.PerMZN = 24.3
	c, err := New(Config{BaseURL: srv.URL + "/api-manager", PartnerCode: "RIH-TEST",
		Countries: map[string]Country{"ZA": za, "NG": ng}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ok() map[string]any {
	return map[string]any{"responseCode": "200", "responseMessage": "OK", "transactionId": "1772531713529", "expiresIn": 600,
		"redirectionUrl": "https://wallet-staging.ma.u2opiaglobal.com/payment?query=abc"}
}

func TestInitiateSendsTheCustomerToU2opia(t *testing.T) {
	f := &fake{answer: ok()}
	c := newClient(t, f, nil)
	var railID string
	res, err := c.Initiate(context.Background(), domain.ChargeRequest{PaymentID: "pay_1", Amount: 100000, Phone: "+27 82 123 4567", Country: "za",
		OnRailID: func(id string) { railID = id }}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Async || res.RedirectURL != "https://wallet-staging.ma.u2opiaglobal.com/payment?query=abc" || railID != "1772531713529" {
		t.Fatalf("result %+v, rail id %q", res, railID)
	}
	got := f.charges[0]
	// 1 000 MT at 0.28 rand per metical, the number without +27.
	if got["msisdn"] != "821234567" || got["amount"] != "280.00" || got["chargingType"] != "Wallet" {
		t.Fatalf("charge body %v", got)
	}
	h := f.headers[0]
	if h.Get("Authorization") != "Bearer tok-ZA" || h.Get("x-partner-code") != "RIH-TEST" || h.Get("x-country-id") != "ZA" {
		t.Fatalf("headers %v", h)
	}
	// The token is kept: a second charge doesn't ask for another.
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Amount: 1000, Phone: "0821234567", Country: "ZA"}, nil); err != nil {
		t.Fatal(err)
	}
	if f.tokens != 1 {
		t.Fatalf("asked for %d tokens", f.tokens)
	}
}

func TestATokenIsRefreshedAndARejectedOneReplaced(t *testing.T) {
	f := &fake{answer: ok()}
	now := time.Now()
	c := newClient(t, f, func() time.Time { return now })
	req := domain.ChargeRequest{Amount: 1000, Phone: "8178487547", Country: "NG"}
	if _, err := c.Initiate(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	now = now.Add(25 * time.Hour)
	if _, err := c.Initiate(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	if f.refreshes != 1 || f.headers[1].Get("Authorization") != "Bearer tok-refreshed" {
		t.Fatalf("an expired token must be refreshed: %d %v", f.refreshes, f.headers[1].Get("Authorization"))
	}
	f.reject = 1
	if _, err := c.Initiate(context.Background(), req, nil); err != nil {
		t.Fatalf("a 401 must be retried once with a new token: %v", err)
	}
	if f.tokens != 2 {
		t.Fatalf("tokens asked %d", f.tokens)
	}
}

func TestInitiateRefusals(t *testing.T) {
	f := &fake{answer: map[string]any{"responseCode": "400", "responseMessage": " Invalid country code:LO"}, status: 400}
	c := newClient(t, f, nil)
	var ie *domain.InputError
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Amount: 1000, Phone: "821234567", Country: "MZ"}, nil); !errors.As(err, &ie) || ie.Field != "country" {
		t.Fatalf("a country without a rate must be refused before asking: %v", err)
	}
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Amount: 1000, Phone: "123", Country: "ZA"}, nil); !errors.As(err, &ie) || ie.Field != "phone" {
		t.Fatalf("a short number: %v", err)
	}
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Amount: 1000, Phone: "821234567", Country: "ZA"}, nil); !errors.As(err, &ie) {
		t.Fatalf("U2opia's 400 must reach the customer as a refusal: %v", err)
	}
	f.answer, f.status = map[string]any{"responseCode": "500", "responseMessage": "Internal server error"}, 500
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Amount: 1000, Phone: "821234567", Country: "ZA"}, nil); err == nil || errors.As(err, &ie) {
		t.Fatalf("a 500 is U2opia's problem, not the customer's: %v", err)
	}
	f.answer, f.status = map[string]any{"responseCode": "200", "transactionId": "1", "redirectionUrl": "http://evil.example/pay"}, 0
	if _, err := c.Initiate(context.Background(), domain.ChargeRequest{Amount: 1000, Phone: "821234567", Country: "ZA"}, nil); err == nil {
		t.Fatal("a payment page that isn't https must not be followed")
	}
}

func TestQueryWaitsForTheCallback(t *testing.T) {
	c := newClient(t, &fake{}, nil)
	if _, known, err := c.Query(context.Background(), domain.StatusQuery{}); known || !errors.Is(err, ports.ErrStillPending) {
		t.Fatalf("U2opia can't be asked, so it's pending: %v %v", known, err)
	}
}

func TestNationalNumber(t *testing.T) {
	za, ng := Known["ZA"], Known["NG"]
	for in, want := range map[string]string{"+27 82 123 4567": "821234567", "0027821234567": "821234567", "082 123 4567": "821234567", "821234567": "821234567"} {
		if got := NationalNumber(in, za); got != want {
			t.Fatalf("%q: %q", in, got)
		}
	}
	if got := NationalNumber("+234 817 848 7547", ng); got != "8178487547" {
		t.Fatalf("ng %q", got)
	}
	ng.PerMZN = 24.3
	if got := LocalAmount(100050, ng); got != "24312.15" {
		t.Fatalf("amount %q", got)
	}
}
