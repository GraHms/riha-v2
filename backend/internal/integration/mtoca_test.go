package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/adapters/rails"
	"riha/internal/adapters/rails/mtoca"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func TestMtocaRecoveredBySweep(t *testing.T) {
	a, ctx := setup(t)
	var mu sync.Mutex
	status := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path == "/api/payments" {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"paymentStatus": status})
	}))
	defer srv.Close()
	mt, err := mtoca.New(mtoca.Config{BaseURL: srv.URL, Channels: map[string]string{"mpesa": "BIM"}, PollEvery: 10 * time.Millisecond, PollFor: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	a.Rail = rails.Router{ByMethod: map[string]ports.Rail{"mpesa": mt}}
	m := approved(t, a, ctx, "Loja Mtoca", "loja@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 150000, Description: "Encomenda 77", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "848255237"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentProcessing || len(p.RailReference) != 12 || p.PayerPhone != "848255237" {
		t.Fatalf("payment %+v", p)
	}
	if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861234567"}); err == nil {
		t.Fatal("methods without a rail must be refused")
	}
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	status = "P"
	mu.Unlock()
	real := a.Now
	a.Now = func() time.Time { return real().Add(4 * time.Minute) }
	a.Sweep(ctx)
	got, _ := a.Payments.ByID(ctx, p.ID)
	if got.Status != domain.PaymentSucceeded || got.OperatorReference != p.RailReference {
		t.Fatalf("sweep must ask mtoca and credit the payment, got %s %q", got.Status, got.OperatorReference)
	}
	if a.Balance(ctx, m.ID, true) != 150000-9000 {
		t.Fatalf("balance %d", a.Balance(ctx, m.ID, true))
	}
	if got.PayerPhone != "" {
		t.Fatal("the whole number must go once the payment has ended")
	}
}

// mtoca knows a pending payment, so the customer can still confirm it: the sweep waits
// the full 30 minutes, and not the 5 it gives a payment the operator has no record of.
func TestMtocaPendingIsNotClosedEarly(t *testing.T) {
	a, ctx := setup(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()
	mt, err := mtoca.New(mtoca.Config{BaseURL: srv.URL, Channels: map[string]string{"emola": "EMOLA"}, PollEvery: time.Hour, PollFor: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	a.Rail = rails.Router{ByMethod: map[string]ports.Rail{"emola": mt}}
	m := approved(t, a, ctx, "Loja Espera", "espera@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Espera", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861234567"})
	if err != nil {
		t.Fatal(err)
	}
	real := a.Now
	a.Now = func() time.Time { return real().Add(10 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentProcessing {
		t.Fatalf("pending at mtoca after 10 minutes must stay open, got %s", got.Status)
	}
	a.Now = func() time.Time { return real().Add(31 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed {
		t.Fatalf("after 30 minutes it must be closed, got %s", got.Status)
	}
}

// The direct M-Pesa rail never needs the whole number, so it isn't stored.
func TestDirectMpesaKeepsNoWholeNumber(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja Direta", "direta@mtoca.test")
	p := pay(t, a, ctx, m, 5000, "841234567")
	if p.PayerPhone != "" || !strings.HasPrefix(p.RailReference, "RIHA") {
		t.Fatalf("phone %q reference %q", p.PayerPhone, p.RailReference)
	}
}
