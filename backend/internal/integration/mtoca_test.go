package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if p.Status != domain.PaymentProcessing || len(p.RailRef) != 12 || p.PayerPhone != "848255237" {
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
	if got.Status != domain.PaymentSucceeded || got.OperatorReference != p.RailRef {
		t.Fatalf("sweep must ask mtoca and credit the payment, got %s %q", got.Status, got.OperatorReference)
	}
	if a.Balance(ctx, m.ID, true) != 150000-9000 {
		t.Fatalf("balance %d", a.Balance(ctx, m.ID, true))
	}
}
