package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

// qaMerchant is a merchant with live money through both integrations: 940 MT
// held by 92 Interactions (M-Pesa direct) and 1 880 MT by mtoca (e-Mola).
func qaMerchant(t *testing.T, a *app.App, ctx context.Context, n int, nib string) *domain.Merchant {
	t.Helper()
	m := approved(t, a, ctx, fmt.Sprintf("QA Loja %d", n), fmt.Sprintf("qa%d@example.com", n))
	payLive(t, a, ctx, m, 100000, "mpesa", "841234231")
	payLive(t, a, ctx, m, 200000, "emola", "861234231")
	bank := "BCI"
	if strings.HasPrefix(nib, domain.BimBankCode) {
		bank = "Millennium bim"
	}
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: bank, Holder: m.Name, NIB: nib}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	return m
}

const qaOtherBank = "000800004821130101135" // BCI: carries the transfer fee
const qaBim = "000100004821130101135"       // bim: no fee

func qaPaidSend(a *app.App, ctx context.Context, payoutID string) int64 {
	var sent int64
	for _, f := range a.PayoutFunding(ctx, []string{payoutID})[payoutID] {
		if f.Status == domain.FundingPaid {
			sent += f.Send
		}
	}
	return sent
}

// A withdrawal split between mtoca and 92 Interactions: mtoca pays its part,
// then the admin marks the withdrawal failed (say 92 Interactions' transfer
// bounced). The merchant must not end up with both mtoca's transfer and the
// whole amount back in its balance.
func TestQAFailingAWithdrawalAfterAPartnerPaidAPartPaysTwice(t *testing.T) {
	a, ctx := settlementApp(t)
	m := qaMerchant(t, a, ctx, 1, qaOtherBank)
	before := a.Balance(ctx, m.ID, true)
	po, err := a.RequestPayout(ctx, m.ID, true, 200000, "u")
	if err != nil {
		t.Fatal(err)
	}
	parts := a.PayoutFunding(ctx, []string{po.ID})[po.ID]
	if len(parts) != 2 {
		t.Fatalf("parts %+v", parts)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActApprove, "", "admin"); err != nil {
		t.Fatal(err)
	}
	var mt domain.PayoutFunding
	for _, f := range parts {
		if f.Source == domain.IntegrationMtoca {
			mt = f
		}
	}
	if _, err := a.PayFunding(ctx, mt.ID, domain.PartnerMtoca, "TRF-MT", receipt(), "usr_mtoca"); err != nil {
		t.Fatal(err)
	}
	_, failErr := a.TransitionPayout(ctx, po.ID, domain.ActFail, "transferência devolvida", "admin")
	after := a.Balance(ctx, m.ID, true)
	sent := qaPaidSend(a, ctx, po.ID)
	rep, _ := a.Settlement(ctx, nil, nil)
	t.Logf("fail err=%v; balance before %d, after %d; already sent to the merchant's bank by mtoca %d; RIHA own %d", failErr, before, after, sent, rep.RIHAOwn)
	if failErr == nil && after+sent > before {
		t.Fatalf("merchant holds %d (balance %d + %d already transferred by mtoca) out of %d: %d paid twice",
			after+sent, after, sent, before, after+sent-before)
	}
}

// Racing the same thing: a partner's part is marked paid while the admin is
// failing the withdrawal. The partner's paid part must not be overwritten.
type qaGateIntegrations struct {
	ports.Integrations
	mu    sync.Mutex
	armed bool
	read  chan struct{}
	gate  chan struct{}
}

func (g *qaGateIntegrations) FundingOf(ctx context.Context, ids []string) ([]domain.PayoutFunding, error) {
	fs, err := g.Integrations.FundingOf(ctx, ids)
	g.mu.Lock()
	hold := g.armed
	g.armed = false
	g.mu.Unlock()
	if hold {
		close(g.read)
		<-g.gate
	}
	return fs, err
}

type qaSlowBody struct {
	open chan struct{}
	r    io.Reader
}

