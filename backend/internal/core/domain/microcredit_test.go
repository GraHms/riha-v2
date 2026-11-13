package domain

import (
	"strings"
	"testing"
	"time"
)

func mz(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, Maputo) }

func TestCreditInstalmentMatchesTheDesign(t *testing.T) {
	cases := []struct {
		amount int64
		n, bp  int
		inst   int64
	}{
		{2000000, 4, 600, 620000},
		{2000000, 7, 600, 405800},
		{1500000, 7, 500, 289300},
		{1500000, 3, 600, 590000},
		{3000000, 6, 600, 680000},
		{800000, 8, 200, 116000},
		{100000, 1, 0, 100000},
	}
	for _, c := range cases {
		if got := CreditInstalment(c.amount, c.n, c.bp); got != c.inst {
			t.Errorf("%d over %d at %d: got %d, want %d", c.amount, c.n, c.bp, got, c.inst)
		}
	}
	p := &CreditProduct{Name: "Crédito Negócio", Freq: FreqMonthly, RateBP: 600, Min: 500000, Max: 5000000, TermMin: 2, TermMax: 12}
	q := p.Quote(2000000, 4, 12, 2500000, mz(2026, 11, 12, 8))
	if q.Total != 2480000 || q.Interest != 480000 || q.EffortPct != 25 {
		t.Fatalf("quote %+v", q)
	}
	if !q.FirstDue.Equal(mz(2026, 12, 12, 9)) || !q.Dates[3].Equal(mz(2027, 3, 12, 9)) {
		t.Fatalf("dates %v", q.Dates)
	}
}

func TestEffortCountsWeeksAsAMonth(t *testing.T) {
	if e := EffortPct(116000, FreqWeekly, 1500000); e != 34 {
		t.Fatalf("weekly effort %d", e)
	}
	if e := EffortPct(730300, FreqMonthly, 1872564); e != 39 {
		t.Fatalf("monthly effort %d", e)
	}
	if EffortPct(100, FreqMonthly, 0) != 0 {
		t.Fatal("no income must give 0")
	}
}

func TestDueDates(t *testing.T) {
	d := DueDates(FreqWeekly, 0, 3, mz(2026, 11, 12, 15))
	if !d[0].Equal(mz(2026, 11, 19, 9)) || !d[2].Equal(mz(2026, 12, 3, 9)) {
		t.Fatalf("weekly %v", d)
	}
	d = DueDates(FreqBiweekly, 0, 2, mz(2026, 11, 12, 15))
	if !d[1].Equal(mz(2026, 12, 10, 9)) {
		t.Fatalf("fortnightly %v", d)
	}
	d = DueDates(FreqMonthly, 1, 2, mz(2026, 11, 28, 10))
	if !d[0].Equal(mz(2027, 1, 1, 9)) {
		t.Fatalf("a day too close must move a month: %v", d)
	}
	d = DueDates(FreqMonthly, 31, 1, mz(2026, 1, 31, 10))
	if d[0].Day() != 28 {
		t.Fatalf("days past 28 must clamp: %v", d)
	}
	n := NextDates(20, 4, mz(2026, 11, 12, 10))
	if !n[0].Equal(mz(2026, 11, 20, 9)) || !n[3].Equal(mz(2027, 2, 20, 9)) {
		t.Fatalf("next dates %v", n)
	}
	n = NextDates(13, 1, mz(2026, 11, 12, 10))
	if !n[0].Equal(mz(2026, 12, 13, 9)) {
		t.Fatalf("a date less than two days away must move: %v", n)
	}
}

