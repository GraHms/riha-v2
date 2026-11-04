package domain

import (
	"fmt"
	"strings"
	"time"
)

const (
	IntervalWeek    = "week"
	IntervalMonth   = "month"
	IntervalQuarter = "quarter"
	IntervalYear    = "year"

	AnchorFixed  = "fixed"
	AnchorSignup = "signup"

	MissedDebt    = "debt"
	MissedForfeit = "forfeit"

	PlanDraft    = "draft"
	PlanActive   = "active"
	PlanArchived = "archived"

	SubIncomplete = "incomplete"
	SubInvited    = "invited"
	SubActive     = "active"
	SubPastDue    = "past_due"
	SubPaused     = "paused"
	SubSuspended  = "suspended"
	SubCanceled   = "canceled"

	ChargeScheduled   = "scheduled"
	ChargeOpen        = "open"
	ChargePastDue     = "past_due"
	ChargePaid        = "paid"
	ChargePaidOutside = "paid_outside"
	ChargeForfeited   = "forfeited"
	ChargeSkipped     = "skipped"
	ChargeCanceled    = "canceled"

	FinalSuspend = "suspend"
	FinalCancel  = "cancel"
	FinalKeep    = "keep"

	ChannelSMS      = "sms"
	ChannelWhatsApp = "whatsapp"
	ChannelEmail    = "email"

	SourceSubscription = "subscription"

	AllMonths    = 0xFFF
	MaxPlanField = 6
)

var Maputo = time.FixedZone("CAT", 2*3600)

var OutsideMethods = map[string]string{"cash": "Dinheiro", "transfer": "Transferência", "deposit": "Depósito", "other": "Outro"}

var UnpaidChargeStatuses = []string{ChargeOpen, ChargePastDue}
var LiveSubStatuses = []string{SubActive, SubPastDue, SubPaused, SubSuspended}

