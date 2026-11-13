package domain

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Microcrédito: the institution lends, RIHA carries the request, the agreement
// and the collection. Amounts are in centavos; instalments are whole meticais.

const (
	FreqWeekly   = "semanal"
	FreqBiweekly = "quinzenal"
	FreqMonthly  = "mensal"

	DayAtDelivery = "entrega"
	DayClient     = "cliente"

	ProductActive = "active"
	ProductDraft  = "draft"

	FieldID       = "id_doc"
	FieldActivity = "activity"
	FieldIncome   = "income"
	FieldPurpose  = "purpose"

	SourceMicrocredit = "microcredit"
)

var CreditFreqs = []string{FreqWeekly, FreqBiweekly, FreqMonthly}

var termMax = map[string]int{FreqWeekly: 52, FreqBiweekly: 26, FreqMonthly: 36}

type CreditProduct struct {
	ID         string
	MerchantID string
	Livemode   bool
	Name       string
	Min, Max   int64
	Freq       string
	TermMin    int
	TermMax    int
	// RateBP is the flat interest per instalment period, in hundredths of a percent.
	RateBP       int
	DayMode      string
	Mora         bool
	MoraBP       int
	Grace        int
	MoraMaxWeeks int
	EffortPct    int
	Fields       []string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (p *CreditProduct) ClientPicksDay() bool { return p.Freq == FreqMonthly && p.DayMode == DayClient }

func (p *CreditProduct) Asks(field string) bool { return contains(p.Fields, field) }

func (p *CreditProduct) Validate() *Error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "" || len([]rune(p.Name)) > 60:
		return Invalid("parameter_invalid", "name", "Dê um nome ao produto, até 60 letras.")
	case !contains(CreditFreqs, p.Freq):
		return Invalid("parameter_invalid", "freq", "Escolha prestações semanais, quinzenais ou mensais.")
	case p.Min < 10000 || p.Min%100 != 0:
		return Invalid("parameter_invalid", "min", "O montante mínimo é de pelo menos 100 MT, em meticais inteiros.")
	case p.Max < p.Min || p.Max%100 != 0:
		return Invalid("parameter_invalid", "max", "O montante máximo tem de ser maior que o mínimo.")
	case p.Max > 100000000:
		return Invalid("parameter_invalid", "max", "O montante máximo é de 1 000 000 MT.")
	case p.TermMin < 1 || p.TermMax < p.TermMin || p.TermMax > termMax[p.Freq]:
		return Invalid("parameter_invalid", "term_max", fmt.Sprintf("O prazo vai de 1 a %s.", TermLabel(p.Freq, termMax[p.Freq])))
	case p.RateBP < 0 || p.RateBP > 3000:
		return Invalid("parameter_invalid", "rate", "A taxa vai de 0% a 30% por prestação.")
	case p.EffortPct < 5 || p.EffortPct > 100:
		return Invalid("parameter_invalid", "effort", "O limite de esforço vai de 5% a 100% do rendimento.")
	case p.Grace < 0 || p.Grace > 15:
		return Invalid("parameter_invalid", "grace", "A tolerância vai de 0 a 15 dias.")
	case p.Mora && (p.MoraBP <= 0 || p.MoraBP > 500):
		return Invalid("parameter_invalid", "mora", "A mora vai de 0,01% a 5% por semana.")
	case p.Mora && (p.MoraMaxWeeks < 1 || p.MoraMaxWeeks > 12):
		return Invalid("parameter_invalid", "mora_max", "A mora conta de 1 a 12 semanas.")
	}
	if p.DayMode != DayClient || p.Freq != FreqMonthly {
		p.DayMode = DayAtDelivery
	}
	if !p.Mora {
		p.MoraBP, p.MoraMaxWeeks = 0, 0
	}
	var fields []string
	for _, f := range p.Fields {
		f = strings.TrimSpace(f)
		if f == "" || contains(fields, f) {
			continue
		}
		if len([]rune(f)) > 40 {
			return Invalid("parameter_invalid", "fields", "Cada campo tem até 40 letras.")
		}
		fields = append(fields, f)
	}
	if len(fields) > 10 {
		return Invalid("parameter_invalid", "fields", "Peça até 10 coisas ao cliente.")
	}
	p.Fields = fields
	return nil
}

