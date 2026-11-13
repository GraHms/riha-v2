package integration

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"riha/internal/adapters/rails"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// scripted answers each request with the next outcome in its list, and
// remembers which integration each went through.
type scripted struct {
	mu   sync.Mutex
	via  string
	outs *[]domain.Outcome
	seen *[]string
}

func (s *scripted) Initiate(_ context.Context, req domain.ChargeRequest, onComplete func(domain.Outcome)) (domain.ChargeResult, error) {
	s.mu.Lock()
	o := (*s.outs)[0]
	if len(*s.outs) > 1 {
		*s.outs = (*s.outs)[1:]
	}
	*s.seen = append(*s.seen, s.via+" "+domain.NormalizeMZPhone(req.Phone))
	s.mu.Unlock()
	onComplete(o)
	return domain.ChargeResult{Async: true, Last4: req.Phone[len(req.Phone)-4:]}, nil
}

func retryApp(t *testing.T, outs ...domain.Outcome) (*app.App, context.Context, *[]string) {
	a, ctx := setup(t)
	seen := &[]string{}
	list := append([]domain.Outcome{}, outs...)
	direct := scripted{via: domain.IntegrationMpesaDirect, outs: &list, seen: seen}
	mtoca := scripted{via: domain.IntegrationMtoca, outs: &list, seen: seen}
	mock := rails.NewMock(0)
	mock.After = func(_ time.Duration, f func()) { f() }
	a.Rail = rails.Modes{
		Test: rails.Single(domain.IntegrationSimulator, mock),
		Live: rails.Router{Via: map[string]rails.Via{
			domain.IntegrationMtoca:       {Rail: &mtoca, Methods: []string{"mpesa", "emola"}},
			domain.IntegrationMpesaDirect: {Rail: &direct, Methods: []string{"mpesa"}},
		}, Default: map[string]string{"mpesa": domain.IntegrationMpesaDirect, "emola": domain.IntegrationMtoca}},
	}
	if err := a.SetupIntegrations(ctx); err != nil {
		t.Fatal(err)
	}
	return a, ctx, seen
}

func sessionPayments(t *testing.T, a *app.App, ctx context.Context, sessionID string) []domain.Payment {
	t.Helper()
	ps, err := a.Payments.Find(ctx, ports.PaymentQuery{SessionID: sessionID})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].Retry < ps[j].Retry })
	return ps
}

var mpesaGaveUp = domain.Outcome{FailureCode: domain.FailOperatorTimeout}

// When M-Pesa fails the transaction, RIHA sends a new request by itself, three
// in all through M-Pesa direct, then one through mtoca, and the customer only
// ever started one.
func TestMpesaFailingIsTriedAgainThenThroughMtoca(t *testing.T) {
	a, ctx, seen := retryApp(t, mpesaGaveUp)
	m := approved(t, a, ctx, "Loja 910", "r910@example.com")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 1000, PaymentMethodTypes: []string{"mpesa"}, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "84 123 4925"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"mpesa_direct 841234925", "mpesa_direct 841234925", "mpesa_direct 841234925", "mtoca 841234925"}
	if len(*seen) != len(want) {
		t.Fatalf("requests %v, want %v", *seen, want)
	}
	for i := range want {
		if (*seen)[i] != want[i] {
			t.Fatalf("requests %v, want %v", *seen, want)
		}
	}
	ps := sessionPayments(t, a, ctx, cs.ID)
	if len(ps) != 4 {
		t.Fatalf("%d payments in the session, want 4", len(ps))
	}
	for i, p := range ps {
		if p.Retry != i || p.Status != domain.PaymentFailed || p.FailureCode != domain.FailOperatorTimeout || p.ContactPhone != "" {
			t.Fatalf("try %d: %+v", i+1, p)
		}
		if a.TriesFor(&p) != 4 {
			t.Fatalf("try %d of %d, want 4", i+1, a.TriesFor(&p))
		}
	}
	if ps[0].Integration != domain.IntegrationMpesaDirect || ps[3].Integration != domain.IntegrationMtoca || !ps[3].Priced || ps[3].RailReference == ps[0].RailReference {
		t.Fatalf("last try %+v", ps[3])
	}
	// RIHA's tries don't use up the customer's.
	if n, _ := a.Payments.CountCustomerFailures(ctx, cs.ID); n != 1 {
		t.Fatalf("%d customer failures, want 1", n)
	}
	noDrift(t, a, ctx)
}

// The second try is paid: the session completes once, the merchant is credited
// once, and nothing more is sent.
func TestMpesaRetryThatPaysEndsTheTries(t *testing.T) {
	a, ctx, seen := retryApp(t, mpesaGaveUp, domain.Outcome{Success: true, OperatorReference: "MP123"})
	m := approved(t, a, ctx, "Loja 911", "r911@example.com")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 1000, PaymentMethodTypes: []string{"mpesa"}, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234925"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*seen) != 2 {
		t.Fatalf("requests %v", *seen)
	}
	if first.Status != domain.PaymentFailed || first.FailureCode != domain.FailOperatorTimeout {
		t.Fatalf("first try %+v", first)
	}
	last := a.LatestPayment(ctx, cs.ID)
	if last.ID == first.ID || last.Retry != 1 || last.Status != domain.PaymentSucceeded || last.DuplicateOf != "" {
		t.Fatalf("second try %+v", last)
	}
	got, err := a.Sessions.ByID(ctx, cs.ID)
	if err != nil || got.Status != domain.SessionComplete || *got.PaymentID != last.ID {
		t.Fatalf("session %+v %v", got, err)
	}
	if b := a.Balance(ctx, m.ID, true); b != last.Net {
		t.Fatalf("balance %d, want %d", b, last.Net)
	}
	noDrift(t, a, ctx)
}

// Only M-Pesa giving up is tried again: a refusal the customer can act on ends
// the payment as before.
func TestMpesaRefusalIsNotTriedAgain(t *testing.T) {
	a, ctx, seen := retryApp(t, domain.Outcome{FailureCode: domain.FailInsufficientFunds})
	m := approved(t, a, ctx, "Loja 912", "r912@example.com")
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 1000, PaymentMethodTypes: []string{"mpesa"}, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234925"})
	if err != nil {
		t.Fatal(err)
	}
	if len(*seen) != 1 || p.FailureCode != domain.FailInsufficientFunds || a.LatestPayment(ctx, cs.ID).ID != p.ID {
		t.Fatalf("requests %v, payment %+v", *seen, p)
	}
}

// Test mode does the same with the simulator's 5555 number, with no other
// integration to fall back to.
func TestMpesaRetryInTestMode(t *testing.T) {
	a, ctx, _ := retryApp(t, mpesaGaveUp)
	m := approved(t, a, ctx, "Loja 913", "r913@example.com")
	cs, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: 1000, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841235555"}); err != nil {
		t.Fatal(err)
	}
	last := a.LatestPayment(ctx, cs.ID)
	if last.Retry != 2 || last.Status != domain.PaymentFailed || last.FailureCode != domain.FailOperatorTimeout || a.TriesFor(last) != 3 {
		t.Fatalf("last try %+v, of %d", last, a.TriesFor(last))
	}
}
