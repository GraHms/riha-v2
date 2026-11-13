package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

type creditWorld struct {
	a     *app.App
	ctx   context.Context
	m     *domain.Merchant
	owner *domain.User
	clock *time.Time
	prod  *domain.CreditProduct
}

func creditSetup(t *testing.T) *creditWorld {
	t.Helper()
	a, ctx := setup(t)
	clock := time.Date(2026, 11, 12, 8, 0, 0, 0, domain.Maputo)
	a.Now = func() time.Time { return clock }
	boss := staff(t, a, ctx, "dona@riha.co.mz")
	m := approved(t, a, ctx, "Avante Microcrédito", "avante@example.com")
	m, err := a.SetMerchantMicrocredit(ctx, boss, m.ID, true, "contrato", app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	owner, _ := a.Users.ByEmail(ctx, "avante@example.com")
	p, err := a.CreateCreditProduct(ctx, m, false, app.ProductInput{Name: "Crédito Negócio", Min: 500000, Max: 5000000, Freq: domain.FreqMonthly,
		TermMin: 2, TermMax: 12, RateBP: 600, DayMode: domain.DayClient, Mora: true, MoraBP: 100, Grace: 2, MoraMaxWeeks: 4, EffortPct: 35,
		Fields: []string{domain.FieldID, domain.FieldActivity, domain.FieldIncome, domain.FieldPurpose}}, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	return &creditWorld{a: a, ctx: ctx, m: m, owner: owner, clock: &clock, prod: p}
}

func (w *creditWorld) at(t time.Time) { *w.clock = t }

func (w *creditWorld) request(t *testing.T, phone string) *domain.CreditRequest {
	t.Helper()
	r, code, err := w.a.StartCreditRequest(w.ctx, w.m.Slug, app.CreditRequestInput{ProductID: w.prod.ID, Amount: 2000000, Term: 4, Day: 12,
		Name: "Celeste Machava", Phone: phone, IDNumber: "110100456789B", Activity: "Vendo capulanas", Purpose: "Comprar stock", Income: 2500000})
	if err != nil {
		t.Fatal(err)
	}
	if code == "" {
		t.Fatal("test mode must show the code")
	}
	if r, err = w.a.ConfirmCreditRequest(w.ctx, r.Token, code); err != nil || r.Status != domain.RequestNew {
		t.Fatalf("confirm %v %+v", err, r)
	}
	return r
}

func (w *creditWorld) accept(t *testing.T, token string) *domain.Loan {
	t.Helper()
	code, err := w.a.SendAgreementCode(w.ctx, token, true)
	if err != nil {
		t.Fatal(err)
	}
	l, err := w.a.AcceptAgreement(w.ctx, token, code, "10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestCreditFromRequestToLastInstalment(t *testing.T) {
	w := creditSetup(t)
	a, ctx, m := w.a, w.ctx, w.m
	pay(t, a, ctx, m, 5000000, "841234231")
	before := a.Balance(ctx, m.ID, false)

	if _, _, err := a.StartCreditRequest(ctx, m.Slug, app.CreditRequestInput{ProductID: w.prod.ID, Amount: 2000000, Term: 4, Day: 12,
		Name: "Celeste Machava", Phone: "845550142", IDNumber: "110100456789B", Activity: "x", Purpose: "y", Income: 1000000}); err == nil || !strings.Contains(err.Error(), "limite") {
		t.Fatalf("a request over the effort limit went through: %v", err)
	}
	r := w.request(t, "845550142")
	if r.Instalment != 620000 || r.EffortPct != 25 {
		t.Fatalf("request %+v", r)
	}
	d, err := a.CreditRequestDetail(ctx, m, false, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Checks[0].OK || d.Quote.Total != 2480000 {
		t.Fatalf("detail %+v", d.Checks)
	}
	l, err := a.ApproveCreditRequest(ctx, m, false, r.ID, app.OfferInput{Amount: 2000000, Term: 4, Day: 12, Delivery: domain.DeliverRiha}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if l.Status != domain.LoanOffered || l.Token != r.Token {
		t.Fatalf("loan %+v", l)
	}
	if _, err := a.ApproveCreditRequest(ctx, m, false, r.ID, app.OfferInput{Amount: 2000000, Term: 4, Day: 12, Delivery: domain.DeliverRiha}, w.owner, app.Actor{}); err == nil {
		t.Fatal("a request was approved twice")
	}
	if _, err := a.AcceptAgreement(ctx, r.Token, "000000", ""); err == nil {
		t.Fatal("accepted without asking a code")
	}
	l = w.accept(t, r.Token)
	if l.Status != domain.LoanActive || l.DeliveryStatus != "sent" || l.Version != 1 {
		t.Fatalf("after accepting %+v", l)
	}
	if got := a.Balance(ctx, m.ID, false); got != before-2000000 {
		t.Fatalf("balance %d, want %d", got, before-2000000)
	}
	v, _ := a.CreditLoan(ctx, m, false, l.ID)
	if len(v.Current) != 4 || v.Current[0].Amount != 620000 || !v.Current[0].DueAt.Equal(time.Date(2026, 12, 12, 9, 0, 0, 0, domain.Maputo)) || v.Debt != 2480000 {
		t.Fatalf("plan %+v debt %d", v.Current, v.Debt)
	}
	if len(v.Agreements) != 1 || v.Agreements[0].Status != domain.AgreementAccepted || v.Agreements[0].Phone != "845550142" {
		t.Fatalf("agreement %+v", v.Agreements)
	}

	p, err := a.PayLoan(ctx, r.Token, app.LoanPay{})
	if err != nil || p.Status != domain.PaymentSucceeded || p.Amount != 620000 {
		t.Fatalf("pay %v %+v", err, p)
	}
	v, _ = a.CreditLoan(ctx, m, false, l.ID)
	if v.Paid != 1 || v.PaidRiha != 620000 || v.Debt != 1860000 {
		t.Fatalf("after paying %+v", v)
	}

	w.at(time.Date(2027, 1, 21, 10, 0, 0, 0, domain.Maputo))
	v, _ = a.CreditLoan(ctx, m, false, l.ID)
	if v.DaysLate != 9 || v.Mora != 6200 || v.State != "Em atraso · 9 dias" {
		t.Fatalf("late %d days, mora %d, %s", v.DaysLate, v.Mora, v.State)
	}
	a.CreditTick(ctx)
	sent, _ := a.Messages.ByMember(ctx, l.ID, 50)
	a.CreditTick(ctx)
	again, _ := a.Messages.ByMember(ctx, l.ID, 50)
	if len(again) != len(sent) {
		t.Fatalf("the worker sent %d more messages on its second run", len(again)-len(sent))
	}
	found := false
	for _, msg := range sent {
		if msg.Kind == "credit_resend2" && strings.Contains(msg.Body, "em atraso há 9 dias") {
			found = true
		}
		if msg.Kind == "credit_resend1" {
			t.Fatal("a skipped step was sent late")
		}
	}
	if !found {
		t.Fatalf("no late message: %+v", sent)
	}

	if err := a.AskNewAgreement(ctx, r.Token); err != nil {
		t.Fatal(err)
	}
	v, err = a.ProposeAgreement(ctx, m, false, l.ID, app.AgreementInput{Option: "extend", ExtraMonths: 2, Day: 20, Waive: true}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if v.Proposal == nil || v.Proposal.Total != 1860000 || len(v.Proposal.Amounts) != 4 || v.Proposal.MoraWaived != 6200 || v.State != "Novo acordo enviado" {
		t.Fatalf("proposal %+v", v.Proposal)
	}
	l = w.accept(t, r.Token)
	v, _ = a.CreditLoan(ctx, m, false, l.ID)
	if l.Version != 2 || len(v.Current) != 4 || v.Current[0].Amount != 465000 || v.DaysLate != 0 || v.Debt != 1860000 {
		t.Fatalf("after the new agreement v%d %+v", l.Version, v.Current)
	}
	if !v.Current[0].DueAt.Equal(time.Date(2027, 2, 20, 9, 0, 0, 0, domain.Maputo)) {
		t.Fatalf("first new date %v", v.Current[0].DueAt)
	}
	replaced := 0
	for _, x := range v.Old {
		if x.Status == domain.InstReplaced {
			replaced++
		}
	}
	if replaced != 3 || v.Agreements[0].Status != domain.AgreementSuperseded {
		t.Fatalf("old plan: %d replaced, first agreement %s", replaced, v.Agreements[0].Status)
	}

	v, err = a.MarkInstalmentPaid(ctx, m, false, l.ID, v.Current[0].ID, app.MarkPaidInput{Method: "cash"}, w.owner, app.Actor{})
	if err != nil || v.PaidOutside != 465000 {
		t.Fatalf("mark paid %v %+v", err, v)
	}
	if _, err := a.SettleEarly(ctx, m, false, l.ID, app.SettleInput{Amount: 2000000, Outside: true, Method: "cash"}, w.owner, app.Actor{}); err == nil {
		t.Fatal("settled for more than is owed")
	}
	v, err = a.SettleEarly(ctx, m, false, l.ID, app.SettleInput{Amount: 1300000, Outside: true, Method: "transfer", Note: "desconto"}, w.owner, app.Actor{})
	if err != nil || v.Loan.Status != domain.LoanSettled || v.PaidOutside != 465000+1300000 {
		t.Fatalf("settle %v %+v", err, v)
	}
	if _, err := a.PayLoan(ctx, r.Token, app.LoanPay{}); err == nil {
		t.Fatal("paid a settled loan")
	}
	o, err := a.CreditOverview(ctx, m, false)
	if err != nil || o.BookLoans != 0 {
		t.Fatalf("overview %v %+v", err, o)
	}
}

func TestCreditSettleThroughTheLinkWithADiscount(t *testing.T) {
	w := creditSetup(t)
	a, ctx, m := w.a, w.ctx, w.m
	l, err := a.CreateLoan(ctx, m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "851184402", Name: "Hortência Macamo", Amount: 1500000, Term: 3, Day: 12, Delivery: domain.DeliverOutside}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	l = w.accept(t, l.Token)
	if l.Status != domain.LoanActive || l.DeliveryStatus != "to_deliver" {
		t.Fatalf("%+v", l)
	}
	if _, err := a.MarkDelivered(ctx, m, false, l.ID, "dinheiro no balcão", app.Actor{}); err != nil {
		t.Fatal(err)
	}
	v, err := a.SettleEarly(ctx, m, false, l.ID, app.SettleInput{Amount: 1600000}, w.owner, app.Actor{})
	if err != nil || v.Loan.SettleOffer != 1600000 {
		t.Fatalf("%v %+v", err, v)
	}
	p, err := a.PayLoan(ctx, l.Token, app.LoanPay{All: true})
	if err != nil || p.Amount != 1600000 || p.Status != domain.PaymentSucceeded {
		t.Fatalf("%v %+v", err, p)
	}
	v, _ = a.CreditLoan(ctx, m, false, l.ID)
	if v.Loan.Status != domain.LoanSettled || v.PaidRiha != 1600000 {
		t.Fatalf("%+v paid %d", v.Loan, v.PaidRiha)
	}
}

func TestCreditDeliveryFailsWithoutBalanceAndRetries(t *testing.T) {
	w := creditSetup(t)
	a, ctx, m := w.a, w.ctx, w.m
	l, err := a.CreateLoan(ctx, m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "860775120", Name: "Graça Nhaca", Amount: 600000, Term: 2, Day: 5, Delivery: domain.DeliverRiha}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	l = w.accept(t, l.Token)
	if l.DeliveryStatus != "failed" || l.Status != domain.LoanDisbursing || a.Balance(ctx, m.ID, false) != 0 {
		t.Fatalf("%+v balance %d", l, a.Balance(ctx, m.ID, false))
	}
	o, _ := a.CreditOverview(ctx, m, false)
	if len(o.Attention) == 0 || o.Attention[0].Kind != "delivery" {
		t.Fatalf("attention %+v", o.Attention)
	}
	pay(t, a, ctx, m, 1000000, "841234231")
	bal := a.Balance(ctx, m.ID, false)
	l, err = a.RetryDelivery(ctx, m, false, l.ID, app.Actor{})
	if err != nil || l.DeliveryStatus != "sent" || l.Status != domain.LoanActive || a.Balance(ctx, m.ID, false) != bal-600000 {
		t.Fatalf("%v %+v", err, l)
	}

	pay(t, a, ctx, m, 1000000, "841234231")
	l2, _ := a.CreateLoan(ctx, m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "860770000", Name: "Recusa Teste", Amount: 500000, Term: 2, Day: 5, Delivery: domain.DeliverRiha}, w.owner, app.Actor{})
	bal = a.Balance(ctx, m.ID, false)
	l2 = w.accept(t, l2.Token)
	if l2.DeliveryStatus != "failed" || a.Balance(ctx, m.ID, false) != bal || !strings.Contains(l2.DeliveryError, "recusou") {
		t.Fatalf("a refused delivery must put the money back: %+v %d", l2, a.Balance(ctx, m.ID, false))
	}
}

func TestCreditGuards(t *testing.T) {
	w := creditSetup(t)
	a, ctx, m := w.a, w.ctx, w.m
	other := approved(t, a, ctx, "Outra", "outra@example.com")
	r := w.request(t, "847301188")

	if _, err := a.CreditRequestDetail(ctx, other, false, r.ID); err == nil {
		t.Fatal("a merchant without microcredit read another's request")
	}
	boss, _ := a.Users.ByEmail(ctx, "dona@riha.co.mz")
	other, _ = a.SetMerchantMicrocredit(ctx, boss, other.ID, true, "", app.Actor{})
	if _, err := a.CreditRequestDetail(ctx, other, false, r.ID); err == nil {
		t.Fatal("one institution read another's request")
	}
	if _, err := a.CreditRequestDetail(ctx, m, true, r.ID); err == nil {
		t.Fatal("a test request showed in live mode")
	}
	if _, _, err := a.StartCreditRequest(ctx, m.Slug, app.CreditRequestInput{ProductID: w.prod.ID, Amount: 2000000, Term: 4, Day: 12,
		Name: "Celeste Machava", Phone: "847301188", IDNumber: "110100456789B", Activity: "x", Purpose: "y", Income: 9000000}); err == nil {
		t.Fatal("a second open request from the same number was taken")
	}

	r2, _, err := a.StartCreditRequest(ctx, m.Slug, app.CreditRequestInput{ProductID: w.prod.ID, Amount: 1000000, Term: 4, Day: 12,
		Name: "Momade Ussene", Phone: "872204410", IDNumber: "110100456780A", Activity: "Alfaiate", Purpose: "Máquina", Income: 9000000})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < domain.CodeMaxTries; i++ {
		if _, err := a.ConfirmCreditRequest(ctx, r2.Token, "000001"); err == nil {
			t.Fatal("a wrong code was taken")
		}
	}
	if _, err := a.ConfirmCreditRequest(ctx, r2.Token, "000001"); err == nil || !strings.Contains(err.Error(), "vezes demais") {
		t.Fatalf("after 5 wrong codes: %v", err)
	}
	if rs, _, _ := a.CreditRequests(ctx, m, false, "todos", "", ""); len(rs) != 1 {
		t.Fatalf("an unconfirmed request is visible: %d", len(rs))
	}

	if _, err := a.CreateLoan(ctx, m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "851184402", Name: "X Y", Amount: 6000000, Term: 3, Day: 12, Delivery: domain.DeliverOutside}, w.owner, app.Actor{}); err == nil {
		t.Fatal("a loan above the product's maximum")
	}
	s := a.CreditSettings(ctx, m.ID)
	s.TwoApprovers = true
	if _, err := a.SaveCreditSettings(ctx, m, s, app.Actor{}); err == nil {
		t.Fatal("two approvers turned on with one person in the account")
	}

	boss2, _ := a.Users.ByEmail(ctx, "dona@riha.co.mz")
	if _, err := a.SetMerchantMicrocredit(ctx, boss2, m.ID, false, "", app.Actor{}); err != nil {
		t.Fatal(err)
	}
	m, _ = a.Merchants.ByID(ctx, m.ID)
	var de *domain.Error
	if _, _, err := a.CreditRequests(ctx, m, false, "", "", ""); !errors.As(err, &de) || de.Code != "microcredit_not_enabled" {
		t.Fatalf("revoked, the panel still answers: %v", err)
	}
	if _, _, _, err := a.PublicCreditProducts(ctx, m.Slug); err == nil {
		t.Fatal("revoked, the public page still answers")
	}
}