func (b *qaSlowBody) Read(p []byte) (int, error) {
	<-b.open
	return b.r.Read(p)
}

func TestQAPartnerPaymentRacingAFailIsErased(t *testing.T) {
	a, ctx := settlementApp(t)
	m := qaMerchant(t, a, ctx, 2, qaBim)
	g := &qaGateIntegrations{Integrations: a.Integrations, read: make(chan struct{}), gate: make(chan struct{})}
	a.Integrations = g
	po, err := a.RequestPayout(ctx, m.ID, true, 200000, "u")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActApprove, "", "admin"); err != nil {
		t.Fatal(err)
	}
	var mt domain.PayoutFunding
	for _, f := range a.PayoutFunding(ctx, []string{po.ID})[po.ID] {
		if f.Source == domain.IntegrationMtoca {
			mt = f
		}
	}
	pdf := []byte("%PDF-1.4\n% comprovativo\n")
	body := &qaSlowBody{open: make(chan struct{}), r: bytes.NewReader(pdf)}
	partnerDone := make(chan error, 1)
	// The partner submits: the withdrawal is approved, its receipt uploads.
	go func() {
		_, err := a.PayFunding(ctx, mt.ID, domain.PartnerMtoca, "TRF-MT", app.Upload{FileName: "c.pdf", Size: int64(len(pdf)), Head: pdf[:8], Body: body}, "usr_mtoca")
		partnerDone <- err
	}()
	time.Sleep(200 * time.Millisecond) // the partner is past its checks, uploading
	g.mu.Lock()
	g.armed = true
	g.mu.Unlock()
	failDone := make(chan error, 1)
	go func() {
		_, err := a.TransitionPayout(ctx, po.ID, domain.ActFail, "devolvida", "admin")
		failDone <- err
	}()
	<-g.read         // the admin's fail has read the parts as open
	close(body.open) // the partner's upload ends and it records its part paid
	var perr error
	waited := false
	select {
	case perr = <-partnerDone:
	case <-time.After(time.Second): // the partner waits for the withdrawal's lock
		waited = true
	}
	close(g.gate) // the fail goes on
	ferr := <-failDone
	if waited {
		perr = <-partnerDone
	}
	var got domain.PayoutFunding
	for _, f := range a.PayoutFunding(ctx, []string{po.ID})[po.ID] {
		if f.ID == mt.ID {
			got = f
		}
	}
	p, _ := a.Payouts.Lock(ctx, po.ID)
	t.Logf("partner err=%v, fail err=%v, payout %s, mtoca part %s ref=%q receipt=%q, balance %d", perr, ferr, p.Status, got.Status, got.Reference, got.ReceiptPath, a.Balance(ctx, m.ID, true))
	if perr == nil && !got.Sent() {
		t.Fatalf("mtoca was told its payment of %d MT was recorded, but its part is now %q with no reference or receipt, and the merchant's balance was given back in full (%d)",
			got.Send/100, got.Status, a.Balance(ctx, m.ID, true))
	}
}