type Plan struct {
	ID           string
	MerchantID   string
	Livemode     bool
	Slug         string
	Name         string
	Status       string
	Interval     string
	Anchor       string
	BillingDay   int
	Months       int
	Missed       string
	Fields       []string
	EndsAt       *time.Time
	Amount       int64
	Version      int
	InCatalog    bool
	CatalogOrder int
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PlanVersion struct {
	ID        string
	PlanID    string
	Version   int
	Amount    int64
	Scope     string
	CreatedBy string
	CreatedAt time.Time
}

type Member struct {
	ID          string
	MerchantID  string
	Livemode    bool
	Name        string
	NameKey     string
	PayerName   string
	Phone       string
	Email       string
	Channel     string
	Fields      map[string]string
	Status      string
	Debt        int64
	DebtCount   int
	NextDueAt   *time.Time
	LastPaidAt  *time.Time
	LastPaidVia string
	PlanNames   string
	LastMethod  string
	LastPhone   string
	Token       string
	CreatedVia  string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NameKey(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

type Subscription struct {
	ID             string
	MerchantID     string
	Livemode       bool
	MemberID       string
	PlanID         string
	PlanVersion    int
	Amount         int64
	PendingAmount  int64
	PendingVersion int
	PendingFrom    *time.Time
	Status         string
	AnchorAt       time.Time
	NextDueAt      *time.Time
	PausedUntil    *time.Time
	SkipPeriods    []string
	ConsentText    string
	ConsentAt      *time.Time
	ConsentIP      string
	CreatedVia     string
	CreatedBy      string
	ActivatedAt    *time.Time
	CanceledAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

var statusRank = map[string]int{SubPastDue: 0, SubSuspended: 1, SubActive: 2, SubPaused: 3, SubInvited: 4, SubIncomplete: 5, SubCanceled: 6}

func MemberStatus(subs []Subscription) string {
	best := SubCanceled
	for _, s := range subs {
		if statusRank[s.Status] < statusRank[best] {
			best = s.Status
		}
	}
	return best
}

type Charge struct {
	ID               string
	MerchantID       string
	Livemode         bool
	SubscriptionID   string
	MemberID         string
	PlanID           string
	Period           string
	Label            string
	DueAt            time.Time
	Amount           int64
	Status           string
	First            bool
	Token            string
	SessionID        *string
	PaymentID        *string
	PaidAt           *time.Time
	Method           string
	OutsideMethod    string
	OutsideRef       string
	OutsideProofPath string
	OutsideProofName string
	OutsideProofHash string
	MarkedBy         string
	Steps            []string
	Resends          int
	LastFailure      string
	LinkSentAt       *time.Time
	OpenedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (c *Charge) Unpaid() bool { return c.Status == ChargeOpen || c.Status == ChargePastDue }
func (c *Charge) Settled() bool {
	return c.Status == ChargePaid || c.Status == ChargePaidOutside
}
func (c *Charge) Done(step string) bool { return contains(c.Steps, step) }

type Message struct {
	ID         string
	MerchantID string
	Livemode   bool
	MemberID   string
	ChargeID   string
	Channel    string
	To         string
	Sender     string
	Kind       string
	Body       string
	Status     string
	Error      string
	CreatedAt  time.Time
}

type SubscriptionSettings struct {
	MerchantID    string
	DisabledSteps []string
	Channels      []string
	SenderID      string
	ToleranceDays int
	FinalAction   string
	RequireProof  bool
	ReceiptSMS    bool
	CatalogMulti  bool
	UpdatedAt     time.Time
}

func DefaultSubscriptionSettings(m *Merchant) SubscriptionSettings {
	return SubscriptionSettings{MerchantID: m.ID, Channels: []string{ChannelSMS, ChannelWhatsApp}, SenderID: SenderID(m.Name),
		ToleranceDays: 10, FinalAction: FinalSuspend, ReceiptSMS: true, CatalogMulti: true}
}

func SenderID(name string) string {
	var b strings.Builder
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		if len(r) == 0 {
			continue
		}
		if b.Len() == 0 {
			b.WriteString(string(r))
		} else {
			b.WriteString(strings.ToUpper(string(r[:1])) + string(r[1:]))
		}
	}
	out := []rune(b.String())
	if len(out) > 11 {
		out = out[:11]
	}
	return string(out)
}

type ReminderStep struct {
	Key    string `json:"key"`
	Days   int    `json:"days"`
	Hour   int    `json:"hour"`
	Title  string `json:"title"`
	Hint   string `json:"hint"`
	Locked bool   `json:"locked"`
}

const (
	StepBefore  = "before"
	StepLink    = "link"
	StepEvening = "evening"
	StepResend1 = "resend1"
	StepResend2 = "resend2"
	StepLast    = "last"
	StepFinal   = "final"

	PastDueAfterDays = 2
)

func ReminderSteps(tolerance int) []ReminderStep {
	return []ReminderStep{
		{StepBefore, -3, 9, "Lembrete", "Valor e data, para ter saldo", false},
		{StepLink, 0, 9, "Link de pagamento", "Sempre enviado", true},
		{StepEvening, 0, 18, "Lembrete", "Só se não abriu o link", false},
		{StepResend1, PastDueAfterDays, 9, "Reenvio", "Passa a Em dívida", false},
		{StepResend2, 5, 9, "Reenvio", "Segundo canal. Recebe um resumo.", false},
		{StepLast, tolerance - 3, 9, "Último aviso", "Diz o que vai acontecer", false},
		{StepFinal, tolerance, 9, "Ação final", "Fim da tolerância", true},
	}
}

func (s *SubscriptionSettings) StepOn(key string) bool {
	return key == StepLink || key == StepFinal || !contains(s.DisabledSteps, key)
}

func (s *SubscriptionSettings) Validate() error {
	if s.ToleranceDays < 6 || s.ToleranceDays > 30 {
		return Invalid("parameter_invalid", "tolerance_days", "A tolerância tem de estar entre 6 e 30 dias.")
	}
	if s.FinalAction != FinalSuspend && s.FinalAction != FinalCancel && s.FinalAction != FinalKeep {
		return Invalid("parameter_invalid", "final_action", "Ação final inválida.")
	}
	var ch []string
	for _, c := range s.Channels {
		if (c == ChannelSMS || c == ChannelWhatsApp || c == ChannelEmail) && !contains(ch, c) {
			ch = append(ch, c)
		}
	}
	if len(ch) == 0 {
		return Invalid("parameter_invalid", "channels", "Escolha pelo menos um canal.")
	}
	s.Channels = ch
	s.SenderID = strings.TrimSpace(s.SenderID)
	if l := len([]rune(s.SenderID)); l < 3 || l > 11 {
		return Invalid("parameter_invalid", "sender_id", "O remetente do SMS tem de ter entre 3 e 11 caracteres.")
	}
	for _, r := range s.SenderID {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return Invalid("parameter_invalid", "sender_id", "O remetente só pode ter letras e números, sem espaços.")
		}
	}
	return nil
}

func StepAt(due time.Time, st ReminderStep) time.Time {
	d := due.In(Maputo)
	t := time.Date(d.Year(), d.Month(), d.Day()+st.Days, st.Hour, 0, 0, 0, Maputo)
	for t.Weekday() == time.Sunday {
		t = time.Date(t.Year(), t.Month(), t.Day()+1, 9, 0, 0, 0, Maputo)
	}
	return t.UTC()
}

func QuietHours(now time.Time) bool {
	l := now.In(Maputo)
	return l.Weekday() == time.Sunday || l.Hour() < 8 || l.Hour() >= 20
}

func (p *Plan) MonthOn(m time.Month) bool { return p.Months&(1<<(int(m)-1)) != 0 }

func (p *Plan) Validate() error {
	p.Name = Trim(strings.TrimSpace(p.Name), 120)
	if len([]rune(p.Name)) < 2 {
		return Invalid("parameter_missing", "name", "Indique o nome do plano.")
	}
	if p.Amount < MinAmount || p.Amount > MaxAmount {
		return Invalid("amount_invalid", "amount", fmt.Sprintf("O valor tem de estar entre %s e %s MT.", Format(MinAmount), Format(MaxAmount)))
	}
	switch p.Interval {
	case IntervalWeek, IntervalMonth, IntervalQuarter, IntervalYear:
	default:
		return Invalid("parameter_invalid", "interval", "Frequência inválida.")
	}
	if p.Anchor != AnchorFixed && p.Anchor != AnchorSignup {
		return Invalid("parameter_invalid", "anchor", "Escolha quando cobrar.")
	}
	if p.Anchor == AnchorFixed {
		if p.Interval == IntervalWeek && (p.BillingDay < 1 || p.BillingDay > 7) {
			return Invalid("parameter_invalid", "billing_day", "Escolha o dia da semana.")
		}
		if p.Interval != IntervalWeek && (p.BillingDay < 1 || p.BillingDay > 28) {
			return Invalid("parameter_invalid", "billing_day", "O dia de cobrança tem de estar entre 1 e 28.")
		}
	} else {
		p.BillingDay = 0
	}
	p.Months &= AllMonths
	if p.Months == 0 {
		return Invalid("parameter_invalid", "months", "Escolha pelo menos um mês com cobrança.")
	}
	if p.Missed != MissedDebt && p.Missed != MissedForfeit {
		return Invalid("parameter_invalid", "missed", "Escolha o que acontece se um mês não for pago.")
	}
	var fields []string
	for _, f := range p.Fields {
		f = Trim(strings.TrimSpace(f), 40)
		if f != "" && !contains(fields, f) {
			fields = append(fields, f)
		}
	}
	if len(fields) > MaxPlanField {
		return Invalid("parameter_invalid", "fields", fmt.Sprintf("No máximo %d campos.", MaxPlanField))
	}
	p.Fields = fields
	return nil
}

func dayIn(y int, m time.Month, d int) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, Maputo).Day()
	if d > last {
		d = last
	}
	return time.Date(y, m, d, 0, 0, 0, 0, Maputo)
}

func monthsPer(interval string) int {
	switch interval {
	case IntervalQuarter:
		return 3
	case IntervalYear:
		return 12
	}
	return 1
}

func LocalDate(t time.Time) time.Time {
	l := t.In(Maputo)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, Maputo)
}