func TestMoraAfterToleranceForLimitedWeeks(t *testing.T) {
	due := mz(2026, 11, 5, 9)
	if m := Mora(680000, due, mz(2026, 11, 7, 20), 100, 2, 4); m != 0 {
		t.Fatalf("inside tolerance %d", m)
	}
	if m := Mora(680000, due, mz(2026, 11, 12, 10), 100, 2, 4); m != 6800 {
		t.Fatalf("7 days late %d", m)
	}
	if m := Mora(680000, due, mz(2026, 11, 15, 10), 100, 2, 4); m != 13600 {
		t.Fatalf("second week %d", m)
	}
	if m := Mora(680000, due, mz(2027, 3, 1, 10), 100, 2, 4); m != 27200 {
		t.Fatalf("capped at 4 weeks %d", m)
	}
	if Mora(680000, due, mz(2027, 3, 1, 10), 0, 2, 4) != 0 {
		t.Fatal("no mora when the product has none")
	}
}

func TestEvenSplitKeepsTheTotal(t *testing.T) {
	for _, c := range []struct {
		total int64
		n     int
	}{{2040000, 4}, {2046800, 2}, {1000100, 3}, {100, 1}} {
		parts := EvenSplit(c.total, c.n)
		var sum int64
		for _, p := range parts {
			sum += p
			if p%100 != 0 || p <= 0 {
				t.Fatalf("%v", parts)
			}
		}
		if sum != c.total {
			t.Fatalf("split of %d into %d gives %v", c.total, c.n, parts)
		}
	}
	if p := EvenSplit(2040000, 4); p[0] != 510000 || p[3] != 510000 {
		t.Fatalf("%v", p)
	}
}