// RIHA pays a withdrawal itself, with one receipt: the part mtoca was to pay
// is booked as paid by mtoca, so the report says mtoca holds less and RIHA's
// account more than they do.
func TestQARIHAPayingAPartnersPartIsBookedAsThePartners(t *testing.T) {
	a, ctx := settlementApp(t)
	m := qaMerchant(t, a, ctx, 3, qaBim)
	before, _ := a.Settlement(ctx, nil, nil)
	po, err := a.RequestPayout(ctx, m.ID, true, 50000, "u")
	if err != nil {
		t.Fatal(err)
	}
	parts := a.PayoutFunding(ctx, []string{po.ID})[po.ID]
	if len(parts) != 1 || parts[0].Source != domain.IntegrationMtoca {
		t.Fatalf("parts %+v", parts)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActApprove, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CompletePayouts(ctx, []string{po.ID}, receipt(), "admin"); err != nil {
		t.Fatal(err)
	}
	after, _ := a.Settlement(ctx, nil, nil)
	t.Logf("held outside %d -> %d, at RIHA %d -> %d, owed %d -> %d", before.HeldOutside, after.HeldOutside, before.AtRIHA, after.AtRIHA, before.OwedToShops, after.OwedToShops)
	if after.AtRIHA != before.AtRIHA-50000 || after.HeldOutside != before.HeldOutside {
		t.Fatalf("RIHA sent 500 MT from its own account, but the report keeps RIHA's account at %d and lowers what mtoca holds by %d",
			after.AtRIHA, before.HeldOutside-after.HeldOutside)
	}
}

// An answer RIHA read as a refusal (mtoca's "99", or a 4xx on create) closes
// the payment as failed. If the operator later says it was paid, the console's
// operator check, which the logs point to, must be able to credit it.
func TestQAPaidAfterAnOperatorFailureCannotBeCredited(t *testing.T) {
	k := newUnconfirmedKit(t)
	cs := k.session()
	p := k.pay(cs, false)
	if err := k.a.CompletePayment(k.ctx, p.ID, domain.Outcome{FailureCode: domain.FailOperatorUnavailable}); err != nil {
		t.Fatal(err)
	}
	k.rail.say(domain.Outcome{Success: true, OperatorReference: "NEP-LATE"}, true, nil)
	r, err := k.a.CheckWithOperator(k.ctx, p.ID, "admin", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("answer %s fix %q applied %v, status %s, balance %d", r.Answer, r.Fix, r.Applied, k.get(p.ID).Status, k.a.Balance(k.ctx, k.m.ID, true))
	if r.Answer == "paid" && !r.Applied {
		t.Fatalf("the operator says %s was paid, but there is no way to credit it: fix %q", p.ID, r.Fix)
	}
}

// The merchant's webhooks must not say a payment is processing after they said
// it succeeded.
func TestQAProcessingEventAfterSucceeded(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "QA Webhooks", "qa-wh@example.com")
	p := pay(t, a, ctx, m, 5000, "841234567")
	evs, err := a.Events.ByObjects(ctx, m.ID, []string{p.ID})
	if err != nil {
		t.Fatal(err)
	}
	types := []string{}
	for _, e := range evs {
		types = append(types, e.Type)
	}
	t.Logf("events in order: %v", types)
	if len(types) > 0 && types[len(types)-1] == "payment.processing" {
		t.Fatalf("payment.processing was emitted after payment.succeeded: %v", types)
	}
}

// Sound checks: callbacks, polling, sweep and operator check landing at once
// credit a payment once; withdrawals at once never go past the balance.
func TestQAConcurrentCompletionCreditsOnce(t *testing.T) {
	k := newUnconfirmedKit(t)
	p := k.pay(k.session(), false)
	k.rail.say(domain.Outcome{Success: true, OperatorReference: "NEP1"}, true, nil)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 3 {
			case 0:
				_ = k.a.CompletePayment(k.ctx, p.ID, domain.Outcome{Success: true, OperatorReference: "NEP1"})
			case 1:
				_, _ = k.a.CheckWithOperator(k.ctx, p.ID, "admin", true)
			default:
				k.a.RailCallback(k.ctx, p.RailReference, "", []byte(`{}`))
			}
		}(i)
	}
	wg.Wait()
	got := k.get(p.ID)
	if bal := k.a.Balance(k.ctx, k.m.ID, true); got.Status != domain.PaymentSucceeded || bal != got.Net {
		t.Fatalf("status %s balance %d net %d", got.Status, bal, got.Net)
	}
}

func TestQAConcurrentWithdrawalsStayWithinBalance(t *testing.T) {
	a, ctx := settlementApp(t)
	m := qaMerchant(t, a, ctx, 4, qaBim)
	bal := a.Balance(ctx, m.ID, true)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := a.RequestPayout(ctx, m.ID, true, bal*6/10, "u"); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 || a.Balance(ctx, m.ID, true) < 0 {
		t.Fatalf("%d withdrawals of 60%% went through, balance %d", ok, a.Balance(ctx, m.ID, true))
	}
}

