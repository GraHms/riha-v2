package app

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type LoanPay struct {
	All      bool   `json:"all"`
	Phone    string `json:"phone"`
	Method   string `json:"method"`
	PayAgain bool   `json:"pay_again"`
	IP       string `json:"-"`
	UA       string `json:"-"`
}

// PayLoan opens a payment for the next instalment, with its mora, or for all
// that is left. The customer confirms it with the PIN.
func (a *App) PayLoan(ctx context.Context, token string, in LoanPay) (*domain.Payment, error) {
	l, m, err := a.creditLoanByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	if e := domain.RequireApproved(m); e != nil {
		return nil, domain.Forbidden("merchant_unavailable", "Esta instituição não pode receber pagamentos agora.")
	}
	if !l.Running() {
		return nil, domain.Invalid("loan_state", "token", "Este crédito não tem prestações por pagar.")
	}
	insts, err := a.Credit.Instalments(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	v := a.loanView(ctx, l, insts, false)
	var lines []*domain.Instalment
	for i := range v.Current {
		if v.Current[i].Open() {
			lines = append(lines, &v.Current[i])
			if !in.All {
				break
			}
		}
	}
	if len(lines) == 0 {
		return nil, domain.Invalid("nothing_due", "token", "Não há nada por pagar.")
	}
	now := a.Now()
	charged := make([]int64, len(lines))
	var total int64
	if in.All && l.SettleOffer > 0 {
		charged = domain.EvenSplit(l.SettleOffer, len(lines))
		total = l.SettleOffer
	} else {
		for i, x := range lines {
			charged[i] = x.Amount + l.MoraOn(x, now)
			total += charged[i]
		}
	}
	phone := domain.FirstNonEmpty(domain.NormalizeMZPhone(in.Phone), l.Phone)
	method := strings.ToLower(in.Method)
	if method == "" {
		method = domain.WalletFor(phone)
	}
	desc := fmt.Sprintf("%s · prestação %d de %d", l.Reference, lines[0].Seq, len(v.Current))
	if len(lines) > 1 {
		desc = l.Reference + " · liquidação"
	}
	ids := make([]string, len(lines))
	for i, x := range lines {
		ids[i] = x.ID
	}
	cs, err := a.CreateSession(ctx, m, l.Livemode, SessionParams{Amount: total, Description: desc, PaymentMethodTypes: []string{domain.MethodMpesa, domain.MethodEmola},
		ClientReferenceID: l.ID, Customer: &Customer{Name: l.Name, Phone: "+258" + phone}, Metadata: map[string]any{"loan": l.ID, "instalments": strings.Join(ids, ",")},
		Origin: domain.Origin{SourceType: domain.SourceMicrocredit, IP: in.IP, UserAgent: in.UA, Channel: "sms", CreatedBy: "microcredit"}})
	if err != nil {
		return nil, err
	}
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		for i, x := range lines {
			c, err := a.Credit.LockInstalment(ctx, x.ID)
			if err != nil {
				return err
			}
			if !c.Open() {
				return domain.Invalid("instalment_state", "token", "Esta prestação já foi paga.")
			}
			if c.SessionID != nil {
				if p := a.LatestPayment(ctx, *c.SessionID); p != nil && p.Status == domain.PaymentProcessing {
					return domain.Conflict("payment_in_progress", "Já existe um pagamento em curso. Confirme no telemóvel ou aguarde.")
				} else if p != nil && p.Status == domain.PaymentUnconfirmed && !in.PayAgain {
					return domain.Conflict("payment_unconfirmed", UnconfirmedNotice)
				}
			}
			c.SessionID, c.Charged, c.UpdatedAt = &cs.ID, charged[i], now
			if err := a.Credit.SaveInstalment(ctx, c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return a.Pay(ctx, cs.ID, PayInput{Method: method, Phone: phone, Name: l.Name, PayerIP: in.IP, PayerUA: in.UA})
}

// settleInstalments marks the instalments a payment covered, inside the
// payment's own transaction.
func (a *App) settleInstalments(ctx context.Context, cs *domain.CheckoutSession, p *domain.Payment) error {
	insts, err := a.Credit.InstalmentsBySession(ctx, cs.ID)
	if err != nil || len(insts) == 0 {
		return err
	}
	now := a.Now()
	l, err := a.Credit.LockLoan(ctx, insts[0].LoanID)
	if err != nil {
		return err
	}
	m, err := a.Merchants.ByID(ctx, l.MerchantID)
	if err != nil {
		return err
	}
	var paid []int
	for _, x := range insts {
		c, err := a.Credit.LockInstalment(ctx, x.ID)
		if err != nil {
			return err
		}
		if !c.Open() || c.SessionID == nil || *c.SessionID != cs.ID {
			continue
		}
		c.Status, c.PaidAt, c.PaymentID, c.Method, c.Failure, c.UpdatedAt = domain.InstPaid, &now, &p.ID, p.Method, "", now
		if c.Charged > 0 && c.Charged < c.Amount {
			c.Amount, c.MoraWaived, c.Note = c.Charged, true, "Liquidação antecipada"
		} else if c.Charged > c.Amount {
			c.MoraPaid = c.Charged - c.Amount
		}
		if err := a.Credit.SaveInstalment(ctx, c); err != nil {
			return err
		}
		paid = append(paid, c.Seq)
	}
	if len(paid) == 0 {
		return nil
	}
	all, err := a.Credit.Instalments(ctx, l.ID)
	if err != nil {
		return err
	}
	left, count := 0, 0
	var leftAmt int64
	for _, x := range all {
		if x.Version != l.Version {
			continue
		}
		count++
		if x.Open() {
			left++
			leftAmt += x.Amount
		}
	}
	if left == 0 {
		return a.closeIfPaid(ctx, m, l)
	}
	what := fmt.Sprintf("Prestação %d de %d paga", paid[0], count)
	return a.creditText(ctx, m, l.Livemode, l.ID, "", l.Phone, "credit_receipt",
		fmt.Sprintf("%s recebeu %s MT. %s. Falta pagar %s MT em %d %s. Ref. %s", m.Name, domain.FormatShort(p.Amount), what,
			domain.FormatShort(leftAmt), left, map[bool]string{true: "prestação", false: "prestações"}[left == 1], p.Reference), true)
}

// CreditTick runs the collection steps of each open instalment: the reminder,
// the link on the day, resends when it is late, the offer of a new agreement,
// and the final step. One message per instalment per run; steps a pause
// skipped are not sent late.
func (a *App) CreditTick(ctx context.Context) {
	now := a.Now()
	due, err := a.Credit.OpenInstalmentsDue(ctx, now.Add(11*24*time.Hour), 500)
	if err != nil {
		return
	}
	loans := map[string]*domain.Loan{}
	merchants := map[string]*domain.Merchant{}
	for i := range due {
		x := due[i]
		l, ok := loans[x.LoanID]
		if !ok {
			l, _ = a.Credit.LoanByID(ctx, x.LoanID)
			loans[x.LoanID] = l
		}
		if l == nil || l.Status != domain.LoanActive || x.Version != l.Version {
			continue
		}
		m, ok := merchants[l.MerchantID]
		if !ok {
			m, _ = a.Merchants.ByID(ctx, l.MerchantID)
			merchants[l.MerchantID] = m
		}
		if m == nil || !m.HasMicrocredit() {
			continue
		}
		if err := a.collectStep(ctx, m, l, x.ID, now); err != nil {
			log.Printf("microcredit: instalment %s: %v", x.ID, err)
		}
	}
	a.SendSoon(ctx)
}

func (a *App) collectStep(ctx context.Context, m *domain.Merchant, l *domain.Loan, id string, now time.Time) error {
	s := a.CreditSettings(ctx, m.ID)
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		x, err := a.Credit.LockInstalment(ctx, id)
		if err != nil || !x.Open() {
			return err
		}
		days := domain.DaysLate(x.DueAt, now)
		hour := now.In(domain.Maputo).Hour()
		type step struct {
			key  string
			due  bool
			body func() string
		}
		total := 0
		if all, err := a.Credit.Instalments(ctx, l.ID); err == nil {
			for _, y := range all {
				if y.Version == l.Version {
					total++
				}
			}
		}
		mora := l.MoraOn(x, now)
		late := func() string {
			return fmt.Sprintf("%s: a prestação %d está em atraso há %d %s. Pague %s MT%s em %s", m.Name, x.Seq, days,
				map[bool]string{true: "dia", false: "dias"}[days == 1], domain.FormatShort(x.Amount+mora),
				map[bool]string{true: fmt.Sprintf(" (inclui %s MT de mora)", domain.FormatShort(mora)), false: ""}[mora > 0], a.creditURL(l.Token))
		}
		steps := []step{
			{domain.StepRemind, days >= -s.Step(domain.StepRemind).Days && days < 0, func() string {
				return fmt.Sprintf("Lembrete %s: a %s paga a prestação %d de %d, %s MT. Nesse dia enviamos o link para pagar com o PIN.",
					m.Name, domain.ShortDate(x.DueAt), x.Seq, total, domain.FormatShort(x.Amount))
			}},
			{domain.StepLink, days >= 0, func() string { return a.linkText(m, l, x, mora, total) }},
			{domain.StepEvening, days == 0 && hour >= 18 && x.OpenedAt == nil, func() string {
				return fmt.Sprintf("%s: a prestação de %s MT vence hoje. Pague com o PIN em %s", m.Name, domain.FormatShort(x.Amount), a.creditURL(l.Token))
			}},
			{domain.StepResend1, days >= s.Step(domain.StepResend1).Days, late},
			{domain.StepResend2, days >= s.Step(domain.StepResend2).Days, late},
			{domain.StepProposal, days >= s.Step(domain.StepProposal).Days, func() string {
				return fmt.Sprintf("%s: se não consegue pagar a prestação em atraso, peça um novo acordo em %s", m.Name, a.creditURL(l.Token))
			}},
			{domain.StepFinal, days >= s.Step(domain.StepFinal).Days, nil},
		}
		last := -1
		for i, st := range steps {
			if st.due && !x.Done(st.key) {
				last = i
			}
		}
		if last < 0 {
			return nil
		}
		for i := 0; i < last; i++ {
			if steps[i].due {
				x.Mark(steps[i].key)
			}
		}
		st := steps[last]
		x.Mark(st.key)
		x.UpdatedAt = now
		on := s.Step(st.key).On
		if days >= 0 && st.key != domain.StepFinal {
			if x.LinkSentAt == nil {
				x.LinkSentAt = &now
				on = true
			}
		}
		if err := a.Credit.SaveInstalment(ctx, x); err != nil {
			return err
		}
		if st.key == domain.StepFinal {
			if s.Final == domain.FinalManual && !l.Manual {
				cur, err := a.Credit.LockLoan(ctx, l.ID)
				if err != nil {
					return err
				}
				cur.Manual, cur.UpdatedAt = true, now
				l.Manual = true
				return a.Credit.SaveLoan(ctx, cur)
			}
			return nil
		}
		if !on || st.body == nil {
			return nil
		}
		return a.creditText(ctx, m, l.Livemode, l.ID, x.ID, l.Phone, "credit_"+st.key, st.body(), false)
	})
}

// CreditView is what the customer's page shows, from the request to the last instalment.
type CreditView struct {
	Merchant *domain.Merchant
	Request  *domain.CreditRequest
	Product  *domain.CreditProduct
	Loan     *LoanView
	Payment  *domain.Payment
	Settings domain.CreditSettings
}

func (a *App) CreditPage(ctx context.Context, token string, opened bool) (*CreditView, error) {
	v := &CreditView{}
	if l, m, err := a.creditLoanByToken(ctx, token); err == nil {
		insts, _ := a.Credit.Instalments(ctx, l.ID)
		v.Merchant, v.Loan = m, a.loanView(ctx, l, insts, true)
		v.Product, _ = a.Credit.Product(ctx, m.ID, l.ProductID)
		if l.RequestID != "" {
			v.Request, _ = a.Credit.Request(ctx, m.ID, l.RequestID)
		}
		for i := range v.Loan.Current {
			x := &v.Loan.Current[i]
			if x.SessionID != nil && (x.Open() || x.PaidAt != nil && a.Now().Sub(*x.PaidAt) < 10*time.Minute) {
				if p := a.LatestPayment(ctx, *x.SessionID); p != nil && (v.Payment == nil || p.CreatedAt.After(v.Payment.CreatedAt)) {
					v.Payment = p
				}
			}
		}
		if opened && v.Loan.Next != nil && v.Loan.Next.OpenedAt == nil && v.Loan.Next.LinkSentAt != nil {
			_ = a.Tx.Do(ctx, func(ctx context.Context) error {
				x, err := a.Credit.LockInstalment(ctx, v.Loan.Next.ID)
				if err != nil || x.OpenedAt != nil {
					return err
				}
				now := a.Now()
				x.OpenedAt = &now
				return a.Credit.SaveInstalment(ctx, x)
			})
		}
	} else {
		r, m, err := a.creditRequestByToken(ctx, token)
		if err != nil {
			return nil, err
		}
		v.Merchant, v.Request = m, r
		v.Product, _ = a.Credit.Product(ctx, m.ID, r.ProductID)
	}
	v.Settings = a.CreditSettings(ctx, v.Merchant.ID)
	return v, nil
}

// CreditOverview is the institution's summary.
type CreditOverview struct {
	Book, BookLoans, BookProducts int64
	Pending                       int64
	OldestWait                    *time.Time
	OldestName                    string
	Overdue                       int64
	OverdueLoans                  int64
	ReceivedRiha                  int64
	Month                         struct {
		Label                                 string
		Expected, Riha, Outside, Late, ToCome int64
		RihaN, OutsideN, LateN, ToComeN       int
	}
	Upcoming  []UpcomingGroup
	Attention []CreditAttention
	Aging     [4]int64
	PAR30     float64
}

type UpcomingGroup struct {
	Date    time.Time
	Product string
	Links   int
	Total   int64
}

type CreditAttention struct {
	Kind, Title, Detail, Tone, LoanID, RequestID, Action string
}

func (a *App) CreditOverview(ctx context.Context, m *domain.Merchant, live bool) (*CreditOverview, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	now := a.Now()
	o := &CreditOverview{}
	views, err := a.allLoanViews(ctx, m, live, "", "")
	if err != nil {
		return nil, err
	}
	products, _ := a.Credit.Products(ctx, m.ID, live)
	pname := map[string]string{}
	pfreq := map[string]string{}
	for _, p := range products {
		pname[p.ID], pfreq[p.ID] = p.Name, p.Freq
	}
	lm := now.In(domain.Maputo)
	monthStart := time.Date(lm.Year(), lm.Month(), 1, 0, 0, 0, 0, domain.Maputo)
	monthEnd := monthStart.AddDate(0, 1, 0)
	o.Month.Label = domain.MonthName(lm.Month())
	inBook := map[string]bool{}
	groups := map[string]*UpcomingGroup{}
	var atRisk int64
	var late []*LoanView
	for _, v := range views {
		l := v.Loan
		for _, x := range append(append([]domain.Instalment{}, v.Current...), v.Old...) {
			if x.DueAt.Before(monthStart) && !(x.PaidAt != nil && !x.PaidAt.Before(monthStart)) || !x.DueAt.Before(monthEnd) && x.PaidAt == nil {
				continue
			}
			switch {
			case x.Status == domain.InstPaid:
				o.Month.Riha += x.Amount + x.MoraPaid
				o.Month.RihaN++
			case x.Status == domain.InstPaidOut:
				o.Month.Outside += x.Amount + x.MoraPaid
				o.Month.OutsideN++
			case x.Open() && x.Version == l.Version && l.Status == domain.LoanActive:
				if domain.DaysLate(x.DueAt, now) > 0 {
					o.Month.Late += x.Amount
					o.Month.LateN++
				} else {
					o.Month.ToCome += x.Amount
					o.Month.ToComeN++
				}
			}
		}
		if l.Status == domain.LoanActive || l.Status == domain.LoanDisbursing {
			o.Book += v.Debt - v.Mora
			o.BookLoans++
			inBook[l.ProductID] = true
			if v.DaysLate > 0 {
				o.Overdue += v.Overdue + v.Mora
				o.OverdueLoans++
				late = append(late, v)
				if v.DaysLate > 30 {
					atRisk += v.Debt - v.Mora
				}
			}
			for _, x := range v.Current {
				if !x.Open() {
					continue
				}
				if d := domain.DaysLate(x.DueAt, now); d > 0 {
					o.Aging[domain.AgingBucket(d)] += x.Amount
				} else if x.DueAt.Before(now.AddDate(0, 0, 31)) {
					day := x.DueAt.In(domain.Maputo)
					k := day.Format("2006-01-02") + l.ProductID
					g := groups[k]
					if g == nil {
						g = &UpcomingGroup{Date: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, domain.Maputo), Product: pname[l.ProductID] + map[bool]string{true: " · semanal", false: ""}[pfreq[l.ProductID] == domain.FreqWeekly]}
						groups[k] = g
					}
					g.Links++
					g.Total += x.Amount
				}
			}
		}
		if l.DeliveryStatus == "failed" && l.AcceptedAt != nil {
			o.Attention = append(o.Attention, CreditAttention{Kind: "delivery", Title: "Entrega falhou · " + l.Name, Tone: "red", LoanID: l.ID, Action: "retry",
				Detail: domain.FormatShort(l.Amount) + " MT para " + domain.FormatPhone(l.Phone) + " · " + l.DeliveryError})
		}
		if l.Status == domain.LoanActive && l.DeliveryStatus == "to_deliver" {
			o.Attention = append(o.Attention, CreditAttention{Kind: "deliver", Title: "Entregar " + domain.FormatShort(l.Amount) + " MT a " + l.Name, Tone: "sun", LoanID: l.ID,
				Detail: "Aceitou o acordo. Marque como entregue depois de entregar fora da RIHA."})
		}
		if l.AskedAt != nil && l.Proposed == 0 && l.Status == domain.LoanActive {
			o.Attention = append(o.Attention, CreditAttention{Kind: "asked", Title: l.Name + " pediu um novo acordo", Tone: "sun", LoanID: l.ID, Detail: "Pedido a " + domain.ShortDate(*l.AskedAt)})
		}
		if l.Manual && l.Status == domain.LoanActive {
			o.Attention = append(o.Attention, CreditAttention{Kind: "manual", Title: l.Name + " · cobrança manual", Tone: "red", LoanID: l.ID, Detail: fmt.Sprintf("%d dias de atraso. Os SMS automáticos pararam.", v.DaysLate)})
		}
	}
	o.BookProducts = int64(len(inBook))
	if o.Book > 0 {
		o.PAR30 = float64(atRisk) / float64(o.Book) * 100
	}
	for _, g := range groups {
		o.Upcoming = append(o.Upcoming, *g)
	}
	sort.Slice(o.Upcoming, func(i, j int) bool {
		if !o.Upcoming[i].Date.Equal(o.Upcoming[j].Date) {
			return o.Upcoming[i].Date.Before(o.Upcoming[j].Date)
		}
		return o.Upcoming[i].Product < o.Upcoming[j].Product
	})
	if len(o.Upcoming) > 6 {
		o.Upcoming = o.Upcoming[:6]
	}
	o.Month.Expected = o.Month.Riha + o.Month.Outside + o.Month.Late + o.Month.ToCome
	paid, _ := a.Credit.PaidInstalments(ctx, m.ID, live, monthStart, monthEnd)
	for _, x := range paid {
		if x.Status == domain.InstPaid {
			o.ReceivedRiha += x.Amount + x.MoraPaid
		}
	}
	reqs, _ := a.Credit.Requests(ctx, portsRequestQuery(m.ID, live, []string{domain.RequestNew, domain.RequestReview}))
	o.Pending = int64(len(reqs))
	if n := len(reqs); n > 0 {
		old := reqs[n-1]
		o.OldestWait, o.OldestName = &old.CreatedAt, old.Name
		o.Attention = append([]CreditAttention{{Kind: "requests", Tone: "sun", RequestID: old.ID,
			Title:  fmt.Sprintf("%d %s por decidir", n, map[bool]string{true: "pedido", false: "pedidos"}[n == 1]),
			Detail: old.Name + " espera desde " + domain.ShortDate(old.CreatedAt) + ", " + old.CreatedAt.In(domain.Maputo).Format("15:04")}}, o.Attention...)
	}
	sort.SliceStable(late, func(i, j int) bool { return late[i].DaysLate > late[j].DaysLate })
	for i, v := range late {
		if i == 3 {
			break
		}
		o.Attention = append(o.Attention, CreditAttention{Kind: "late", Tone: map[bool]string{true: "red", false: "sun"}[v.DaysLate > 30], LoanID: v.Loan.ID,
			Title:  fmt.Sprintf("%s · %d %s de atraso", v.Loan.Name, v.DaysLate, map[bool]string{true: "dia", false: "dias"}[v.DaysLate == 1]),
			Detail: fmt.Sprintf("%s MT com mora", domain.FormatShort(v.Overdue+v.Mora))})
	}
	waiting := 0
	for _, v := range views {
		if v.Loan.Status == domain.LoanOffered && now.Sub(v.Loan.CreatedAt) > 48*time.Hour {
			waiting++
		}
	}
	if waiting > 0 {
		o.Attention = append(o.Attention, CreditAttention{Kind: "unaccepted", Tone: "grey", Action: "resend_offers",
			Title: fmt.Sprintf("%d %s por aceitar há mais de 2 dias", waiting, map[bool]string{true: "acordo", false: "acordos"}[waiting == 1])})
	}
	return o, nil
}

func portsRequestQuery(merchantID string, live bool, statuses []string) ports.CreditRequestQuery {
	return ports.CreditRequestQuery{MerchantID: merchantID, Live: live, Statuses: statuses}
}

// ResendOffers reminds the customers who haven't accepted their agreement for
// more than two days.
func (a *App) ResendOffers(ctx context.Context, m *domain.Merchant, live bool, actor Actor) (int, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return 0, e
	}
	loans, err := a.Credit.Loans(ctx, portsLoanQuery(m.ID, live, "", ""))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, l := range loans {
		if l.Status == domain.LoanOffered && a.Now().Sub(l.CreatedAt) > 48*time.Hour {
			if a.ResendLoanLink(ctx, m, live, l.ID, actor) == nil {
				n++
			}
		}
	}
	return n, nil
}