func TestProductValidation(t *testing.T) {
	ok := CreditProduct{Name: " Crédito Escolar ", Min: 200000, Max: 3000000, Freq: FreqMonthly, TermMin: 1, TermMax: 10, RateBP: 500, DayMode: DayClient,
		Mora: true, MoraBP: 100, Grace: 2, MoraMaxWeeks: 4, EffortPct: 35, Fields: []string{FieldID, FieldID, " ", FieldIncome}}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	if ok.Name != "Crédito Escolar" || len(ok.Fields) != 2 || !ok.ClientPicksDay() {
		t.Fatalf("%+v", ok)
	}
	weekly := ok
	weekly.Freq, weekly.TermMin, weekly.TermMax = FreqWeekly, 4, 8
	if err := weekly.Validate(); err != nil || weekly.DayMode != DayAtDelivery {
		t.Fatalf("a weekly product can't let the client pick a day: %v %s", err, weekly.DayMode)
	}
	for name, f := range map[string]func(p *CreditProduct){
		"max below min":  func(p *CreditProduct) { p.Max = p.Min - 100 },
		"centavos":       func(p *CreditProduct) { p.Min = 200050 },
		"term too long":  func(p *CreditProduct) { p.TermMax = 37 },
		"usury":          func(p *CreditProduct) { p.RateBP = 3100 },
		"empty name":     func(p *CreditProduct) { p.Name = "  " },
		"bad frequency":  func(p *CreditProduct) { p.Freq = "diario" },
		"effort zero":    func(p *CreditProduct) { p.EffortPct = 0 },
		"mora too high":  func(p *CreditProduct) { p.MoraBP = 900 },
		"mora forever":   func(p *CreditProduct) { p.MoraMaxWeeks = 52 },
		"too many asked": func(p *CreditProduct) { p.Fields = strings.Split("a,b,c,d,e,f,g,h,i,j,k", ",") },
	} {
		p := ok
		f(&p)
		if p.Validate() == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestCheckTerms(t *testing.T) {
	p := &CreditProduct{Name: "Crédito Negócio", Freq: FreqMonthly, Min: 500000, Max: 5000000, TermMin: 2, TermMax: 12}
	if p.CheckTerms(2000000, 4, 12) != nil {
		t.Fatal("valid terms refused")
	}
	for _, c := range [][3]int64{{400000, 4, 12}, {5000100, 4, 12}, {2000050, 4, 12}, {2000000, 1, 12}, {2000000, 13, 12}, {2000000, 4, 29}, {2000000, 4, 0}} {
		if p.CheckTerms(c[0], int(c[1]), int(c[2])) == nil {
			t.Errorf("%v accepted", c)
		}
	}
}

func TestRateAndTermLabels(t *testing.T) {
	for bp, want := range map[int]string{600: "6% ao mês", 150: "1,5% ao mês", 105: "1,05% ao mês", 0: "0% ao mês"} {
		if got := RateLabel(bp, FreqMonthly); got != want {
			t.Errorf("%d: %q", bp, got)
		}
	}
	if RateLabel(200, FreqWeekly) != "2% por semana" || TermLabel(FreqMonthly, 1) != "1 mês" || TermLabel(FreqWeekly, 8) != "8 semanas" {
		t.Fatal("labels")
	}
}

func TestCreditSettingsClean(t *testing.T) {
	s := DefaultCreditSettings()
	if err := s.Clean(); err != nil {
		t.Fatal(err)
	}
	s.Steps = []CreditStep{{StepLink, false, 5}, {StepResend1, true, 9}}
	if err := s.Clean(); err == nil {
		t.Fatal("resend after the second resend was accepted")
	}
	s = DefaultCreditSettings()
	s.Steps = []CreditStep{{StepLink, false, 5}, {StepFinal, false, 40}}
	if err := s.Clean(); err != nil || !s.Step(StepLink).On || s.Step(StepLink).Days != 0 || !s.Step(StepFinal).On || s.Step(StepFinal).Days != 40 {
		t.Fatalf("locked steps must stay on: %v %+v", err, s.Steps)
	}
	s.ViaRiha, s.Outside = false, false
	if s.Clean() == nil {
		t.Fatal("no way to deliver was accepted")
	}
	s = DefaultCreditSettings()
	s.SenderID = "Avante MC <x>"
	if s.Clean() == nil {
		t.Fatal("bad sender accepted")
	}
}

func TestSendWindow(t *testing.T) {
	s := DefaultCreditSettings()
	if got := s.SendWindow(mz(2026, 11, 12, 6)); !got.Equal(mz(2026, 11, 12, 8)) {
		t.Fatalf("early morning %v", got)
	}
	if got := s.SendWindow(mz(2026, 11, 12, 21)); !got.Equal(mz(2026, 11, 13, 8)) {
		t.Fatalf("late evening %v", got)
	}
	if got := s.SendWindow(mz(2026, 11, 15, 10)); !got.Equal(mz(2026, 11, 16, 8)) {
		t.Fatalf("sunday %v", got)
	}
	if got := s.SendWindow(mz(2026, 11, 12, 10)); !got.Equal(mz(2026, 11, 12, 10)) {
		t.Fatalf("inside hours %v", got)
	}
}

func TestAgreementText(t *testing.T) {
	l := &Loan{Amount: 2000000, Freq: FreqMonthly, RateBP: 600, MoraBP: 100, Grace: 2, MoraMax: 4, Version: 1}
	q := (&CreditProduct{Freq: FreqMonthly, RateBP: 600}).Quote(2000000, 4, 12, 0, mz(2026, 11, 12, 8))
	txt := strings.ReplaceAll(CreditAgreementText("Avante Microcrédito", l, q.Dates, q.Instalments, 0), "\u00a0", " ")
	for _, want := range []string{"Recebo 20 000 MT de Avante Microcrédito", "4 prestações de 6 200 MT", "no dia 12 de cada mês", "24 800 MT", "4 800 MT de juros", "mais de 2 dias", "1% por semana", "4 semanas"} {
		if !strings.Contains(txt, want) {
			t.Errorf("missing %q in %s", want, txt)
		}
	}
	l.Version = 2
	txt = strings.ReplaceAll(CreditAgreementText("Avante", l, NextDates(20, 4, mz(2026, 11, 12, 8)), EvenSplit(2040000, 4), 6800), "\u00a0", " ")
	if !strings.Contains(txt, "Mora de 68 MT perdoada") || !strings.Contains(txt, "Substitui o plano anterior") {
		t.Fatal(txt)
	}
}