// Two partners pay the last two parts of a withdrawal at the same moment: one
// of them must close it.
func TestQALastTwoPartsPaidAtOnceLeaveTheWithdrawalOpen(t *testing.T) {
	a, ctx := settlementApp(t)
	stuck := 0
	for n := 0; n < 12; n++ {
		m := qaMerchant(t, a, ctx, 100+n, qaBim)
		po, err := a.RequestPayout(ctx, m.ID, true, 250000, "u")
		if err != nil {
			t.Fatal(err)
		}
		parts := a.PayoutFunding(ctx, []string{po.ID})[po.ID]
		if len(parts) != 2 {
			t.Fatalf("parts %+v", parts)
		}
		if _, err := a.TransitionPayout(ctx, po.ID, domain.ActApprove, "", "admin"); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, f := range parts {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				<-start
				if _, err := a.PayFunding(ctx, id, "", "TRF-"+id, receipt(), "admin"); err != nil {
					t.Logf("pay part: %v", err)
				}
			}(f.ID)
		}
		close(start)
		wg.Wait()
		p, _ := a.Payouts.Lock(ctx, po.ID)
		allPaid := true
		for _, f := range a.PayoutFunding(ctx, []string{po.ID})[po.ID] {
			allPaid = allPaid && f.Status == domain.FundingPaid
		}
		if allPaid && p.Status != domain.PayoutPaid {
			stuck++
		}
	}
	if stuck > 0 {
		t.Fatalf("%d of 12 withdrawals have every part paid but are still %q: the merchant is never told, and the admin can still fail them", stuck, domain.PayoutSent)
	}
}

// The public API's Idempotency-Key: the same withdrawal sent twice at once,
// as a client retrying a slow request does, must make one withdrawal.
func TestQAIdempotencyKeyRaceMakesTwoWithdrawals(t *testing.T) {
	a, ctx := settlementApp(t)
	m := qaMerchant(t, a, ctx, 5, qaBim)
	secret, _, err := a.RollSecretKey(ctx, m, true, app.Actor{ID: "u"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	var wg sync.WaitGroup
	codes := make([]int, 6)
	start := make(chan struct{})
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req, _ := http.NewRequest("POST", srv.URL+"/v1/payouts", strings.NewReader(`{"amount":20000}`))
			req.Header.Set("Authorization", "Bearer "+secret)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "levantamento-semanal-1")
			res, err := http.DefaultClient.Do(req)
			if err == nil {
				codes[i] = res.StatusCode
				_ = res.Body.Close()
			}
		}(i)
	}
	close(start)
	wg.Wait()
	live := true
	ps, _ := a.Payouts.Find(ctx, ports.PayoutQuery{MerchantID: m.ID, Live: &live})
	t.Logf("status codes %v, withdrawals made %d", codes, len(ps))
	if len(ps) != 1 {
		t.Fatalf("one Idempotency-Key made %d withdrawals of 200 MT", len(ps))
	}
}