// RateLabel: "6% ao mês", "1,5% por semana".
func RateLabel(bp int, freq string) string {
	s := strings.TrimSuffix(strings.TrimSuffix(fmt.Sprintf("%d,%02d", bp/100, bp%100), "0"), ",0")
	return s + "% " + map[string]string{FreqWeekly: "por semana", FreqBiweekly: "por quinzena", FreqMonthly: "ao mês"}[freq]
}

func TermLabel(freq string, n int) string {
	u := map[string][2]string{FreqWeekly: {"semana", "semanas"}, FreqBiweekly: {"quinzena", "quinzenas"}, FreqMonthly: {"mês", "meses"}}[freq]
	if n == 1 {
		return "1 " + u[0]
	}
	return fmt.Sprintf("%d %s", n, u[1])
}

func FreqLabel(freq string) string {
	return map[string]string{FreqWeekly: "Semanal", FreqBiweekly: "Quinzenal", FreqMonthly: "Mensal"}[freq]
}

func PerLabel(freq string) string {
	return map[string]string{FreqWeekly: "sem", FreqBiweekly: "quinz", FreqMonthly: "mês"}[freq]
}

// Quote is what a loan costs: equal instalments in whole meticais, flat
// interest on the amount lent.
type Quote struct {
	Amount      int64
	N           int
	Instalment  int64
	Total       int64
	Interest    int64
	EffortPct   int
	FirstDue    time.Time
	Dates       []time.Time
	Instalments []int64
}

func CreditInstalment(amount int64, n, rateBP int) int64 {
	if n <= 0 {
		return 0
	}
	mt := amount / 100
	num := mt * int64(10000+rateBP*n)
	den := int64(10000 * n)
	return ((num + den - 1) / den) * 100
}

// periodsPerMonth turns an instalment into what it weighs on a month's income.
func periodsPerMonth(freq string) float64 {
	switch freq {
	case FreqWeekly:
		return 52.0 / 12
	case FreqBiweekly:
		return 26.0 / 12
	}
	return 1
}

func EffortPct(instalment int64, freq string, income int64) int {
	if income <= 0 {
		return 0
	}
	return int(math.Round(float64(instalment) * periodsPerMonth(freq) / float64(income) * 100))
}

// DueDates: monthly on the day, from the month after the start (a month later
// when the day is too close); weekly and fortnightly from the start.
func DueDates(freq string, day, n int, start time.Time) []time.Time {
	s := start.In(Maputo)
	base := time.Date(s.Year(), s.Month(), s.Day(), 9, 0, 0, 0, Maputo)
	out := make([]time.Time, n)
	switch freq {
	case FreqWeekly, FreqBiweekly:
		step := 7
		if freq == FreqBiweekly {
			step = 14
		}
		for i := range out {
			out[i] = base.AddDate(0, 0, step*(i+1))
		}
	default:
		if day < 1 || day > 28 {
			day = clampDay(s.Day())
		}
		first := time.Date(s.Year(), s.Month()+1, day, 9, 0, 0, 0, Maputo)
		if first.Sub(base) < 7*24*time.Hour {
			first = first.AddDate(0, 1, 0)
		}
		for i := range out {
			out[i] = first.AddDate(0, i, 0)
		}
	}
	return out
}

// NextDates: the next n dates on a day of the month, the first one at least two
// days away. Used for a new agreement.
func NextDates(day, n int, from time.Time) []time.Time {
	f := from.In(Maputo)
	first := time.Date(f.Year(), f.Month(), clampDay(day), 9, 0, 0, 0, Maputo)
	for first.Sub(f) < 48*time.Hour {
		first = first.AddDate(0, 1, 0)
	}
	out := make([]time.Time, n)
	for i := range out {
		out[i] = first.AddDate(0, i, 0)
	}
	return out
}

func clampDay(d int) int {
	if d > 28 {
		return 28
	}
	if d < 1 {
		return 1
	}
	return d
}

