package domain

import (
	"strings"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, Maputo)
	return t
}

func TestCyclesStopAfterTheLastPayment(t *testing.T) {
	p := &Plan{Interval: IntervalMonth, Anchor: AnchorFixed, BillingDay: 5, Months: AllMonths, Missed: MissedDebt, Cycles: 3, Name: "Curso", Amount: 100000}
	first, ok := FirstDue(p, at("2026-11-05"))
	if !ok {
		t.Fatal("no first payment")
	}
	ds := Schedule(p, first, 10)
	if len(ds) != 3 {
		t.Fatalf("want 3 payments, got %d: %v", len(ds), ds)
	}
	if got := PeriodKey(ds[2]); got != "2027-01-05" {
		t.Fatalf("last payment %s", got)
	}
	if _, ok := NextDue(p, first, ds[2]); ok {
		t.Fatal("a fourth payment")
	}
}

func TestCyclesSkipMonthsWithoutPayments(t *testing.T) {
	// Paid February to November: three payments from November end in March.
	p := &Plan{Interval: IntervalMonth, Anchor: AnchorFixed, BillingDay: 5, Months: AllMonths &^ (1<<11 | 1<<0), Cycles: 3}
	first, _ := FirstDue(p, at("2026-11-05"))
	ds := Schedule(p, first, 10)
	var keys []string
	for _, d := range ds {
		keys = append(keys, PeriodKey(d))
	}
	if strings.Join(keys, " ") != "2026-11-05 2027-02-05 2027-03-05" {
		t.Fatalf("got %v", keys)
	}
}

func TestCustomerWhenUsesTheJoinDay(t *testing.T) {
	p := &Plan{Interval: IntervalMonth, Anchor: AnchorSignup, Months: AllMonths}
	if got := p.CustomerWhen(at("2026-11-18")); got != "Todos os meses, no dia 18" {
		t.Fatal(got)
	}
	w := &Plan{Interval: IntervalWeek, Anchor: AnchorSignup, Months: AllMonths}
	if got := w.CustomerWhen(at("2026-11-05")); got != "Todas as semanas, à quinta" {
		t.Fatal(got)
	}
	f := &Plan{Interval: IntervalMonth, Anchor: AnchorFixed, BillingDay: 5, Months: AllMonths}
	if got := f.CustomerWhen(at("2026-11-18")); got != "Todos os meses, no dia 5" {
		t.Fatal(got)
	}
	if strings.Contains(p.ScheduleLabel(), "Aniversário") {
		t.Fatal("anniversary is back")
	}
}

func TestAgreementNamesTheDay(t *testing.T) {
	p := &Plan{Interval: IntervalMonth, Anchor: AnchorSignup, Months: AllMonths, Cycles: 3}
	got := Agreement("Escola", p, 120000, "841234567", ChannelSMS, at("2026-11-18"))
	for _, want := range []string{"no dia 18 de cada mês", "3 vezes no total"} {
		if !strings.Contains(got, want) {
			t.Fatalf("%q lacks %q", got, want)
		}
	}
}

func TestCatalogSlug(t *testing.T) {
	for in, ok := range map[string]bool{"escola-primavera": true, "ab": false, "Escola": true, "pay": false, "www": false, "tickets": false, "-x-y": false, "a--b": false, "çafé": false, "x1y": true} {
		_, err := CleanCatalogSlug(in)
		if (err == nil) != ok {
			t.Errorf("%q: %v", in, err)
		}
	}
	if _, err := CleanColor("accent", "#12abEF"); err != nil {
		t.Error(err)
	}
	if _, err := CleanColor("accent", "red"); err == nil {
		t.Error("red is not #RRGGBB")
	}
}

func TestHighlightsAndLimitsValidate(t *testing.T) {
	p := &Plan{Name: "Curso", Amount: 100000, Interval: IntervalMonth, Anchor: AnchorFixed, BillingDay: 5, Months: AllMonths, Missed: MissedDebt,
		Highlights: []string{"  Material  incluído ", "", "Material incluído", "Lanche"}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(p.Highlights) != 2 || p.Highlights[0] != "Material incluído" {
		t.Fatalf("%q", p.Highlights)
	}
	end := at("2027-01-01")
	p.Cycles, p.EndsAt = 3, &end
	if err := p.Validate(); err == nil {
		t.Fatal("both a number of payments and an end date")
	}
}