func dueAt(p *Plan, anchor time.Time, k int) time.Time {
	a := anchor.In(Maputo)
	if p.Interval == IntervalWeek {
		return time.Date(a.Year(), a.Month(), a.Day()+7*k, 0, 0, 0, 0, Maputo)
	}
	return dayIn(a.Year(), a.Month()+time.Month(k*monthsPer(p.Interval)), a.Day())
}

func (p *Plan) inRange(d time.Time) bool {
	return p.MonthOn(d.In(Maputo).Month()) && (p.EndsAt == nil || !d.After(*p.EndsAt))
}

func FirstDue(p *Plan, now time.Time) (time.Time, bool) {
	today := LocalDate(now)
	var start time.Time
	switch {
	case p.Anchor == AnchorSignup:
		start = today
	case p.Interval == IntervalWeek:
		wd := int(today.Weekday())
		if wd == 0 {
			wd = 7
		}
		start = today.AddDate(0, 0, -((wd - p.BillingDay + 7) % 7))
	default:
		start = dayIn(today.Year(), today.Month(), p.BillingDay)
	}
	for k := 0; k < 120; k++ {
		d := dueAt(p, start, k)
		if p.EndsAt != nil && d.After(*p.EndsAt) {
			return time.Time{}, false
		}
		if p.inRange(d) {
			if p.Anchor == AnchorSignup && k > 0 {
				return d, true
			}
			return d, true
		}
	}
	return time.Time{}, false
}