func DefaultDay(t time.Time) int { return clampDay(t.In(Maputo).Day()) }

func (p *CreditProduct) Quote(amount int64, n, day int, income int64, start time.Time) Quote {
	inst := CreditInstalment(amount, n, p.RateBP)
	q := Quote{Amount: amount, N: n, Instalment: inst, Total: inst * int64(n), EffortPct: EffortPct(inst, p.Freq, income)}
	q.Interest = q.Total - amount
	q.Dates = DueDates(p.Freq, day, n, start)
	for range q.Dates {
		q.Instalments = append(q.Instalments, inst)
	}
	if n > 0 {
		q.FirstDue = q.Dates[0]
	}
	return q
}

func (p *CreditProduct) CheckTerms(amount int64, n, day int) *Error {
	switch {
	case amount%100 != 0 || amount < p.Min || amount > p.Max:
		return Invalid("amount_out_of_range", "amount", fmt.Sprintf("Fora dos limites de %s: de %s a %s MT.", p.Name, FormatShort(p.Min), FormatShort(p.Max)))
	case n < p.TermMin || n > p.TermMax:
		return Invalid("term_out_of_range", "term", fmt.Sprintf("O prazo de %s vai de %d a %s.", p.Name, p.TermMin, TermLabel(p.Freq, p.TermMax)))
	case p.Freq == FreqMonthly && (day < 1 || day > 28):
		return Invalid("day_invalid", "day", "O dia de pagamento vai de 1 a 28.")
	}
	return nil
}

// EvenSplit spreads a total over n instalments in whole meticais, the last one
// taking what is left.
func EvenSplit(total int64, n int) []int64 {
	if n <= 0 {
		return nil
	}
	mt := total / 100
	a := (mt + int64(n) - 1) / int64(n)
	out := make([]int64, n)
	for i := range out {
		if i == n-1 {
			out[i] = (mt - a*int64(n-1)) * 100
		} else {
			out[i] = a * 100
		}
	}
	return out
}

// Mora on one late instalment: a share per week after the tolerance, counted
// from the first day past it, for a limited number of weeks.
func Mora(instalment int64, due, now time.Time, moraBP, grace, maxWeeks int) int64 {
	if moraBP <= 0 || maxWeeks <= 0 {
		return 0
	}
	days := DaysLate(due, now) - grace
	if days <= 0 {
		return 0
	}
	weeks := (days + 6) / 7
	if weeks > maxWeeks {
		weeks = maxWeeks
	}
	mt := float64(instalment/100) * float64(moraBP) / 10000 * float64(weeks)
	return int64(math.Round(mt)) * 100
}

func DaysLate(due, now time.Time) int {
	d := due.In(Maputo)
	n := now.In(Maputo)
	dd := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, Maputo)
	nd := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, Maputo)
	return int(nd.Sub(dd).Hours() / 24)
}

const (
	RequestVerifying = "verifying"
	RequestNew       = "new"
	RequestReview    = "review"
	RequestOffered   = "offered"
	RequestDeclined  = "declined"
	RequestCanceled  = "canceled"
)

type CreditRequest struct {
	ID          string
	MerchantID  string
	Livemode    bool
	ProductID   string
	Token       string
	Name        string
	Phone       string
	Wallet      string
	IDNumber    string
	NUIT        string
	Address     string
	Activity    string
	Purpose     string
	Income      int64
	Extra       map[string]string
	Amount      int64
	Term        int
	Day         int
	Instalment  int64
	EffortPct   int
	Status      string
	Source      string
	PhoneOKAt   *time.Time
	DeclineNote string
	DecidedBy   string
	DecidedAt   *time.Time
	LoanID      string
	IP          string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *CreditRequest) Open() bool { return r.Status == RequestNew || r.Status == RequestReview }

type CreditDocument struct {
	ID          string
	RequestID   string
	MerchantID  string
	Kind        string
	FileName    string
	StoredPath  string
	ContentType string
	Size        int64
	CreatedAt   time.Time
}

