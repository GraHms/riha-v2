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

type mtocaFake struct {
	mu       sync.Mutex
	status   string
	noAnswer bool
	asked    []map[string]any
}

func (f *mtocaFake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var b map[string]any
	_ = json.NewDecoder(r.Body).Decode(&b)
	if r.URL.Path == "/api/payments" {
		if f.noAnswer {
			w.WriteHeader(502)
			return
		}
		w.WriteHeader(201)
		_ = json.NewEncoder(w).Encode(map[string]any{"vendorTransactionId_NEP": "NEP1", "vendorTransactionStatus": "OK"})
		return
	}
	f.asked = append(f.asked, b)
	w.WriteHeader(201)
	_ = json.NewEncoder(w).Encode(map[string]any{"paymentStatus": f.status, "transactionID_NEP": "NEP1"})
}

func mtocaApp(t *testing.T, f *mtocaFake, a *app.App) {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	mt, err := mtoca.New(mtoca.Config{BaseURL: srv.URL, Channels: map[string]string{"mpesa": "MPESA", "emola": "EMOLA"}, PollEvery: 10 * time.Millisecond, PollFor: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	a.Rail = rails.Router{ByMethod: map[string]ports.Rail{"mpesa": mt, "emola": mt}}
}

// A restart loses the polling. The id is in the database, so the sweep can still credit it.
func TestMtocaRecoveredBySweep(t *testing.T) {
	a, ctx := setup(t)
	f := &mtocaFake{}
	mtocaApp(t, f, a)
	m := approved(t, a, ctx, "Loja Mtoca", "loja@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 150000, Description: "Encomenda 77", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "848255237"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentProcessing || len(p.RailReference) != 12 {
		t.Fatalf("payment %+v", p)
	}
	if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861234567"}); err == nil {
		t.Fatal("a second attempt must wait for the first")
	}
	time.Sleep(200 * time.Millisecond)
	stored, _ := a.Payments.ByID(ctx, p.ID)
	if stored.RailID != "NEP1" {
		t.Fatalf("the id must be stored, got %q", stored.RailID)
	}
	f.mu.Lock()
	f.status = "P"
	f.mu.Unlock()
	real := a.Now
	a.Now = func() time.Time { return real().Add(4 * time.Minute) }
	a.Sweep(ctx)
	got, _ := a.Payments.ByID(ctx, p.ID)
	if got.Status != domain.PaymentSucceeded || got.OperatorReference != "NEP1" {
		t.Fatalf("sweep must ask mtoca and credit the payment, got %s %q", got.Status, got.OperatorReference)
	}
	if a.Balance(ctx, m.ID, true) != 150000-9000 {
		t.Fatalf("balance %d", a.Balance(ctx, m.ID, true))
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	last := f.asked[len(f.asked)-1]
	if len(last) != 2 || last["transactionID_NEP"] != "NEP1" || last["operationNumber"] != p.RailReference {
		t.Fatalf("status body %v", last)
	}
}

// mtoca knows a pending payment, so the customer can still confirm it: the sweep waits
// the full 30 minutes, and not the 5 it gives a payment the operator has no record of.
func TestMtocaPendingIsNotClosedEarly(t *testing.T) {
	a, ctx := setup(t)
	mtocaApp(t, &mtocaFake{}, a)
	m := approved(t, a, ctx, "Loja Espera", "espera@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Espera", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861234567"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
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

// A lost create answer leaves no id. The customer can't retry until the sweep closes it,
// at 5 minutes, which is after the create call itself has given up.
func TestMtocaLostAnswerClosesWithoutAnId(t *testing.T) {
	a, ctx := setup(t)
	mtocaApp(t, &mtocaFake{noAnswer: true}, a)
	m := approved(t, a, ctx, "Loja Perdida", "perdida@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Perdida", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234567"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	real := a.Now
	a.Now = func() time.Time { return real().Add(4 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentProcessing {
		t.Fatalf("too soon: %s", got.Status)
	}
	a.Now = func() time.Time { return real().Add(6 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed {
		t.Fatalf("must close so the customer can retry: %s", got.Status)
	}
}

// Without mtoca, nothing about a payment changes.
func TestSimulatedPaymentHasOldReference(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja Direta", "direta@mtoca.test")
	p := pay(t, a, ctx, m, 5000, "841234567")
	if p.RailID != "" || !strings.HasPrefix(p.RailReference, "RIHA") {
		t.Fatalf("id %q reference %q", p.RailID, p.RailReference)
	}
}
