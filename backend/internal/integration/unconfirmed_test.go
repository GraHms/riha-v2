package integration

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// lateRail stands in for the operator: a payment stays pending until a test
// gives it an answer.
type lateRail struct {
	mu    sync.Mutex
	out   domain.Outcome
	known bool
	err   error
}

func (r *lateRail) Initiate(context.Context, domain.ChargeRequest, func(domain.Outcome)) (domain.ChargeResult, error) {
	return domain.ChargeResult{Async: true}, nil
}

func (r *lateRail) Query(context.Context, domain.StatusQuery) (domain.Outcome, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.out, r.known, r.err
}

func (r *lateRail) say(out domain.Outcome, known bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.out, r.known, r.err = out, known, err
}

// texts records the SMS RIHA sends.
type texts struct {
	mu   sync.Mutex
	sent []ports.OutboundMessage
}

func (x *texts) Send(_ context.Context, m ports.OutboundMessage) (ports.SentMessage, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	x.sent = append(x.sent, m)
	return ports.SentMessage{Channel: m.Channel}, nil
}

// all waits for what is queued to go out: RIHA sends in the background.
func (x *texts) all(a *app.App, ctx context.Context) []ports.OutboundMessage {
	for i := 0; i < 40; i++ {
		a.SendQueued(ctx)
		if q, _ := a.Messages.Queued(ctx, time.Now().Add(time.Hour), 1); len(q) == 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	x.mu.Lock()
	defer x.mu.Unlock()
	return append([]ports.OutboundMessage(nil), x.sent...)
}

type unconfirmedKit struct {
	a    *app.App
	ctx  context.Context
	rail *lateRail
	sms  *texts
	m    *domain.Merchant
	real func() time.Time
	t    *testing.T
}

func newUnconfirmedKit(t *testing.T) *unconfirmedKit {
	a, ctx := setup(t)
	k := &unconfirmedKit{a: a, ctx: ctx, rail: &lateRail{err: ports.ErrStillPending}, sms: &texts{}, real: a.Now, t: t}
	a.Rail, a.Messenger = k.rail, k.sms
	k.m = approved(t, a, ctx, "Corridas Lda", "corridas@example.com")
	return k
}

func (k *unconfirmedKit) at(d time.Duration) { k.a.Now = func() time.Time { return k.real().Add(d) } }

func (k *unconfirmedKit) session() *domain.CheckoutSession {
	cs, err := k.a.CreateSession(k.ctx, k.m, true, app.SessionParams{Amount: 1000, Description: "Corrida Azul", Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		k.t.Fatal(err)
	}
	return cs
}

func (k *unconfirmedKit) pay(cs *domain.CheckoutSession, again bool) *domain.Payment {
	p, err := k.a.Pay(k.ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861233118", PayAgain: again})
	if err != nil {
		k.t.Fatal(err)
	}
	return p
}

func (k *unconfirmedKit) get(id string) *domain.Payment {
	p, err := k.a.Payments.ByID(k.ctx, id)
	if err != nil {
		k.t.Fatal(err)
	}
	return p
}

// leaveUnconfirmed moves time past the point where the sweep stops waiting.
func (k *unconfirmedKit) leaveUnconfirmed(p *domain.Payment) {
	k.at(11 * time.Minute)
	k.a.Sweep(k.ctx)
	if got := k.get(p.ID); got.Status != domain.PaymentUnconfirmed || got.RecheckAt == nil {
		k.t.Fatalf("no final answer in 10 minutes must leave it unconfirmed, not failed: %s %v", got.Status, got.RecheckAt)
	}
}

// The case that started this: no answer in time, the customer paid anyway.
// It is credited on its own when the answer comes, and the customer told.
func TestUnconfirmedIsPaidLate(t *testing.T) {
	k := newUnconfirmedKit(t)
	cs := k.session()
	p := k.pay(cs, false)
	k.leaveUnconfirmed(p)

	_, err := k.a.Pay(k.ctx, cs.ID, app.PayInput{Method: "emola", Phone: "861233118"})
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != "payment_unconfirmed" {
		t.Fatalf("paying again while one is unconfirmed must warn first, got %v", err)
	}

	k.a.RecheckUnconfirmed(k.ctx)
	if got := k.get(p.ID); got.Status != domain.PaymentUnconfirmed || got.Rechecks != 0 {
		t.Fatalf("it isn't due yet: %s %d", got.Status, got.Rechecks)
	}
	k.at(14 * time.Minute)
	k.a.RecheckUnconfirmed(k.ctx)
	if got := k.get(p.ID); got.Rechecks != 1 || !got.RecheckAt.After(k.a.Now()) {
		t.Fatalf("still pending: asked again later, got %d %v", got.Rechecks, got.RecheckAt)
	}

	k.rail.say(domain.Outcome{Success: true, OperatorReference: "BIM1"}, true, nil)
	k.at(3 * time.Hour)
	k.a.RecheckUnconfirmed(k.ctx)
	got := k.get(p.ID)
	if got.Status != domain.PaymentSucceeded || !got.PaidLate || got.DuplicateOf != "" || got.ContactPhone != "" || got.RecheckAt != nil {
		t.Fatalf("paid late %+v", got)
	}
	if s, _ := k.a.Sessions.ByID(k.ctx, cs.ID); s.Status != domain.SessionComplete || *s.PaymentID != p.ID {
		t.Fatalf("session %+v", s)
	}
	if b := k.a.Balance(k.ctx, k.m.ID, true); b != got.Net {
		t.Fatalf("the merchant must be credited: %d", b)
	}
	sent := k.sms.all(k.a, k.ctx)
	if len(sent) != 1 || sent[0].To != "861233118" || !strings.Contains(sent[0].Body, "Não precisa de pagar de novo") {
		t.Fatalf("the customer must be told: %+v", sent)
	}
}

// They paid again in the same session, then the first one turns out paid too.
func TestUnconfirmedPaidTwiceInOneSession(t *testing.T) {
	k := newUnconfirmedKit(t)
	cs := k.session()
	first := k.pay(cs, false)
	k.leaveUnconfirmed(first)
	second := k.pay(cs, true)
	if err := k.a.CompletePayment(k.ctx, second.ID, domain.Outcome{Success: true, OperatorReference: "BIM2"}); err != nil {
		t.Fatal(err)
	}
	k.rail.say(domain.Outcome{Success: true, OperatorReference: "BIM1"}, true, nil)
	k.at(time.Hour)
	k.a.RecheckUnconfirmed(k.ctx)
	got := k.get(first.ID)
	if got.Status != domain.PaymentSucceeded || got.DuplicateOf != second.ID {
		t.Fatalf("the late one must be flagged as a duplicate of the second: %+v", got)
	}
	if s, _ := k.a.Sessions.ByID(k.ctx, cs.ID); *s.PaymentID != second.ID {
		t.Fatalf("the session stays with the payment that completed it, got %s", *s.PaymentID)
	}
	sent := k.sms.all(k.a, k.ctx)
	if len(sent) != 1 || !strings.Contains(sent[0].Body, "duas vezes") {
		t.Fatalf("the customer must hear they paid twice: %+v", sent)
	}
}

// What happened with the e-Mola payment: the customer gave up on the first
// session and paid in a new one the next day.
func TestUnconfirmedPaidAgainInANewSession(t *testing.T) {
	k := newUnconfirmedKit(t)
	first := k.pay(k.session(), false)
	k.leaveUnconfirmed(first)
	k.at(13 * time.Hour)
	second := k.pay(k.session(), false)
	if err := k.a.CompletePayment(k.ctx, second.ID, domain.Outcome{Success: true}); err != nil {
		t.Fatal(err)
	}
	k.rail.say(domain.Outcome{Success: true}, true, nil)
	k.at(14 * time.Hour)
	k.a.RecheckUnconfirmed(k.ctx)
	if got := k.get(first.ID); got.Status != domain.PaymentSucceeded || got.DuplicateOf != second.ID {
		t.Fatalf("same phone, merchant and amount: a possible duplicate, got %+v", got)
	}
}

// The operator turns it down late: it fails, and the customer hears nothing was taken.
func TestUnconfirmedIsRejectedLate(t *testing.T) {
	k := newUnconfirmedKit(t)
	p := k.pay(k.session(), false)
	k.leaveUnconfirmed(p)
	k.rail.say(domain.Outcome{FailureCode: domain.FailRejected}, true, nil)
	k.at(time.Hour)
	k.a.RecheckUnconfirmed(k.ctx)
	if got := k.get(p.ID); got.Status != domain.PaymentFailed || got.FailureCode != domain.FailRejected || got.ContactPhone != "" {
		t.Fatalf("rejected late %+v", got)
	}
	sent := k.sms.all(k.a, k.ctx)
	if len(sent) != 1 || !strings.Contains(sent[0].Body, "nada foi cobrado") {
		t.Fatalf("sms %+v", sent)
	}
}

// Never answered: after a day it fails, quietly, since the customer most
// likely just never approved it.
func TestUnconfirmedEndsAfterADay(t *testing.T) {
	k := newUnconfirmedKit(t)
	p := k.pay(k.session(), false)
	k.leaveUnconfirmed(p)
	k.at(23 * time.Hour)
	k.a.RecheckUnconfirmed(k.ctx)
	if got := k.get(p.ID); got.Status != domain.PaymentUnconfirmed {
		t.Fatalf("too soon: %s", got.Status)
	}
	k.at(25 * time.Hour)
	k.a.RecheckUnconfirmed(k.ctx)
	if got := k.get(p.ID); got.Status != domain.PaymentFailed || got.FailureCode != domain.FailNotConfirmed || got.ContactPhone != "" {
		t.Fatalf("after a day %+v", got)
	}
	if sent := k.sms.all(k.a, k.ctx); len(sent) != 0 {
		t.Fatalf("no SMS for a payment nobody answered: %+v", sent)
	}
}

// From the console: ask the operator now, see what it would change, apply it.
// A payment closed as timed out before unconfirmed existed gets credited.
func TestOperatorCheckCreditsAClosedPayment(t *testing.T) {
	k := newUnconfirmedKit(t)
	p := k.pay(k.session(), false)
	if err := k.a.CompletePayment(k.ctx, p.ID, domain.Outcome{FailureCode: domain.FailCustomerTimeout}); err != nil {
		t.Fatal(err)
	}
	k.rail.say(domain.Outcome{Success: true, OperatorReference: "BIM9"}, true, nil)
	look, err := k.a.CheckWithOperator(k.ctx, p.ID, "admin", false)
	if err != nil || look.Answer != "paid" || look.Fix != "credit" || look.Applied {
		t.Fatalf("look %+v %v", look, err)
	}
	if got := k.get(p.ID); got.Status != domain.PaymentFailed {
		t.Fatalf("looking must change nothing: %s", got.Status)
	}
	done, err := k.a.CheckWithOperator(k.ctx, p.ID, "admin", true)
	if err != nil || !done.Applied || done.Payment.Status != domain.PaymentSucceeded || !done.Payment.PaidLate || done.Payment.OperatorReference != "BIM9" {
		t.Fatalf("apply %+v %v", done, err)
	}
	if b := k.a.Balance(k.ctx, k.m.ID, true); b != done.Payment.Net {
		t.Fatalf("balance %d", b)
	}

	// One the operator turned down isn't reopened, whatever it says now.
	q := k.pay(k.session(), false)
	if err := k.a.CompletePayment(k.ctx, q.ID, domain.Outcome{FailureCode: domain.FailRejected}); err != nil {
		t.Fatal(err)
	}
	if look, _ := k.a.CheckWithOperator(k.ctx, q.ID, "admin", true); look.Fix != "" || look.Applied {
		t.Fatalf("a rejected payment must stay as it is: %+v", look)
	}
}
