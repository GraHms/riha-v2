package integration

import (
	"testing"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// A plan with a number of payments ends by itself once the last one is paid.
func TestLimitedPlanEndsAfterTheLastPayment(t *testing.T) {
	a, ctx := setup(t)
	clock := at("2026-11-05 10:00")
	a.Now = func() time.Time { return clock }
	m := approved(t, a, ctx, "Escola de Línguas", "linguas@example.com")
	p, err := a.CreatePlan(ctx, m, false, app.PlanInput{Name: "Inglês intensivo", Amount: 150000, Interval: domain.IntervalMonth, Anchor: domain.AnchorSignup,
		Months: domain.AllMonths, Missed: domain.MissedDebt, Fields: []string{"Nome do aluno"}, Cycles: 2, Highlights: []string{"Livro incluído"}}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	mem, err := a.InviteMember(ctx, m, false, app.MemberInput{PlanIDs: []string{p.ID}, Phone: "84 123 4567", Fields: map[string]string{"Nome do aluno": "Rui"}}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	pay := func() {
		t.Helper()
		for _, c := range memberCharges(a, ctx, mem.ID) {
			if c.Unpaid() {
				if _, err := a.PayInvoice(ctx, c.Token, app.InvoicePay{Method: "mpesa", UseLast: true, Accept: true}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	pay()
	d, _ := a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Subs[0].Status != domain.SubActive || d.Subs[0].NextDueAt == nil || domain.PeriodKey(*d.Subs[0].NextDueAt) != "2026-12-05" {
		t.Fatalf("after the first payment: %+v", d.Subs[0])
	}
	// The second and last payment.
	clock = at("2026-12-05 09:30")
	a.BillingTick(ctx)
	pay()
	d, _ = a.MemberDetailFor(ctx, m.ID, false, mem.ID)
	if d.Subs[0].Status != domain.SubEnded || d.Subs[0].NextDueAt != nil || d.Member.Status != domain.SubEnded {
		t.Fatalf("after the last payment: sub %s next %v member %s", d.Subs[0].Status, d.Subs[0].NextDueAt, d.Member.Status)
	}
	clock = at("2027-01-06 10:00")
	a.BillingTick(ctx)
	if n := len(memberCharges(a, ctx, mem.ID)); n != 2 {
		t.Fatalf("an ended plan charged again: %d charges", n)
	}
	// Ended, the member can join again.
	if _, err := a.InviteMember(ctx, m, false, app.MemberInput{PlanIDs: []string{p.ID}, Phone: "84 123 4567", Fields: map[string]string{"Nome do aluno": "Rui"}}, app.Actor{ID: "owner"}); err != nil {
		t.Fatalf("joining again after the end: %v", err)
	}
}

// A catalog's address belongs to one merchant, and answers before account slugs.
func TestCatalogAddress(t *testing.T) {
	a, ctx := setup(t)
	one := approved(t, a, ctx, "Ginásio Forte", "forte@example.com")
	two := approved(t, a, ctx, "Escola Sol", "sol@example.com")
	p, err := a.CreatePlan(ctx, one, false, app.PlanInput{Name: "Mensal", Amount: 100000, Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed, BillingDay: 1,
		Months: domain.AllMonths, Missed: domain.MissedDebt}, app.Actor{ID: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	entries := []app.CatalogEntry{{PlanID: p.ID, InCatalog: true}}
	if err := a.SaveCatalog(ctx, one, false, entries, true, &app.CatalogPage{Slug: "forte", Title: "Planos 2027", Accent: "#1d4ed8"}, app.Actor{ID: "owner"}); err != nil {
		t.Fatal(err)
	}
	if got, _, _, err := a.PublicCatalog(ctx, "forte"); err != nil || got.ID != one.ID {
		t.Fatalf("by address: %v", err)
	}
	if got, _, _, err := a.PublicCatalog(ctx, one.Slug); err != nil || got.ID != one.ID {
		t.Fatalf("the old address must keep working: %v", err)
	}
	for _, slug := range []string{"forte", one.Slug, "www", "a"} {
		if err := a.SaveCatalog(ctx, two, false, nil, true, &app.CatalogPage{Slug: slug}, app.Actor{ID: "owner"}); err == nil {
			t.Fatalf("%q must be refused", slug)
		}
	}
	if s := a.SubSettingsFor(ctx, one); s.CatalogAccent != "#1D4ED8" || s.CatalogTitle != "Planos 2027" {
		t.Fatalf("settings %+v", s)
	}
}