const (
	LoanOffered    = "offered"
	LoanApproving  = "approving"
	LoanDisbursing = "disbursing"
	LoanActive     = "active"
	LoanSettled    = "settled"
	LoanWrittenOff = "written_off"
	LoanCanceled   = "canceled"

	DeliverRiha    = "riha"
	DeliverOutside = "outside"
)

type Loan struct {
	ID         string
	MerchantID string
	Livemode   bool
	ProductID  string
	RequestID  string
	Token      string
	Reference  string
	Name       string
	Phone      string
	Wallet     string
	Amount     int64
	Term       int
	Day        int
	Freq       string
	RateBP     int
	MoraBP     int
	Grace      int
	MoraMax    int
	Status     string
	Delivery   string
	// DeliveryStatus: pending until the agreement is accepted, then sending,
	// sent, failed (from the RIHA balance) or delivered (outside RIHA).
	DeliveryStatus string
	DeliveryError  string
	DeliveredAt    *time.Time
	PayoutRef      string
	Version        int
	Proposed       int
	AskedAt        *time.Time
	Manual         bool
	SettleOffer    int64
	ApprovedBy     string
	SecondBy       string
	CreatedBy      string
	AcceptedAt     *time.Time
	ClosedAt       *time.Time
	CloseNote      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (l *Loan) Running() bool { return l.Status == LoanActive || l.Status == LoanDisbursing }

const (
	InstScheduled  = "scheduled"
	InstPaid       = "paid"
	InstPaidOut    = "paid_outside"
	InstReplaced   = "replaced"
	InstCanceled   = "canceled"
	InstWrittenOff = "written_off"
)

type Instalment struct {
	ID         string
	LoanID     string
	MerchantID string
	Livemode   bool
	Version    int
	Seq        int
	DueAt      time.Time
	Amount     int64
	Status     string
	MoraPaid   int64
	MoraWaived bool
	Charged    int64
	SessionID  *string
	PaymentID  *string
	Method     string
	PaidAt     *time.Time
	MarkedBy   string
	Note       string
	LinkSentAt *time.Time
	OpenedAt   *time.Time
	Steps      string
	Failure    string
	UpdatedAt  time.Time
}

func (i *Instalment) Open() bool { return i.Status == InstScheduled }

func (i *Instalment) Paid() bool { return i.Status == InstPaid || i.Status == InstPaidOut }

func (i *Instalment) Late(now time.Time) bool { return i.Open() && DaysLate(i.DueAt, now) > 0 }

func (i *Instalment) Done(step string) bool { return strings.Contains(","+i.Steps+",", ","+step+",") }

func (i *Instalment) Mark(step string) {
	if !i.Done(step) {
		i.Steps = strings.Trim(i.Steps+","+step, ",")
	}
}

func (l *Loan) MoraOn(i *Instalment, now time.Time) int64 {
	if !i.Open() || i.MoraWaived {
		return 0
	}
	return Mora(i.Amount, i.DueAt, now, l.MoraBP, l.Grace, l.MoraMax)
}

const (
	AgreementProposed   = "proposed"
	AgreementAccepted   = "accepted"
	AgreementSuperseded = "superseded"
	AgreementDeclined   = "declined"
	AgreementExpired    = "expired"
)

type LoanAgreement struct {
	ID         string
	LoanID     string
	MerchantID string
	Version    int
	Status     string
	Text       string
	Total      int64
	Dates      []time.Time
	Amounts    []int64
	MoraWaived int64
	SentAt     time.Time
	AcceptedAt *time.Time
	Phone      string
	IP         string
	CreatedBy  string
}

// SMS codes: what the customer types to confirm their number or accept an agreement.
const (
	CodeVerifyPhone = "phone"
	CodeAgreement   = "agreement"
	CodeMaxTries    = 5
	CodeTTL         = 15 * time.Minute
)

type CreditCode struct {
	ID        string
	Subject   string
	Purpose   string
	Phone     string
	Hash      string
	Tries     int
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type CreditStep struct {
	Key  string `json:"key"`
	On   bool   `json:"on"`
	Days int    `json:"days"`
}

const (
	StepRemind   = "remind"
	StepProposal = "proposal"

	FinalManual   = "manual"
	FinalContinue = "continue"
)

type CreditSettings struct {
	Steps         []CreditStep `json:"steps"`
	Grace         int          `json:"grace"`
	Final         string       `json:"final"`
	ViaRiha       bool         `json:"via_riha"`
	Outside       bool         `json:"outside"`
	DeliveryLimit int64        `json:"delivery_limit"`
	TwoApprovers  bool         `json:"two_approvers"`
	TwoAbove      int64        `json:"two_above"`
	SenderID      string       `json:"sender_id"`
	QuietStart    int          `json:"quiet_start"`
	QuietEnd      int          `json:"quiet_end"`
	NoSundays     bool         `json:"no_sundays"`
}

func DefaultCreditSettings() CreditSettings {
	return CreditSettings{
		Steps: []CreditStep{{StepRemind, true, 3}, {StepLink, true, 0}, {StepEvening, true, 0}, {StepResend1, true, 2}, {StepResend2, true, 7},
			{StepProposal, true, 14}, {StepFinal, true, 30}},
		Grace: 2, Final: FinalManual, ViaRiha: true, Outside: true, DeliveryLimit: 5000000, TwoAbove: 3000000,
		QuietStart: 8, QuietEnd: 20, NoSundays: true,
	}
}

func (s *CreditSettings) Step(key string) CreditStep {
	for _, st := range s.Steps {
		if st.Key == key {
			return st
		}
	}
	for _, st := range DefaultCreditSettings().Steps {
		if st.Key == key {
			return st
		}
	}
	return CreditStep{Key: key}
}

func (s *CreditSettings) Clean() *Error {
	def := DefaultCreditSettings()
	steps := make([]CreditStep, len(def.Steps))
	for i, d := range def.Steps {
		st := d
		for _, in := range s.Steps {
			if in.Key == d.Key {
				st.On, st.Days = in.On, in.Days
			}
		}
		if d.Key == StepLink || d.Key == StepFinal {
			st.On = true
		}
		if d.Key == StepLink || d.Key == StepEvening {
			st.Days = 0
		}
		steps[i] = st
	}
	s.Steps = steps
	r := s.Step(StepRemind).Days
	r1, r2, pr, fi := s.Step(StepResend1).Days, s.Step(StepResend2).Days, s.Step(StepProposal).Days, s.Step(StepFinal).Days
	switch {
	case r < 1 || r > 10:
		return Invalid("parameter_invalid", "steps", "O lembrete vai de 1 a 10 dias antes.")
	case !(1 <= r1 && r1 < r2 && r2 < pr && pr < fi && fi <= 90):
		return Invalid("parameter_invalid", "steps", "Os passos depois da data têm de seguir a ordem, até 90 dias.")
	case s.Grace < 0 || s.Grace > 7:
		return Invalid("parameter_invalid", "grace", "A tolerância vai de 0 a 7 dias.")
	case !s.ViaRiha && !s.Outside:
		return Invalid("parameter_invalid", "delivery", "Deixe pelo menos uma forma de entregar o dinheiro.")
	case s.DeliveryLimit < 10000 || s.DeliveryLimit > 100000000:
		return Invalid("parameter_invalid", "delivery_limit", "O limite por entrega vai de 100 a 1 000 000 MT.")
	case s.TwoAbove < 10000:
		return Invalid("parameter_invalid", "two_above", "O valor para dois aprovadores tem de ser de pelo menos 100 MT.")
	}
	if s.Final != FinalContinue {
		s.Final = FinalManual
	}
	s.SenderID = strings.TrimSpace(s.SenderID)
	if len(s.SenderID) > 11 || strings.ContainsAny(s.SenderID, " <>\"'") {
		return Invalid("parameter_invalid", "sender_id", "O remetente tem até 11 letras, sem espaços.")
	}
	if s.QuietStart < 0 || s.QuietStart > 12 || s.QuietEnd < 13 || s.QuietEnd > 23 {
		s.QuietStart, s.QuietEnd = def.QuietStart, def.QuietEnd
	}
	return nil
}

// SendWindow moves a message to the institution's hours.
func (s *CreditSettings) SendWindow(t time.Time) time.Time {
	l := t.In(Maputo)
	for i := 0; i < 8; i++ {
		start := time.Date(l.Year(), l.Month(), l.Day(), s.QuietStart, 0, 0, 0, Maputo)
		end := time.Date(l.Year(), l.Month(), l.Day(), s.QuietEnd, 0, 0, 0, Maputo)
		if s.NoSundays && l.Weekday() == time.Sunday || !l.Before(end) {
			l = start.AddDate(0, 0, 1)
			continue
		}
		if l.Before(start) {
			return start
		}
		return l
	}
	return l
}

// CreditAgreementText is what the customer accepts with the SMS code.
func CreditAgreementText(merchant string, l *Loan, dates []time.Time, amounts []int64, waived int64) string {
	var total int64
	for _, a := range amounts {
		total += a
	}
	n := len(amounts)
	if n == 0 {
		return ""
	}
	same := true
	for _, a := range amounts {
		if a != amounts[0] {
			same = false
		}
	}
	pay := fmt.Sprintf("%d prestações de %s MT", n, FormatShort(amounts[0]))
	if n == 1 {
		pay = "1 prestação de " + FormatShort(amounts[0]) + " MT"
	} else if !same {
		pay = fmt.Sprintf("%d prestações, num total de %s MT", n, FormatShort(total))
	}
	when := ""
	switch l.Freq {
	case FreqWeekly:
		when = "uma por semana"
	case FreqBiweekly:
		when = "uma a cada 2 semanas"
	default:
		when = fmt.Sprintf("no dia %d de cada mês", dates[0].In(Maputo).Day())
	}
	span := "a " + LongDate(dates[0])
	if n > 1 {
		span = "de " + LongDate(dates[0]) + " a " + LongDate(dates[n-1])
	}
	var b strings.Builder
	if l.Version <= 1 {
		fmt.Fprintf(&b, "Recebo %s MT de %s e aceito pagar %s, %s, %s. Total a pagar: %s MT, dos quais %s MT de juros (%s).",
			FormatShort(l.Amount), merchant, pay, when, span, FormatShort(total), FormatShort(total-l.Amount), RateLabel(l.RateBP, l.Freq))
	} else {
		fmt.Fprintf(&b, "Aceito o novo acordo com %s: %s, %s, %s. Total: %s MT.", merchant, pay, when, span, FormatShort(total))
		if waived > 0 {
			fmt.Fprintf(&b, " Mora de %s MT perdoada.", FormatShort(waived))
		}
		b.WriteString(" Substitui o plano anterior.")
	}
	b.WriteString(" Recebo o link de pagamento por SMS antes de cada data e confirmo cada pagamento com o meu PIN.")
	if l.MoraBP > 0 {
		g := ""
		if l.Grace > 0 {
			g = fmt.Sprintf(" mais de %d %s", l.Grace, map[bool]string{true: "dia", false: "dias"}[l.Grace == 1])
		}
		fmt.Fprintf(&b, " Se atrasar%s, pago mora de %s por semana sobre a prestação em atraso, até %s.", g,
			strings.TrimSuffix(RateLabel(l.MoraBP, FreqWeekly), " por semana"), TermLabel(FreqWeekly, l.MoraMax))
	}
	return b.String()
}

func LoanReference() string { return "RH-MC-" + randFrom("0123456789", 4) + randFrom(upper, 2) }

// AgingBucket groups late money by how late it is.
func AgingBucket(days int) int {
	switch {
	case days <= 7:
		return 0
	case days <= 30:
		return 1
	case days <= 60:
		return 2
	}
	return 3
}

var ErrCodeWrong = Invalid("code_invalid", "code", "Código errado. Confirme o SMS e tente de novo.")

type Disbursement struct {
	Reference string
	Phone     string
	Method    string
	Amount    int64
	Live      bool
	Note      string
}

type DisbursementResult struct {
	ProviderRef string
	Pending     bool
}

func CreditCodeDigits() string { return randFrom("0123456789", 6) }