// A parent whose first invoice payment is still being confirmed (unconfirmed)
// opens the invoice again: the checkout warns before a second charge
// (UnconfirmedNotice, PayAgain), the invoice must too.
func TestQAInvoiceSkipsTheUnconfirmedWarning(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-10-19 10:00")
	a.Now = func() time.Time { return clock }
	rail := &lateRail{err: ports.ErrStillPending}
	a.Rail = rail
	m := approved(t, a, ctx, "Escola QA2", "escola-qa2@example.com")
	p := schoolPlan(t, a, ctx, m, "Propina QA2", 250000, domain.MissedDebt)
	mem, err := a.InviteMember(ctx, m, false, app.MemberInput{PlanIDs: []string{p.ID}, Phone: "84 123 4567", PayerName: "Tânia",
		Fields: map[string]string{"Nome do aluno": "Kelvin", "Turma": "3ª B"}}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	tok := memberCharges(a, ctx, mem.ID)[0].Token
	first, err := a.PayInvoice(ctx, tok, app.InvoicePay{Method: "mpesa", UseLast: true, Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(11 * time.Minute)
	a.Sweep(ctx)
	if got, _ := a.Payments.ByID(ctx, first.ID); got.Status != domain.PaymentUnconfirmed {
		t.Fatalf("first %s", got.Status)
	}
	second, err := a.PayInvoice(ctx, tok, app.InvoicePay{Method: "mpesa", UseLast: true, Accept: true})
	if err == nil {
		t.Fatalf("a second charge %s (%s) went to the phone with no warning while %s may already be paid", second.ID, second.Status, first.ID)
	}
	t.Logf("refused: %v", err)
}

// What mtoca still holds overall bounds what it can be asked to pay: two
// merchants withdrawing at once must not both be given the same room.
func TestQATwoMerchantsWithdrawingAtOnceOverbookAPartner(t *testing.T) {
	qaRoomRounds(t, true)
}

// The same withdrawals one after the other: the control, which must pass.
func TestQATwoMerchantsWithdrawingInTurnStayWithinThePartner(t *testing.T) {
	qaRoomRounds(t, false)
}

func qaRoomRounds(t *testing.T, together bool) {
	a, ctx := settlementApp(t)
	overbooked := 0
	for n := 0; n < 5; n++ {
		ms := []*domain.Merchant{}
		for i := 0; i < 2; i++ {
			m := approved(t, a, ctx, fmt.Sprintf("QA Room %d-%d", n, i), fmt.Sprintf("qa-room-%d-%d@example.com", n, i))
			payLive(t, a, ctx, m, 200000, "emola", "861234231") // mtoca: 1 880 MT for the merchant
			if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "Millennium bim", Holder: m.Name, NIB: qaBim}, app.Actor{}); err != nil {
				t.Fatal(err)
			}
			ms = append(ms, m)
		}
		// mtoca pays RIHA most of what it holds: 300 MT of room is left.
		rep, _ := a.Settlement(ctx, nil, nil)
		var held int64
		for _, p := range rep.Partners {
			if p.Integration.Code == domain.IntegrationMtoca {
				held = p.Held - p.Reserved
			}
		}
		if _, err := a.RecordSettlement(ctx, app.SettlementInput{Integration: domain.IntegrationMtoca, Amount: held - 30000, Reference: fmt.Sprintf("TRF-%d", n), ReceivedOn: time.Now().Format("2006-01-02")}, admin); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		start := make(chan struct{})
		for _, m := range ms {
			wg.Add(1)
			run := func(m *domain.Merchant) {
				defer wg.Done()
				<-start
				if _, err := a.RequestPayout(ctx, m.ID, true, 25000, "u"); err != nil {
					t.Error(err)
				}
			}
			if together {
				go run(m)
			} else {
				close(start)
				run(m)
				start = make(chan struct{})
			}
		}
		if together {
			close(start)
		}
		wg.Wait()
		rep, _ = a.Settlement(ctx, nil, nil)
		for _, p := range rep.Partners {
			if p.Integration.Code == domain.IntegrationMtoca && p.Held < p.Reserved {
				overbooked++
				t.Logf("round %d: mtoca holds %d but is asked to pay %d", n, p.Held, p.Reserved)
			}
		}
		// Clear for the next round.
		live := true
		ps, _ := a.Payouts.Find(ctx, ports.PayoutQuery{Live: &live, Statuses: domain.OpenPayoutStatuses})
		for _, p := range ps {
			_, _ = a.TransitionPayout(ctx, p.ID, domain.ActReject, "qa", "admin")
		}
	}
	if overbooked > 0 {
		t.Fatalf("in %d of 5 rounds mtoca was asked to pay merchants more than it holds", overbooked)
	}
}
