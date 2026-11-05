package integration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// countRail counts how many times a customer would be asked to pay.
type countRail struct{ charges atomic.Int32 }

func (r *countRail) Initiate(context.Context, domain.ChargeRequest, func(domain.Outcome)) (domain.ChargeResult, error) {
	r.charges.Add(1)
	return domain.ChargeResult{Async: true, Last4: "4567"}, nil
}

func TestSameAttemptNeverChargesTwice(t *testing.T) {
	a, ctx := setup(t)
	rail := &countRail{}
	a.Rail = rail
	m := approved(t, a, ctx, "Loja Tentativa", "tentativa@mtoca.test")
	session := func() *domain.CheckoutSession {
		t.Helper()
		cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 5000, Description: "Tentativa", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
		if err != nil {
			t.Fatal(err)
		}
		return cs
	}
	pay := func(cs *domain.CheckoutSession, attempt string) (*domain.Payment, error) {
		return a.Pay(ctx, cs.ID, app.PayInput{Method: "mpesa", Phone: "841234567", AttemptID: attempt})
	}
	const first = "AAAA-BBBB-CCCC"

	t.Run("the answer was lost and the page asks again", func(t *testing.T) {
		cs := session()
		rail.charges.Store(0)
		p1, err := pay(cs, first)
		if err != nil {
			t.Fatal(err)
		}
		p2, err := pay(cs, first)
		if err != nil {
			t.Fatalf("a repeat must return the payment, not an error: %v", err)
		}
		if p1.ID != p2.ID || rail.charges.Load() != 1 {
			t.Fatalf("payments %s %s, charges %d", p1.ID, p2.ID, rail.charges.Load())
		}
		if p1.AttemptID != first {
			t.Fatalf("attempt %q", p1.AttemptID)
		}
	})

	t.Run("the same request arrives many times at once", func(t *testing.T) {
		cs := session()
		rail.charges.Store(0)
		var wg sync.WaitGroup
		ids := make([]string, 8)
		errs := make([]error, 8)
		for i := range ids {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				p, err := pay(cs, first)
				errs[i] = err
				if err == nil {
					ids[i] = p.ID
				}
			}(i)
		}
		wg.Wait()
		for i := range ids {
			if errs[i] != nil {
				t.Fatalf("request %d: %v", i, errs[i])
			}
			if ids[i] != ids[0] {
				t.Fatalf("two payments: %v", ids)
			}
		}
		if rail.charges.Load() != 1 {
			t.Fatalf("charges %d", rail.charges.Load())
		}
	})

	t.Run("a repeat after it failed doesn't charge, and a new attempt does", func(t *testing.T) {
		cs := session()
		rail.charges.Store(0)
		p1, err := pay(cs, first)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.CompletePayment(ctx, p1.ID, domain.Outcome{FailureCode: domain.FailNotCompleted}); err != nil {
			t.Fatal(err)
		}
		again, err := pay(cs, first)
		if err != nil || again.ID != p1.ID || again.Status != domain.PaymentFailed || rail.charges.Load() != 1 {
			t.Fatalf("repeat %+v %v, charges %d", again, err, rail.charges.Load())
		}
		next, err := pay(cs, "DDDD-EEEE-FFFF")
		if err != nil || next.ID == p1.ID || rail.charges.Load() != 2 {
			t.Fatalf("new attempt %+v %v, charges %d", next, err, rail.charges.Load())
		}
	})

	t.Run("a repeat after it was paid returns the paid payment", func(t *testing.T) {
		cs := session()
		p1, err := pay(cs, first)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.CompletePayment(ctx, p1.ID, domain.Outcome{Success: true, OperatorReference: "NEP1"}); err != nil {
			t.Fatal(err)
		}
		again, err := pay(cs, first)
		if err != nil || again.ID != p1.ID || again.Status != domain.PaymentSucceeded {
			t.Fatalf("repeat %+v %v", again, err)
		}
	})

	t.Run("another attempt while one is open is still refused", func(t *testing.T) {
		cs := session()
		if _, err := pay(cs, first); err != nil {
			t.Fatal(err)
		}
		if _, err := pay(cs, "GGGG-HHHH-JJJJ"); err == nil {
			t.Fatal("a second attempt must wait for the first")
		}
	})

	t.Run("the same code on another session is its own payment", func(t *testing.T) {
		p1, err := pay(session(), first)
		if err != nil {
			t.Fatal(err)
		}
		p2, err := pay(session(), first)
		if err != nil || p1.ID == p2.ID {
			t.Fatalf("%+v %v", p2, err)
		}
	})

	t.Run("an id the page wouldn't make reads as none, and an old page still works", func(t *testing.T) {
		rail.charges.Store(0)
		for _, junk := range []string{"", "lower-case-id", "x", "AAAA'; DROP TABLE payments;--"} {
			p, err := pay(session(), junk)
			if err != nil || p.AttemptID != "" {
				t.Fatalf("%q: %+v %v", junk, p, err)
			}
		}
		if rail.charges.Load() != 4 {
			t.Fatalf("charges %d", rail.charges.Load())
		}
	})

	t.Run("support finds a payment by the code", func(t *testing.T) {
		cs := session()
		p, err := pay(cs, "KKKK-MMMM-NNNN")
		if err != nil {
			t.Fatal(err)
		}
		got, err := a.Payments.BySessionAttempt(ctx, cs.ID, "KKKK-MMMM-NNNN")
		if err != nil || got.ID != p.ID {
			t.Fatalf("%+v %v", got, err)
		}
	})
}
