package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, domain.Maputo)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func schoolPlan(t *testing.T, a *app.App, ctx context.Context, m *domain.Merchant, name string, amount int64, missed string) *domain.Plan {
	t.Helper()
	p, err := a.CreatePlan(ctx, m, false, app.PlanInput{Name: name, Amount: amount, Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed,
		BillingDay: 5, Months: 0x7FE, Missed: missed, Fields: []string{"Nome do aluno", "Turma"}}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func memberCharges(a *app.App, ctx context.Context, memberID string) []domain.Charge {
	return a.ChargesFor(ctx, ports.ChargeQuery{MemberID: memberID})
}

func TestSubscriptionLifecycle(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-10-19 10:00")
	a.Now = func() time.Time { return clock }
	m := approved(t, a, ctx, "Escola Primavera", "escola@primavera.co.mz")
	p := schoolPlan(t, a, ctx, m, "Propina 1ª a 4ª classe", 250000, domain.MissedDebt)
	if p.ScheduleLabel() != "Dia 5 · fev a nov" {
		t.Fatalf("schedule label %q", p.ScheduleLabel())
	}

	mem, err := a.InviteMember(ctx, m, false, app.MemberInput{PlanIDs: []string{p.ID}, Phone: "84 123 4567", PayerName: "Tânia Mabunda",
		Fields: map[string]string{"Nome do aluno": "Kelvin Mabunda", "Turma": "3ª B"}}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	if mem.Name != "Kelvin Mabunda" || mem.Status != domain.SubInvited {
		t.Fatalf("member %+v", mem)
	}
	if _, err := a.InviteMember(ctx, m, false, app.MemberInput{PlanIDs: []string{p.ID}, Phone: "841234567", Fields: map[string]string{"Nome do aluno": "Kelvin Mabunda", "Turma": "3ª B"}}, app.Actor{}); err == nil {
		t.Fatal("duplicate invite must fail")
	}
	cs := memberCharges(a, ctx, mem.ID)
	if len(cs) != 1 || cs[0].Label != "Outubro 2026" || cs[0].Status != domain.ChargeOpen || len(cs[0].Token) < 16 {
		t.Fatalf("first charge %+v", cs)
	}
	a.SendQueued(ctx)

	v, err := a.Invoice(ctx, cs[0].Token, true)
	if err != nil || !v.NeedsConsent || v.Total != 250000 {
		t.Fatalf("invoice %+v %v", v, err)
	}
	if _, err := a.PayInvoice(ctx, cs[0].Token, app.InvoicePay{Method: "mpesa", UseLast: true}); err == nil {
		t.Fatal("paying without accepting the agreement must fail")
	}
	pay, err := a.PayInvoice(ctx, cs[0].Token, app.InvoicePay{Method: "mpesa", UseLast: true, Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	if pay.Status != domain.PaymentSucceeded {
		t.Fatalf("payment %s", pay.Status)
	}
	d, _ := a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Member.Status != domain.SubActive || d.Subs[0].ConsentAt == nil || !strings.Contains(d.Subs[0].ConsentText, "dia 5 de cada mês") {
		t.Fatalf("after first payment: %+v %+v", d.Member, d.Subs[0])
	}
	if d.Subs[0].NextDueAt == nil || domain.PeriodKey(*d.Subs[0].NextDueAt) != "2026-11-05" {
		t.Fatalf("next due %v", d.Subs[0].NextDueAt)
	}
	balanceAfterFirst := a.Balance(ctx, m.ID, false)
	if balanceAfterFirst != 250000-15000 {
		t.Fatalf("balance %d", balanceAfterFirst)
	}

	clock = at("2026-11-02 09:30")
	a.BillingTick(ctx)
	cs = memberCharges(a, ctx, mem.ID)
	if len(cs) != 2 || cs[1].Label != "Novembro 2026" || cs[1].Status != domain.ChargeScheduled || !cs[1].Done(domain.StepBefore) {
		t.Fatalf("november %+v", cs)
	}
	clock = at("2026-11-05 09:30")
	a.BillingTick(ctx)
	nov := memberCharges(a, ctx, mem.ID)[1]
	if nov.Status != domain.ChargeOpen {
		t.Fatalf("november must be open, got %s", nov.Status)
	}
	if _, err := a.PayInvoice(ctx, nov.Token, app.InvoicePay{Method: "mpesa", Phone: "841231111"}); err != nil {
		t.Fatal(err)
	}
	nov = memberCharges(a, ctx, mem.ID)[1]
	if nov.Status != domain.ChargeOpen || !strings.HasPrefix(nov.LastFailure, "M-Pesa: ") {
		t.Fatalf("failed attempt %+v", nov)
	}
	clock = at("2026-11-07 09:30")
	a.BillingTick(ctx)
	d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Member.Status != domain.SubPastDue || d.Member.Debt != 250000 {
		t.Fatalf("past due %+v", d.Member)
	}
	clock = at("2026-11-15 10:00")
	a.BillingTick(ctx)
	if d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID); d.Member.Status != domain.SubPastDue {
		t.Fatalf("nothing happens on Sunday, got %s", d.Member.Status)
	}
	clock = at("2026-11-16 09:30")
	a.BillingTick(ctx)
	d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Member.Status != domain.SubSuspended {
		t.Fatalf("suspended after tolerance, got %s", d.Member.Status)
	}
	last := 0
	for _, msg := range d.Messages {
		if msg.Kind == "final" {
			last++
		}
	}
	if last != 1 {
		t.Fatalf("one final message expected, got %d", last)
	}

	if _, err := a.MarkPaidOutside(ctx, m, false, mem.ID, app.OutsideInput{ChargeIDs: []string{nov.ID}, Method: "cash", Date: clock}, app.Actor{ID: "owner"}, "Ana C."); err != nil {
		t.Fatal(err)
	}
	d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Member.Status != domain.SubActive || d.Member.Debt != 0 || d.Member.LastPaidVia != "outside" {
		t.Fatalf("after paid outside %+v", d.Member)
	}
	if a.Balance(ctx, m.ID, false) != balanceAfterFirst {
		t.Fatal("paid outside must not touch the balance")
	}
	if d.Subs[0].NextDueAt == nil || domain.PeriodKey(*d.Subs[0].NextDueAt) != "2027-02-05" {
		t.Fatalf("december and january are off, next %v", d.Subs[0].NextDueAt)
	}
	o := a.SubscriptionOverview(ctx, m, false)
	if o.Month.Outside != 250000 || o.Month.OutsideCount != 1 || o.Members != 1 {
		t.Fatalf("overview %+v", o.Month)
	}

	ch, err := a.ChangePrice(ctx, m, false, p.ID, 280000, app.PriceScopeAll, nil, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Affected != 1 || ch.EffectsAt == nil || domain.PeriodKey(*ch.EffectsAt) != "2027-02-05" || ch.Plan.Version != 2 {
		t.Fatalf("price change %+v", ch)
	}
	clock = at("2027-02-02 09:30")
	a.BillingTick(ctx)
	cs = memberCharges(a, ctx, mem.ID)
	if feb := cs[len(cs)-1]; feb.Label != "Fevereiro 2027" || feb.Amount != 280000 {
		t.Fatalf("february %+v", feb)
	}
	if err := a.CancelSubscription(ctx, m, false, d.Subs[0].ID, "saiu", app.Actor{ID: "owner"}); err != nil {
		t.Fatal(err)
	}
	if d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID); d.Member.Status != domain.SubCanceled {
		t.Fatalf("canceled, got %s", d.Member.Status)
	}
}

func TestDebtAccumulatesAndOldestFirst(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-10-19 10:00")
	a.Now = func() time.Time { return clock }
	m := approved(t, a, ctx, "Escola Sol", "escola@sol.co.mz")
	p := schoolPlan(t, a, ctx, m, "Propina 5ª a 7ª classe", 300000, domain.MissedDebt)
	tr, err := a.CreatePlan(ctx, m, false, app.PlanInput{Name: "Transporte escolar", Amount: 120000, Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed,
		BillingDay: 5, Months: 0x7FE, Missed: domain.MissedForfeit, Fields: []string{"Nome do aluno"}}, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	m.Status = domain.MerchantActive
	tok, err := a.Subscribe(ctx, m.Slug, app.SubscribeInput{MemberInput: app.MemberInput{PlanIDs: []string{p.ID, tr.ID}, Phone: "871112233",
		Fields: map[string]string{"Nome do aluno": "Ivan Nhantumbo", "Turma": "6ª A"}}, Accept: true})
	if err != nil {
		t.Fatal(err)
	}
	v, _ := a.Invoice(ctx, tok, false)
	if len(v.Lines) != 2 || v.Total != 420000 || v.NeedsConsent {
		t.Fatalf("catalog first payment %+v", v)
	}
	if _, err := a.PayInvoice(ctx, tok, app.InvoicePay{Method: "emola", UseLast: true}); err != nil {
		t.Fatal(err)
	}
	clock = at("2026-11-07 09:30")
	a.BillingTick(ctx)
	a.BillingTick(ctx)
	clock = at("2026-12-01 10:00")
	a.BillingTick(ctx)
	mem := v.Member
	d, _ := a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	var propinaNov, transNov domain.Charge
	for _, c := range d.Charges {
		if c.Label == "Novembro 2026" && c.PlanID == p.ID {
			propinaNov = c
		}
		if c.Label == "Novembro 2026" && c.PlanID == tr.ID {
			transNov = c
		}
	}
	if propinaNov.Status != domain.ChargePastDue || transNov.Status != domain.ChargeForfeited {
		t.Fatalf("debt plan keeps debt (%s), forfeit plan loses the month (%s)", propinaNov.Status, transNov.Status)
	}
	if d.Member.Debt != 300000 {
		t.Fatalf("debt %d", d.Member.Debt)
	}
	clock = at("2027-02-05 09:30")
	a.BillingTick(ctx)
	a.BillingTick(ctx)
	d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Member.Status != domain.SubSuspended {
		t.Fatalf("propina suspended, got %s", d.Member.Status)
	}
	v, _ = a.Invoice(ctx, propinaNov.Token, false)
	var feb *domain.Charge
	for _, l := range v.Lines {
		if l.Charge.Label == "Fevereiro 2027" && l.Charge.PlanID == p.ID {
			c := l.Charge
			feb = &c
		}
	}
	if feb != nil {
		if _, err := a.PayInvoice(ctx, propinaNov.Token, app.InvoicePay{ChargeIDs: []string{feb.ID}, Method: "emola", UseLast: true}); err == nil || !strings.Contains(err.Error(), "primeiro") {
			t.Fatalf("must pay oldest first, got %v", err)
		}
	}
	if _, err := a.PayInvoice(ctx, propinaNov.Token, app.InvoicePay{ChargeIDs: []string{propinaNov.ID}, Method: "emola", UseLast: true}); err != nil {
		t.Fatal(err)
	}
	d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Member.Debt != 0 || (d.Member.Status != domain.SubActive && d.Member.Status != domain.SubPastDue) {
		t.Fatalf("after paying the debt %+v", d.Member)
	}
}

func TestImportPreviewAndPause(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-10-19 10:00")
	a.Now = func() time.Time { return clock }
	m := approved(t, a, ctx, "Escola Lua", "escola@lua.co.mz")
	_ = schoolPlan(t, a, ctx, m, "Propina 1ª a 4ª classe", 250000, domain.MissedDebt)
	csv := "Nome do aluno;Turma;Número;Plano;Encarregado\nAna Sitoe;1ª A;85 220 1145;Propina 1ª a 4ª;Carlos Sitoe\nRui Cumbe;2ª B;82 11 04;Propina;\nHélio Cossa;2ª A;866620419;propina 1a 4a;Pedro Cossa\n"
	pv, err := a.PreviewImport(ctx, m, false, "alunos.csv", []byte(csv), "")
	if err != nil {
		t.Fatal(err)
	}
	if pv.Ready != 2 || pv.Invalid != 1 || pv.Rows[0].Error != "Número inválido" {
		t.Fatalf("preview %+v", pv)
	}
	res, err := a.ImportMembers(ctx, m, false, "alunos.csv", []byte(csv), "", app.Actor{ID: "owner"})
	if err != nil || res.Invited != 2 {
		t.Fatalf("import %+v %v", res, err)
	}
	ms, n := a.MemberList(ctx, ports.MemberQuery{MerchantID: m.ID, Live: false, Search: "sitoe"})
	if n != 1 || ms[0].PayerName != "Carlos Sitoe" || ms[0].Fields["Turma"] != "1ª A" {
		t.Fatalf("search %+v", ms)
	}
	if vals := a.MemberFieldValues(ctx, m.ID, false, "Turma"); len(vals) != 2 {
		t.Fatalf("field values %v", vals)
	}
	c := memberCharges(a, ctx, ms[0].ID)[0]
	if _, err := a.PayInvoice(ctx, c.Token, app.InvoicePay{Method: "mpesa", UseLast: true, Accept: true}); err != nil {
		t.Fatal(err)
	}
	d, _ := a.MemberDetailFor(ctx, m.ID, false, ms[0].ID)
	if err := a.PauseSubscription(ctx, m, false, d.Subs[0].ID, at("2027-03-01 00:00"), app.Actor{}); err != nil {
		t.Fatal(err)
	}
	clock = at("2026-11-03 10:00")
	a.BillingTick(ctx)
	d, _ = a.MemberDetailFor(ctx, m.ID, false, ms[0].ID)
	if d.Charges[0].Status != domain.ChargeSkipped || d.Member.Status != domain.SubPaused {
		t.Fatalf("paused member skips november: %+v", d.Charges[0])
	}
	if _, err := a.SkipNext(ctx, m, false, d.Subs[0].ID, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if err := a.SendNextNow(ctx, m, true, d.Subs[0].ID, app.Actor{}); err == nil {
		t.Fatal("send now is test mode only")
	}
}

func TestScheduleMath(t *testing.T) {
	p := &domain.Plan{Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed, BillingDay: 5, Months: 0x7FE}
	first, _ := domain.FirstDue(p, at("2026-12-10 10:00"))
	if domain.PeriodKey(first) != "2027-02-05" {
		t.Fatalf("december is off, first %s", domain.PeriodKey(first))
	}
	s := &domain.Plan{Interval: domain.IntervalMonth, Anchor: domain.AnchorSignup, Months: domain.AllMonths}
	f, _ := domain.FirstDue(s, at("2026-01-31 10:00"))
	n, _ := domain.NextDue(s, f, f)
	if domain.PeriodKey(n) != "2026-02-28" {
		t.Fatalf("end of month clamps, got %s", domain.PeriodKey(n))
	}
	w := &domain.Plan{Interval: domain.IntervalWeek, Anchor: domain.AnchorFixed, BillingDay: 1, Months: domain.AllMonths}
	wf, _ := domain.FirstDue(w, at("2026-11-04 10:00"))
	if domain.PeriodKey(wf) != "2026-11-02" {
		t.Fatalf("weekly monday, got %s", domain.PeriodKey(wf))
	}
	if domain.MonthsLabel(0x7FE) != "fev a nov" || domain.MonthsLabel(domain.AllMonths) != "todos os meses" {
		t.Fatal(domain.MonthsLabel(0x7FE))
	}
	if domain.StepAt(at("2026-11-05 00:00"), domain.ReminderStep{Days: 10, Hour: 9}).In(domain.Maputo).Day() != 16 {
		t.Fatal("a step on Sunday moves to Monday")
	}
}