func NextDue(p *Plan, anchor, after time.Time) (time.Time, bool) {
	for k := 0; k < 600; k++ {
		d := dueAt(p, anchor, k)
		if p.EndsAt != nil && d.After(*p.EndsAt) {
			return time.Time{}, false
		}
		if d.After(after) && p.inRange(d) {
			return d.UTC(), true
		}
	}
	return time.Time{}, false
}

func PeriodKey(due time.Time) string { return due.In(Maputo).Format("2006-01-02") }

var monthNames = []string{"Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho", "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"}
var monthShort = []string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}
var weekdays = []string{"domingo", "segunda", "terça", "quarta", "quinta", "sexta", "sábado"}

func MonthName(m time.Month) string { return monthNames[m-1] }

func ShortDate(t time.Time) string {
	l := t.In(Maputo)
	return fmt.Sprintf("%d %s", l.Day(), monthShort[l.Month()-1])
}

func LongDate(t time.Time) string {
	l := t.In(Maputo)
	return fmt.Sprintf("%d de %s de %d", l.Day(), strings.ToLower(monthNames[l.Month()-1]), l.Year())
}

func PeriodLabel(p *Plan, due time.Time) string {
	d := due.In(Maputo)
	switch p.Interval {
	case IntervalWeek:
		return "Semana de " + ShortDate(d)
	case IntervalQuarter:
		e := d.AddDate(0, 2, 0)
		return fmt.Sprintf("%s %d a %s %d", monthShort[d.Month()-1], d.Year(), monthShort[e.Month()-1], e.Year())
	case IntervalYear:
		return fmt.Sprintf("Ano de %s %d", monthShort[d.Month()-1], d.Year())
	}
	return fmt.Sprintf("%s %d", monthNames[d.Month()-1], d.Year())
}

