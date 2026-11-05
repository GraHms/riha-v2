package integration

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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
	unknown  bool
	gone     bool
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
	if f.unknown {
		w.WriteHeader(502)
		return
	}
	if f.gone {
		w.WriteHeader(404)
		return
	}
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

// An error on the customer's phone never reaches the status call, which just stays
// pending. So the sweep lets them retry after 10 minutes, and then keeps asking in case
// they paid late.
func TestMtocaPendingClosesAndIsWatched(t *testing.T) {
	a, ctx := setup(t)
	f := &mtocaFake{status: "N"}
	mtocaApp(t, f, a)
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
	a.Now = func() time.Time { return real().Add(9 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentProcessing {
		t.Fatalf("pending after 9 minutes must stay open, got %s", got.Status)
	}
	a.Now = func() time.Time { return real().Add(11 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed || got.FailureCode != domain.FailCustomerTimeout {
		t.Fatalf("after 10 minutes it must be closed, got %s %s", got.Status, got.FailureCode)
	}
	if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861234567"}); err != nil {
		t.Fatalf("the customer must be able to retry: %v", err)
	}
	closed, err := a.Payments.RecentlyTimedOut(ctx, real().Add(-time.Hour), 10)
	if err != nil || len(closed) != 1 || closed[0].ID != p.ID {
		t.Fatalf("closed %+v %v", closed, err)
	}
	logs := &lockedLog{}
	log.SetOutput(logs)
	defer log.SetOutput(os.Stderr)
	f.mu.Lock()
	f.status = "P"
	f.mu.Unlock()
	// A fresh client: the sweep's check moments ago holds the old one back for 30 seconds.
	mtocaApp(t, f, a)
	a.CheckClosedPayments(ctx)
	if !strings.Contains(logs.text(), "PAID AFTER IT WAS CLOSED") {
		t.Fatalf("logs: %s", logs.text())
	}
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed {
		t.Fatalf("it must not be credited on its own, because the customer may have retried: %s", got.Status)
	}
}

type lockedLog struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// When mtoca lost the answer too, nobody knows, so it closes like a pending one.
func TestMtocaLostAnswerClosesAfter10Minutes(t *testing.T) {
	a, ctx := setup(t)
	mtocaApp(t, &mtocaFake{noAnswer: true, unknown: true}, a)
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
	a.Now = func() time.Time { return real().Add(9 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentProcessing {
		t.Fatalf("too soon: %s", got.Status)
	}
	a.Now = func() time.Time { return real().Add(11 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed {
		t.Fatalf("must close so the customer can retry: %s", got.Status)
	}
}

// A create answer lost by RIHA alone: mtoca still has the payment and it gets credited.
func TestMtocaLostAnswerIsCreditedByOperationNumber(t *testing.T) {
	a, ctx := setup(t)
	mtocaApp(t, &mtocaFake{noAnswer: true, status: "P"}, a)
	m := approved(t, a, ctx, "Loja Salva", "salva@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Salva", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234567"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		if got, _ := a.Payments.ByID(ctx, p.ID); got.Status == domain.PaymentSucceeded {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the payment must be credited")
}

// A payment mtoca has never heard of closes at 5 minutes, because nobody can have paid it.
func TestMtocaNoRecordClosesAfter5Minutes(t *testing.T) {
	a, ctx := setup(t)
	mtocaApp(t, &mtocaFake{noAnswer: true, gone: true}, a)
	m := approved(t, a, ctx, "Loja Nada", "nada@mtoca.test")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Nada", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234567"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	real := a.Now
	a.Now = func() time.Time { return real().Add(6 * time.Minute) }
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed {
		t.Fatalf("got %s", got.Status)
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