func MonthsLabel(mask int) string {
	mask &= AllMonths
	if mask == AllMonths {
		return "todos os meses"
	}
	var parts []string
	for i := 0; i < 12; {
		if mask&(1<<i) == 0 {
			i++
			continue
		}
		j := i
		for j+1 < 12 && mask&(1<<(j+1)) != 0 {
			j++
		}
		switch {
		case j == i:
			parts = append(parts, monthShort[i])
		case j == i+1:
			parts = append(parts, monthShort[i], monthShort[j])
		default:
			parts = append(parts, monthShort[i]+" a "+monthShort[j])
		}
		i = j + 1
	}
	return strings.Join(parts, ", ")
}

func IntervalLabel(i string) string {
	switch i {
	case IntervalWeek:
		return "semana"
	case IntervalQuarter:
		return "trimestre"
	case IntervalYear:
		return "ano"
	}
	return "mês"
}

func (p *Plan) WhenLabel() string {
	if p.Anchor == AnchorSignup {
		return "Aniversário"
	}
	if p.Interval == IntervalWeek {
		return "À " + weekdays[p.BillingDay%7]
	}
	return fmt.Sprintf("Dia %d", p.BillingDay)
}

func (p *Plan) ScheduleLabel() string { return p.WhenLabel() + " · " + MonthsLabel(p.Months) }

func MonthlyEquivalent(amount int64, interval string) int64 {
	switch interval {
	case IntervalWeek:
		return amount * 52 / 12
	case IntervalQuarter:
		return amount / 3
	case IntervalYear:
		return amount / 12
	}
	return amount
}

func Agreement(merchant string, p *Plan, amount int64, phone, channel string) string {
	when := ""
	switch {
	case p.Anchor == AnchorSignup && p.Interval == IntervalWeek:
		when = "no mesmo dia de cada semana"
	case p.Anchor == AnchorSignup:
		when = "no dia em que subscrevo, em cada " + IntervalLabel(p.Interval)
	case p.Interval == IntervalWeek:
		when = "à " + weekdays[p.BillingDay%7] + " de cada semana"
	case p.Interval == IntervalMonth:
		when = fmt.Sprintf("no dia %d de cada mês", p.BillingDay)
	default:
		when = fmt.Sprintf("no dia %d, em cada %s", p.BillingDay, IntervalLabel(p.Interval))
	}
	months := ""
	if p.Months != AllMonths {
		months = ", nos meses " + MonthsLabel(p.Months)
	}
	end := ""
	if p.EndsAt != nil {
		end = ", até " + LongDate(*p.EndsAt)
	}
	ch := "SMS"
	if channel == ChannelWhatsApp {
		ch = "WhatsApp"
	}
	return fmt.Sprintf("Aceito pagar %s MT à %s %s%s%s. Recebo o link de pagamento por %s no %s antes de cada data. Nada é cobrado sem a minha confirmação.",
		FormatShort(amount), merchant, when, months, end, ch, FormatPhone(phone))
}

func FormatShort(c int64) string {
	s := Format(c)
	return strings.TrimSuffix(s, ",00")
}

func FormatPhone(p string) string {
	d := NormalizeMZPhone(p)
	if len(d) != 9 {
		return p
	}
	return d[:2] + " " + d[2:5] + " " + d[5:]
}

func MaskPhone(p string) string {
	d := NormalizeMZPhone(p)
	if len(d) != 9 {
		return ""
	}
	return d[:2] + " ••• " + d[5:]
}

func ValidMZMobile(p string) bool {
	d := NormalizeMZPhone(p)
	return len(d) == 9 && d[0] == '8' && strings.ContainsRune("234567", rune(d[1]))
}

func WalletFor(phone string) string {
	d := NormalizeMZPhone(phone)
	if len(d) == 9 && (d[1] == '6' || d[1] == '7') {
		return MethodEmola
	}
	return MethodMpesa
}
