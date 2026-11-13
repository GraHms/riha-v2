package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

type OfferInput struct {
	ProductID string `json:"product_id"`
	Phone     string `json:"phone"`
	Name      string `json:"name"`
	Amount    int64  `json:"amount"`
	Term      int    `json:"term"`
	Day       int    `json:"day"`
	Delivery  string `json:"delivery"`
}

func (a *App) checkDelivery(ctx context.Context, m *domain.Merchant, live bool, delivery string, amount int64) *domain.Error {
	s := a.CreditSettings(ctx, m.ID)
	switch delivery {
	case domain.DeliverRiha:
		if !s.ViaRiha {
			return domain.Invalid("delivery_off", "delivery", "A entrega pelo saldo RIHA está desligada nas definições.")
		}
		if !a.CanDisburse(live) {
			return domain.Invalid("delivery_unavailable", "delivery", "A entrega para a carteira ainda não está disponível. Entregue fora da RIHA.")
		}
		if amount > s.DeliveryLimit {
			return domain.Invalid("delivery_limit", "amount", "Acima do limite por entrega de "+domain.FormatShort(s.DeliveryLimit)+" MT. Entregue fora da RIHA.")
		}
	case domain.DeliverOutside:
		if !s.Outside {
			return domain.Invalid("delivery_off", "delivery", "A entrega fora da RIHA está desligada nas definições.")
		}
	default:
		return domain.Invalid("parameter_invalid", "delivery", "Escolha como entrega o dinheiro.")
	}
	return nil
}

func (a *App) needsSecond(ctx context.Context, m *domain.Merchant, amount int64) bool {
	s := a.CreditSettings(ctx, m.ID)
	return s.TwoApprovers && amount > s.TwoAbove
}

func newLoan(p *domain.CreditProduct, m *domain.Merchant, live bool, name, phone string, amount int64, term, day int, delivery, by string, now time.Time) *domain.Loan {
	l := &domain.Loan{ID: domain.NewID("loan"), MerchantID: m.ID, Livemode: live, ProductID: p.ID, Token: domain.Token("mc_", 32), Reference: domain.LoanReference(),
		Name: name, Phone: phone, Wallet: domain.WalletFor(phone), Amount: amount, Term: term, Day: day, Freq: p.Freq, RateBP: p.RateBP,
		Grace: p.Grace, Status: domain.LoanOffered, Delivery: delivery, DeliveryStatus: "pending", Version: 0, Proposed: 1,
		ApprovedBy: by, CreatedBy: by, CreatedAt: now, UpdatedAt: now}
	if p.Mora {
		l.MoraBP, l.MoraMax = p.MoraBP, p.MoraMaxWeeks
	}
	if p.Freq != domain.FreqMonthly {
		l.Day = 0
	}
	return l
}

// offer saves the first agreement and, unless a second person must approve,
// sends it to the customer.
func (a *App) offer(ctx context.Context, m *domain.Merchant, l *domain.Loan, second bool) error {
	if second {
		l.Status = domain.LoanApproving
	}
	if err := a.Credit.CreateLoan(ctx, l); err != nil {
		return err
	}
	if second {
		return nil
	}
	return a.proposeFirst(ctx, m, l)
}

func (a *App) proposeFirst(ctx context.Context, m *domain.Merchant, l *domain.Loan) error {
	inst := domain.CreditInstalment(l.Amount, l.Term, l.RateBP)
	dates := domain.DueDates(l.Freq, l.Day, l.Term, a.Now())
	amounts := make([]int64, l.Term)
	for i := range amounts {
		amounts[i] = inst
	}
	g := &domain.LoanAgreement{ID: domain.NewID("lagr"), LoanID: l.ID, MerchantID: l.MerchantID, Version: 1, Status: domain.AgreementProposed,
		Total: inst * int64(l.Term), Dates: dates, Amounts: amounts, SentAt: a.Now(), CreatedBy: l.ApprovedBy}
	cp := *l
	cp.Version = 1
	g.Text = domain.CreditAgreementText(m.Name, &cp, dates, amounts, 0)
	if err := a.Credit.CreateAgreement(ctx, g); err != nil {
		return err
	}
	return a.creditText(ctx, m, l.Livemode, l.ID, "", l.Phone, "credit_offer",
		fmt.Sprintf("%s aprovou %s MT para si: %d prestações de %s MT. Leia e aceite o acordo com um código SMS em %s",
			m.Name, domain.FormatShort(l.Amount), l.Term, domain.FormatShort(inst), a.creditURL(l.Token)), true)
}

func (a *App) ApproveCreditRequest(ctx context.Context, m *domain.Merchant, live bool, id string, in OfferInput, by *domain.User, actor Actor) (*domain.Loan, error) {
	var out *domain.Loan
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		r, err := a.lockRequest(ctx, m, live, id)
		if err != nil {
			return err
		}
		if !r.Open() {
			return domain.Invalid("request_decided", "id", "Este pedido já foi decidido.")
		}
		p, err := a.Credit.Product(ctx, m.ID, r.ProductID)
		if err != nil {
			return domain.NotFound("O produto")
		}
		if p.Freq != domain.FreqMonthly {
			in.Day = 1
		}
		if e := p.CheckTerms(in.Amount, in.Term, in.Day); e != nil {
			return e
		}
		if e := a.checkDelivery(ctx, m, live, in.Delivery, in.Amount); e != nil {
			return e
		}
		now := a.Now()
		l := newLoan(p, m, live, r.Name, r.Phone, in.Amount, in.Term, in.Day, in.Delivery, by.ID, now)
		l.RequestID, l.Token = r.ID, r.Token
		if err := a.offer(ctx, m, l, a.needsSecond(ctx, m, in.Amount)); err != nil {
			return err
		}
		r.Status, r.LoanID, r.DecidedBy, r.DecidedAt, r.UpdatedAt = domain.RequestOffered, l.ID, by.ID, &now, now
		out = l
		return a.Credit.SaveRequest(ctx, r)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.request_approved", out.ID, domain.FormatShort(out.Amount)+" MT")
	a.SendSoon(ctx)
	return out, nil
}

// CreateLoan is a loan for a customer who already spoke with the institution:
// they get the agreement by SMS.
func (a *App) CreateLoan(ctx context.Context, m *domain.Merchant, live bool, in OfferInput, by *domain.User, actor Actor) (*domain.Loan, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	p, err := a.CreditProduct(ctx, m, live, in.ProductID)
	if err != nil {
		return nil, err
	}
	if p.Status != domain.ProductActive {
		return nil, domain.Invalid("product_draft", "product_id", "Este produto é um rascunho. Ative-o primeiro.")
	}
	name, phone := clean(in.Name, 120), domain.NormalizeMZPhone(in.Phone)
	if !domain.ValidMZMobile(phone) {
		return nil, domain.Invalid("parameter_invalid", "phone", "Use um número de telemóvel moçambicano, de 9 algarismos.")
	}
	if len([]rune(name)) < 3 {
		return nil, domain.Invalid("parameter_invalid", "name", "Escreva o nome do cliente.")
	}
	if p.Freq != domain.FreqMonthly {
		in.Day = 1
	}
	if e := p.CheckTerms(in.Amount, in.Term, in.Day); e != nil {
		return nil, e
	}
	if e := a.checkDelivery(ctx, m, live, in.Delivery, in.Amount); e != nil {
		return nil, e
	}
	l := newLoan(p, m, live, name, phone, in.Amount, in.Term, in.Day, in.Delivery, by.ID, a.Now())
	if err := a.Tx.Do(ctx, func(ctx context.Context) error { return a.offer(ctx, m, l, a.needsSecond(ctx, m, in.Amount)) }); err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.loan_created", l.ID, domain.FormatShort(l.Amount)+" MT para "+domain.FormatPhone(phone))
	a.SendSoon(ctx)
	return l, nil
}

func (a *App) lockLoan(ctx context.Context, m *domain.Merchant, live bool, id string) (*domain.Loan, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	l, err := a.Credit.LockLoan(ctx, id)
	if err != nil || l.MerchantID != m.ID || l.Livemode != live {
		return nil, domain.NotFound("O empréstimo")
	}
	return l, nil
}

// ApproveLoanSecond is the second person's approval, above the amount the
// institution set.
func (a *App) ApproveLoanSecond(ctx context.Context, m *domain.Merchant, live bool, id string, by *domain.User, actor Actor) (*domain.Loan, error) {
	var out *domain.Loan
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanApproving {
			return domain.Invalid("loan_state", "id", "Este empréstimo não espera aprovação.")
		}
		if l.ApprovedBy == by.ID {
			return domain.Forbidden("same_approver", "A segunda aprovação tem de ser de outra pessoa.")
		}
		l.Status, l.SecondBy, l.UpdatedAt = domain.LoanOffered, by.ID, a.Now()
		if err := a.Credit.SaveLoan(ctx, l); err != nil {
			return err
		}
		out = l
		return a.proposeFirst(ctx, m, l)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.loan_second_approval", id, "")
	a.SendSoon(ctx)
	return out, nil
}

func (a *App) CancelLoanOffer(ctx context.Context, m *domain.Merchant, live bool, id string, actor Actor) error {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanOffered && l.Status != domain.LoanApproving {
			return domain.Invalid("loan_state", "id", "Só um acordo por aceitar pode ser cancelado.")
		}
		now := a.Now()
		l.Status, l.ClosedAt, l.Proposed, l.UpdatedAt = domain.LoanCanceled, &now, 0, now
		if err := a.Credit.SaveLoan(ctx, l); err != nil {
			return err
		}
		if err := a.closeProposals(ctx, l.ID, domain.AgreementDeclined); err != nil {
			return err
		}
		if l.RequestID != "" {
			if r, err := a.Credit.LockRequest(ctx, l.RequestID); err == nil {
				r.Status, r.UpdatedAt = domain.RequestCanceled, now
				return a.Credit.SaveRequest(ctx, r)
			}
		}
		return nil
	})
	if err == nil {
		a.Record(ctx, m.ID, actor, "microcredit.offer_canceled", id, "")
	}
	return err
}

func (a *App) closeProposals(ctx context.Context, loanID, status string) error {
	gs, err := a.Credit.Agreements(ctx, loanID)
	if err != nil {
		return err
	}
	for i := range gs {
		if gs[i].Status == domain.AgreementProposed {
			gs[i].Status = status
			if err := a.Credit.SaveAgreement(ctx, &gs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func proposed(gs []domain.LoanAgreement) *domain.LoanAgreement {
	for i := range gs {
		if gs[i].Status == domain.AgreementProposed {
			return &gs[i]
		}
	}
	return nil
}

func (a *App) creditLoanByToken(ctx context.Context, token string) (*domain.Loan, *domain.Merchant, error) {
	l, err := a.Credit.LoanByToken(ctx, token)
	if err != nil {
		return nil, nil, domain.NotFound("Este crédito")
	}
	m, err := a.Merchants.ByID(ctx, l.MerchantID)
	if err != nil || !m.HasMicrocredit() {
		return nil, nil, domain.NotFound("Este crédito")
	}
	return l, m, nil
}

// SendAgreementCode sends the code that accepts the agreement on offer.
func (a *App) SendAgreementCode(ctx context.Context, token string, accept bool) (string, error) {
	l, m, err := a.creditLoanByToken(ctx, token)
	if err != nil {
		return "", err
	}
	if !accept {
		return "", domain.Invalid("agreement_required", "accept", "Leia e aceite o acordo primeiro.")
	}
	gs, _ := a.Credit.Agreements(ctx, l.ID)
	g := proposed(gs)
	if g == nil || (l.Status != domain.LoanOffered && l.Status != domain.LoanActive) {
		return "", domain.Invalid("no_agreement", "accept", "Não há nenhum acordo por aceitar.")
	}
	return a.sendCreditCode(ctx, m, l.Livemode, g.ID, domain.CodeAgreement, l.Phone, "aceitar o acordo de pagamento")
}

// AcceptAgreement: the customer types the code. The first agreement starts the
// loan and sends the money; a later one replaces the plan.
func (a *App) AcceptAgreement(ctx context.Context, token, code, ip string) (*domain.Loan, error) {
	l, m, err := a.creditLoanByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	first, _ := a.Credit.Agreements(ctx, l.ID)
	pg := proposed(first)
	if pg == nil {
		return nil, domain.Invalid("no_agreement", "code", "Não há nenhum acordo por aceitar.")
	}
	if err := a.checkCreditCode(ctx, pg.ID, domain.CodeAgreement, code); err != nil {
		return nil, err
	}
	deliver := false
	err = a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err = a.Credit.LockLoan(ctx, l.ID)
		if err != nil {
			return err
		}
		gs, err := a.Credit.Agreements(ctx, l.ID)
		if err != nil {
			return err
		}
		g := proposed(gs)
		if g == nil || g.ID != pg.ID || (l.Status != domain.LoanOffered && l.Status != domain.LoanActive) {
			return domain.Invalid("no_agreement", "code", "O acordo mudou. Peça um código novo.")
		}
		now := a.Now()
		if g.Version == 1 {
			g.Dates = domain.DueDates(l.Freq, l.Day, l.Term, now)
		} else {
			g.Dates = domain.NextDates(g.Dates[0].In(domain.Maputo).Day(), len(g.Amounts), now)
		}
		cp := *l
		cp.Version = g.Version
		g.Text = domain.CreditAgreementText(m.Name, &cp, g.Dates, g.Amounts, g.MoraWaived)
		g.Status, g.AcceptedAt, g.Phone, g.IP = domain.AgreementAccepted, &now, l.Phone, domain.Trim(ip, 64)
		if err := a.Credit.SaveAgreement(ctx, g); err != nil {
			return err
		}
		for i := range gs {
			if gs[i].Status == domain.AgreementAccepted && gs[i].ID != g.ID {
				gs[i].Status = domain.AgreementSuperseded
				if err := a.Credit.SaveAgreement(ctx, &gs[i]); err != nil {
					return err
				}
			}
		}
		if g.Version > 1 {
			insts, err := a.Credit.Instalments(ctx, l.ID)
			if err != nil {
				return err
			}
			for i := range insts {
				if insts[i].Open() {
					insts[i].Status, insts[i].UpdatedAt = domain.InstReplaced, now
					if err := a.Credit.SaveInstalment(ctx, &insts[i]); err != nil {
						return err
					}
				}
			}
		}
		for i, d := range g.Dates {
			in := &domain.Instalment{ID: domain.NewID("inst"), LoanID: l.ID, MerchantID: l.MerchantID, Livemode: l.Livemode, Version: g.Version,
				Seq: i + 1, DueAt: d, Amount: g.Amounts[i], Status: domain.InstScheduled, UpdatedAt: now}
			if err := a.Credit.CreateInstalment(ctx, in); err != nil {
				return err
			}
		}
		l.Version, l.Proposed, l.AskedAt, l.SettleOffer, l.Manual, l.UpdatedAt = g.Version, 0, nil, 0, false, now
		body := ""
		if g.Version == 1 {
			l.AcceptedAt = &now
			l.Status = domain.LoanActive
			if l.Delivery == domain.DeliverRiha {
				l.Status, l.DeliveryStatus = domain.LoanDisbursing, "sending"
				deliver = true
				body = fmt.Sprintf("Acordo aceite. %s está a enviar %s MT para o seu %s %s. Primeira prestação a %s. Ref. %s",
					m.Name, domain.FormatShort(l.Amount), domain.MethodNames[l.Wallet], domain.FormatPhone(l.Phone), domain.ShortDate(g.Dates[0]), l.Reference)
			} else {
				l.DeliveryStatus = "to_deliver"
				body = fmt.Sprintf("Acordo aceite. %s entrega-lhe %s MT. Primeira prestação a %s. Ref. %s",
					m.Name, domain.FormatShort(l.Amount), domain.ShortDate(g.Dates[0]), l.Reference)
			}
		} else {
			body = fmt.Sprintf("Novo acordo aceite com %s: %d prestações, a primeira a %s. O plano anterior deixou de valer.", m.Name, len(g.Amounts), domain.ShortDate(g.Dates[0]))
		}
		if err := a.Credit.SaveLoan(ctx, l); err != nil {
			return err
		}
		return a.creditText(ctx, m, l.Livemode, l.ID, "", l.Phone, "credit_accepted", body, true)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, Actor{Role: "customer", IP: ip}, "microcredit.agreement_accepted", l.ID, fmt.Sprintf("v%d", l.Version))
	if deliver {
		a.deliverLoan(ctx, l.ID)
	}
	a.SendSoon(ctx)
	return a.Credit.LoanByID(ctx, l.ID)
}

// deliverLoan takes the loan out of the institution's balance and sends it to the
// customer's wallet. A failure puts the money back and waits for a retry.
func (a *App) deliverLoan(ctx context.Context, loanID string) {
	var l *domain.Loan
	var m *domain.Merchant
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		var err error
		l, err = a.Credit.LockLoan(ctx, loanID)
		if err != nil || l.DeliveryStatus != "sending" {
			l = nil
			return err
		}
		m, err = a.Merchants.Lock(ctx, l.MerchantID)
		if err != nil {
			return err
		}
		bal, err := a.Ledger.Balance(ctx, l.MerchantID, l.Livemode)
		if err != nil {
			return err
		}
		if bal < l.Amount {
			l.DeliveryStatus, l.DeliveryError, l.UpdatedAt = "failed", "Saldo RIHA insuficiente: "+domain.FormatShort(bal)+" MT disponíveis.", a.Now()
			if err := a.Credit.SaveLoan(ctx, l); err != nil {
				return err
			}
			l = nil
			return nil
		}
		return a.ledger(ctx, l.MerchantID, l.Livemode, domain.LedgerLoanOut, -l.Amount, l.ID, "Empréstimo "+l.Reference+" para "+l.Name)
	})
	if err != nil || l == nil {
		return
	}
	res, derr := a.Disburser.Disburse(ctx, domain.Disbursement{Reference: l.Reference, Phone: l.Phone, Method: l.Wallet, Amount: l.Amount, Live: l.Livemode, Note: m.Name})
	_ = a.Tx.Do(ctx, func(ctx context.Context) error {
		cur, err := a.Credit.LockLoan(ctx, loanID)
		if err != nil {
			return err
		}
		now := a.Now()
		if derr != nil {
			cur.DeliveryStatus, cur.DeliveryError, cur.UpdatedAt = "failed", domain.Trim(derr.Error(), 200), now
			if err := a.ledger(ctx, cur.MerchantID, cur.Livemode, domain.LedgerLoanOutReversal, cur.Amount, cur.ID, "Entrega falhou: "+cur.Reference); err != nil {
				return err
			}
			return a.Credit.SaveLoan(ctx, cur)
		}
		cur.DeliveryStatus, cur.DeliveryError, cur.DeliveredAt, cur.PayoutRef, cur.UpdatedAt = "sent", "", &now, res.ProviderRef, now
		if cur.Status == domain.LoanDisbursing {
			cur.Status = domain.LoanActive
		}
		if err := a.Credit.SaveLoan(ctx, cur); err != nil {
			return err
		}
		return a.creditText(ctx, m, cur.Livemode, cur.ID, "", cur.Phone, "credit_sent",
			fmt.Sprintf("%s MT enviados para o seu %s por %s. Ref. %s. Acompanhe o seu crédito em %s",
				domain.FormatShort(cur.Amount), domain.MethodNames[cur.Wallet], m.Name, cur.Reference, a.creditURL(cur.Token)), true)
	})
	a.SendSoon(ctx)
}

func (a *App) RetryDelivery(ctx context.Context, m *domain.Merchant, live bool, id string, actor Actor) (*domain.Loan, error) {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.Delivery != domain.DeliverRiha || l.DeliveryStatus != "failed" {
			return domain.Invalid("delivery_state", "id", "Esta entrega não falhou.")
		}
		if !a.CanDisburse(live) {
			return domain.Invalid("delivery_unavailable", "id", "A entrega para a carteira não está disponível agora.")
		}
		l.DeliveryStatus, l.DeliveryError, l.UpdatedAt = "sending", "", a.Now()
		return a.Credit.SaveLoan(ctx, l)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.delivery_retried", id, "")
	a.deliverLoan(ctx, id)
	return a.Credit.Loan(ctx, m.ID, id)
}

// MarkDelivered: the institution gave the money outside RIHA, or switches a
// failed delivery to outside.
func (a *App) MarkDelivered(ctx context.Context, m *domain.Merchant, live bool, id, note string, actor Actor) (*domain.Loan, error) {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.AcceptedAt == nil || (l.DeliveryStatus != "to_deliver" && l.DeliveryStatus != "failed") {
			return domain.Invalid("delivery_state", "id", "Só depois de o cliente aceitar, e se o dinheiro ainda não foi entregue.")
		}
		now := a.Now()
		l.Delivery, l.DeliveryStatus, l.DeliveryError, l.DeliveredAt, l.CloseNote, l.UpdatedAt = domain.DeliverOutside, "delivered", "", &now, clean(note, 300), now
		if l.Status == domain.LoanDisbursing {
			l.Status = domain.LoanActive
		}
		return a.Credit.SaveLoan(ctx, l)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.delivered_outside", id, note)
	return a.Credit.Loan(ctx, m.ID, id)
}

// LoanView is a loan with what it owes now.
type LoanView struct {
	Loan        *domain.Loan
	Product     *domain.CreditProduct
	Current     []domain.Instalment
	Old         []domain.Instalment
	Agreements  []domain.LoanAgreement
	Proposal    *domain.LoanAgreement
	Debt        int64
	Overdue     int64
	ToCome      int64
	Mora        int64
	PaidRiha    int64
	PaidOutside int64
	MoraPaid    int64
	Interest    int64
	DaysLate    int
	Next        *domain.Instalment
	Paid        int
	State       string
	Tone        string
	LastPaid    *domain.Instalment
}

func (a *App) loanView(ctx context.Context, l *domain.Loan, insts []domain.Instalment, withAgreements bool) *LoanView {
	now := a.Now()
	v := &LoanView{Loan: l}
	for i := range insts {
		in := insts[i]
		if in.Version == l.Version {
			v.Current = append(v.Current, in)
		} else {
			v.Old = append(v.Old, in)
		}
		switch in.Status {
		case domain.InstPaid:
			v.PaidRiha += in.Amount
			v.MoraPaid += in.MoraPaid
		case domain.InstPaidOut:
			v.PaidOutside += in.Amount
			v.MoraPaid += in.MoraPaid
		}
		if in.Paid() && (v.LastPaid == nil || in.PaidAt != nil && v.LastPaid.PaidAt != nil && in.PaidAt.After(*v.LastPaid.PaidAt)) {
			v.LastPaid = &insts[i]
		}
	}
	for i := range v.Current {
		in := &v.Current[i]
		if in.Paid() {
			v.Paid++
		}
		if !in.Open() {
			continue
		}
		mora := l.MoraOn(in, now)
		v.Mora += mora
		v.Debt += in.Amount + mora
		if d := domain.DaysLate(in.DueAt, now); d > 0 {
			v.Overdue += in.Amount
			if d > v.DaysLate {
				v.DaysLate = d
			}
		} else {
			v.ToCome += in.Amount
		}
		if v.Next == nil {
			v.Next = in
		}
	}
	v.Interest = domain.CreditInstalment(l.Amount, l.Term, l.RateBP)*int64(l.Term) - l.Amount
	if withAgreements {
		v.Agreements, _ = a.Credit.Agreements(ctx, l.ID)
		v.Proposal = proposed(v.Agreements)
		if l.Status == domain.LoanOffered || l.Status == domain.LoanApproving {
			if g := v.Proposal; g != nil {
				v.Debt = g.Total
			} else {
				v.Debt = domain.CreditInstalment(l.Amount, l.Term, l.RateBP) * int64(l.Term)
			}
		}
	}
	v.State, v.Tone = loanState(l, v)
	return v
}

func loanState(l *domain.Loan, v *LoanView) (string, string) {
	switch l.Status {
	case domain.LoanOffered:
		return "Por aceitar", "wait"
	case domain.LoanApproving:
		return "Por aprovar", "wait"
	case domain.LoanCanceled:
		return "Cancelado", "wait"
	case domain.LoanSettled:
		return "Liquidado", "ok"
	case domain.LoanWrittenOff:
		return "Perda", "bad"
	}
	if l.DeliveryStatus == "failed" {
		return "Entrega falhou", "bad"
	}
	if l.Status == domain.LoanDisbursing {
		return "A entregar", "neu"
	}
	if l.Proposed > 0 {
		return "Novo acordo enviado", "neu"
	}
	if v.DaysLate > 0 {
		t := "late"
		if v.DaysLate > 30 {
			t = "bad"
		}
		return fmt.Sprintf("Em atraso · %d %s", v.DaysLate, map[bool]string{true: "dia", false: "dias"}[v.DaysLate == 1]), t
	}
	return "Em dia", "ok"
}

var loanTabs = map[string][]string{
	"ativos":     {domain.LoanActive, domain.LoanDisbursing},
	"atraso":     {domain.LoanActive},
	"acordo":     {domain.LoanActive},
	"aceitar":    {domain.LoanOffered, domain.LoanApproving},
	"liquidados": {domain.LoanSettled},
	"perdas":     {domain.LoanWrittenOff},
}

func (a *App) CreditLoans(ctx context.Context, m *domain.Merchant, live bool, tab, productID, search string) ([]*LoanView, map[string]int, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, nil, e
	}
	all, err := a.allLoanViews(ctx, m, live, productID, search)
	if err != nil {
		return nil, nil, err
	}
	counts := map[string]int{}
	var out []*LoanView
	if _, ok := loanTabs[tab]; !ok {
		tab = "ativos"
	}
	for _, v := range all {
		for k, st := range loanTabs {
			if !domain.Contains(st, v.Loan.Status) {
				continue
			}
			if k == "atraso" && v.DaysLate == 0 || k == "acordo" && v.Loan.Proposed == 0 {
				continue
			}
			counts[k]++
			if k == tab {
				out = append(out, v)
			}
		}
	}
	if tab == "ativos" {
		sort.SliceStable(out, func(i, j int) bool { return out[i].DaysLate > out[j].DaysLate })
	}
	return out, counts, nil
}

func (a *App) allLoanViews(ctx context.Context, m *domain.Merchant, live bool, productID, search string) ([]*LoanView, error) {
	loans, err := a.Credit.Loans(ctx, portsLoanQuery(m.ID, live, productID, search))
	if err != nil {
		return nil, err
	}
	insts, err := a.Credit.InstalmentsOf(ctx, m.ID, live, []string{domain.InstScheduled, domain.InstPaid, domain.InstPaidOut, domain.InstWrittenOff})
	if err != nil {
		return nil, err
	}
	by := map[string][]domain.Instalment{}
	for _, in := range insts {
		by[in.LoanID] = append(by[in.LoanID], in)
	}
	out := make([]*LoanView, 0, len(loans))
	for i := range loans {
		out = append(out, a.loanView(ctx, &loans[i], by[loans[i].ID], false))
	}
	return out, nil
}

func (a *App) CreditLoan(ctx context.Context, m *domain.Merchant, live bool, id string) (*LoanView, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, e
	}
	l, err := a.Credit.Loan(ctx, m.ID, id)
	if err != nil || l.Livemode != live {
		return nil, domain.NotFound("O empréstimo")
	}
	insts, err := a.Credit.Instalments(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	v := a.loanView(ctx, l, insts, true)
	v.Product, _ = a.Credit.Product(ctx, m.ID, l.ProductID)
	return v, nil
}

type MarkPaidInput struct {
	Method   string `json:"method"`
	Note     string `json:"note"`
	WithMora bool   `json:"with_mora"`
}

var outsideMethods = map[string]string{"cash": "Dinheiro", "transfer": "Transferência", "wallet": "Carteira", "other": "Outro"}

func OutsideMethodLabel(k string) string { return domain.FirstNonEmpty(outsideMethods[k], k) }

func (a *App) MarkInstalmentPaid(ctx context.Context, m *domain.Merchant, live bool, loanID, instID string, in MarkPaidInput, by *domain.User, actor Actor) (*LoanView, error) {
	if _, ok := outsideMethods[in.Method]; !ok {
		return nil, domain.Invalid("parameter_invalid", "method", "Diga como foi pago.")
	}
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, loanID)
		if err != nil {
			return err
		}
		if !l.Running() {
			return domain.Invalid("loan_state", "id", "Este empréstimo já não está a correr.")
		}
		in2, err := a.Credit.LockInstalment(ctx, instID)
		if err != nil || in2.LoanID != l.ID || in2.Version != l.Version {
			return domain.NotFound("A prestação")
		}
		if !in2.Open() {
			return domain.Invalid("instalment_state", "id", "Esta prestação já não está por pagar.")
		}
		if in2.SessionID != nil {
			if p := a.LatestPayment(ctx, *in2.SessionID); p != nil && p.Status == domain.PaymentProcessing {
				return domain.Conflict("payment_in_progress", "Há um pagamento desta prestação em curso. Espere uns minutos.")
			}
		}
		now := a.Now()
		mora := l.MoraOn(in2, now)
		in2.Status, in2.PaidAt, in2.Method, in2.Note, in2.UpdatedAt = domain.InstPaidOut, &now, in.Method, clean(in.Note, 200), now
		in2.MarkedBy = domain.FirstNonEmpty(by.Name, by.Email)
		if in.WithMora {
			in2.MoraPaid = mora
		} else if mora > 0 {
			in2.MoraWaived = true
		}
		if err := a.Credit.SaveInstalment(ctx, in2); err != nil {
			return err
		}
		return a.closeIfPaid(ctx, m, l)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.instalment_paid_outside", instID, OutsideMethodLabel(in.Method))
	a.SendSoon(ctx)
	return a.CreditLoan(ctx, m, live, loanID)
}

// closeIfPaid settles a loan with nothing left to pay.
func (a *App) closeIfPaid(ctx context.Context, m *domain.Merchant, l *domain.Loan) error {
	insts, err := a.Credit.Instalments(ctx, l.ID)
	if err != nil {
		return err
	}
	for _, in := range insts {
		if in.Version == l.Version && in.Open() {
			return nil
		}
	}
	now := a.Now()
	l.Status, l.ClosedAt, l.Proposed, l.SettleOffer, l.UpdatedAt = domain.LoanSettled, &now, 0, 0, now
	if err := a.closeProposals(ctx, l.ID, domain.AgreementExpired); err != nil {
		return err
	}
	if err := a.Credit.SaveLoan(ctx, l); err != nil {
		return err
	}
	return a.creditText(ctx, m, l.Livemode, l.ID, "", l.Phone, "credit_settled",
		fmt.Sprintf("O seu crédito %s com %s está liquidado. Obrigado.", l.Reference, m.Name), true)
}

type SettleInput struct {
	Amount  int64  `json:"amount"`
	Outside bool   `json:"outside"`
	Method  string `json:"method"`
	Note    string `json:"note"`
}

// SettleEarly: the customer pays what is left at once, or the amount the
// institution agrees to, outside RIHA or through a link.
func (a *App) SettleEarly(ctx context.Context, m *domain.Merchant, live bool, id string, in SettleInput, by *domain.User, actor Actor) (*LoanView, error) {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanActive {
			return domain.Invalid("loan_state", "id", "Só um empréstimo a correr pode ser liquidado.")
		}
		insts, err := a.Credit.Instalments(ctx, l.ID)
		if err != nil {
			return err
		}
		v := a.loanView(ctx, l, insts, false)
		if in.Amount <= 0 || in.Amount%100 != 0 || in.Amount > v.Debt {
			return domain.Invalid("parameter_invalid", "amount", "O montante vai até "+domain.FormatShort(v.Debt)+" MT, em meticais inteiros.")
		}
		now := a.Now()
		if !in.Outside {
			l.SettleOffer, l.UpdatedAt = in.Amount, now
			if err := a.Credit.SaveLoan(ctx, l); err != nil {
				return err
			}
			return a.creditText(ctx, m, live, l.ID, "", l.Phone, "credit_settle_offer",
				fmt.Sprintf("%s propõe liquidar o seu crédito com um pagamento de %s MT. Pague com o PIN em %s", m.Name, domain.FormatShort(in.Amount), a.creditURL(l.Token)), true)
		}
		if _, ok := outsideMethods[in.Method]; !ok {
			return domain.Invalid("parameter_invalid", "method", "Diga como foi pago.")
		}
		var open []*domain.Instalment
		for i := range v.Current {
			if v.Current[i].Open() {
				open = append(open, &v.Current[i])
			}
		}
		parts := domain.EvenSplit(in.Amount, len(open))
		for i, o := range open {
			o.Status, o.PaidAt, o.Method, o.UpdatedAt = domain.InstPaidOut, &now, in.Method, now
			o.Amount, o.MoraWaived, o.MoraPaid, o.Note = parts[i], true, 0, "Liquidação antecipada"
			o.MarkedBy = domain.FirstNonEmpty(by.Name, by.Email)
			if err := a.Credit.SaveInstalment(ctx, o); err != nil {
				return err
			}
		}
		l.CloseNote = domain.Trim(fmt.Sprintf("Liquidação antecipada de %s MT. %s", domain.FormatShort(in.Amount), clean(in.Note, 200)), 300)
		return a.closeIfPaid(ctx, m, l)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.settled_early", id, domain.FormatShort(in.Amount)+" MT")
	a.SendSoon(ctx)
	return a.CreditLoan(ctx, m, live, id)
}

func (a *App) WriteOffLoan(ctx context.Context, m *domain.Merchant, live bool, id, note string, actor Actor) (*LoanView, error) {
	note = clean(note, 300)
	if note == "" {
		return nil, domain.Invalid("parameter_invalid", "note", "Diga porquê.")
	}
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanActive {
			return domain.Invalid("loan_state", "id", "Só um empréstimo a correr pode passar a perda.")
		}
		insts, err := a.Credit.Instalments(ctx, l.ID)
		if err != nil {
			return err
		}
		now := a.Now()
		for i := range insts {
			if insts[i].Open() {
				insts[i].Status, insts[i].UpdatedAt = domain.InstWrittenOff, now
				if err := a.Credit.SaveInstalment(ctx, &insts[i]); err != nil {
					return err
				}
			}
		}
		if err := a.closeProposals(ctx, l.ID, domain.AgreementExpired); err != nil {
			return err
		}
		l.Status, l.ClosedAt, l.CloseNote, l.Proposed, l.UpdatedAt = domain.LoanWrittenOff, &now, note, 0, now
		return a.Credit.SaveLoan(ctx, l)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.written_off", id, note)
	return a.CreditLoan(ctx, m, live, id)
}

// ResendLoanLink sends the customer the link to what is due, at most every 10 minutes.
func (a *App) ResendLoanLink(ctx context.Context, m *domain.Merchant, live bool, id string, actor Actor) error {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.UpdatedAt.After(a.Now().Add(-10*time.Minute)) && l.Status == domain.LoanOffered && l.CreatedAt.Before(l.UpdatedAt) {
			return domain.Conflict("too_soon", "Enviámos há pouco. Tente daqui a uns minutos.")
		}
		if l.Status == domain.LoanOffered {
			l.UpdatedAt = a.Now()
			if err := a.Credit.SaveLoan(ctx, l); err != nil {
				return err
			}
			return a.creditText(ctx, m, live, l.ID, "", l.Phone, "credit_offer",
				fmt.Sprintf("Lembrete: %s aprovou %s MT para si. Leia e aceite o acordo em %s", m.Name, domain.FormatShort(l.Amount), a.creditURL(l.Token)), true)
		}
		if l.Status != domain.LoanActive {
			return domain.Invalid("loan_state", "id", "Este empréstimo não tem nada por pagar.")
		}
		insts, err := a.Credit.Instalments(ctx, l.ID)
		if err != nil {
			return err
		}
		v := a.loanView(ctx, l, insts, false)
		if v.Next == nil {
			return domain.Invalid("nothing_due", "id", "Não há prestações por pagar.")
		}
		next, err := a.Credit.LockInstalment(ctx, v.Next.ID)
		if err != nil {
			return err
		}
		if next.LinkSentAt != nil && a.Now().Sub(*next.LinkSentAt) < 10*time.Minute {
			return domain.Conflict("too_soon", "Enviámos o link há pouco. Tente daqui a uns minutos.")
		}
		now := a.Now()
		next.LinkSentAt, next.UpdatedAt = &now, now
		if err := a.Credit.SaveInstalment(ctx, next); err != nil {
			return err
		}
		return a.creditText(ctx, m, live, l.ID, next.ID, l.Phone, "credit_link", a.linkText(m, l, next, l.MoraOn(next, now), len(v.Current)), true)
	})
	if err != nil {
		return err
	}
	a.Record(ctx, m.ID, actor, "microcredit.link_resent", id, "")
	a.SendSoon(ctx)
	return nil
}

func (a *App) linkText(m *domain.Merchant, l *domain.Loan, in *domain.Instalment, mora int64, n int) string {
	amt := domain.FormatShort(in.Amount + mora)
	extra := ""
	if mora > 0 {
		extra = fmt.Sprintf(" (com %s MT de mora)", domain.FormatShort(mora))
	}
	return fmt.Sprintf("%s: prestação %d de %d, %s MT%s, vence a %s. Pague com o PIN em %s", m.Name, in.Seq, n, amt, extra, domain.ShortDate(in.DueAt), a.creditURL(l.Token))
}

type AgreementInput struct {
	Option      string `json:"option"`
	ExtraMonths int    `json:"extra_months"`
	Day         int    `json:"day"`
	Waive       bool   `json:"waive"`
}

// PlanAgreement works out a new plan from what the loan owes now.
func (a *App) PlanAgreement(v *LoanView, in AgreementInput) ([]time.Time, []int64, int64, *domain.Error) {
	if in.Day < 1 || in.Day > 28 {
		return nil, nil, 0, domain.Invalid("parameter_invalid", "day", "O dia de pagamento vai de 1 a 28.")
	}
	left := 0
	for _, in := range v.Current {
		if in.Open() && domain.DaysLate(in.DueAt, a.Now()) <= 0 {
			left++
		}
	}
	n := left
	switch in.Option {
	case "join":
	case "extend":
		if in.ExtraMonths < 1 || in.ExtraMonths > 12 {
			return nil, nil, 0, domain.Invalid("parameter_invalid", "extra_months", "Alargue de 1 a 12 meses.")
		}
		n += in.ExtraMonths
	default:
		return nil, nil, 0, domain.Invalid("parameter_invalid", "option", "Escolha juntar o atraso ou alargar o prazo.")
	}
	if n < 1 {
		n = 1
	}
	total := v.Overdue + v.ToCome
	waived := v.Mora
	if !in.Waive {
		total += v.Mora
		waived = 0
	}
	return domain.NextDates(in.Day, n, a.Now()), domain.EvenSplit(total, n), waived, nil
}

func (a *App) ProposeAgreement(ctx context.Context, m *domain.Merchant, live bool, id string, in AgreementInput, by *domain.User, actor Actor) (*LoanView, error) {
	err := a.Tx.Do(ctx, func(ctx context.Context) error {
		l, err := a.lockLoan(ctx, m, live, id)
		if err != nil {
			return err
		}
		if l.Status != domain.LoanActive {
			return domain.Invalid("loan_state", "id", "Só um empréstimo a correr pode ter um novo acordo.")
		}
		insts, err := a.Credit.Instalments(ctx, l.ID)
		if err != nil {
			return err
		}
		v := a.loanView(ctx, l, insts, false)
		if v.Debt <= 0 {
			return domain.Invalid("nothing_due", "id", "Não há nada por pagar.")
		}
		dates, amounts, waived, e := a.PlanAgreement(v, in)
		if e != nil {
			return e
		}
		if err := a.closeProposals(ctx, l.ID, domain.AgreementSuperseded); err != nil {
			return err
		}
		var total int64
		for _, x := range amounts {
			total += x
		}
		gs, _ := a.Credit.Agreements(ctx, l.ID)
		ver := l.Version + 1
		for _, g := range gs {
			if g.Version >= ver {
				ver = g.Version + 1
			}
		}
		cp := *l
		cp.Version = ver
		g := &domain.LoanAgreement{ID: domain.NewID("lagr"), LoanID: l.ID, MerchantID: m.ID, Version: ver, Status: domain.AgreementProposed, Total: total,
			Dates: dates, Amounts: amounts, MoraWaived: waived, SentAt: a.Now(), CreatedBy: by.ID, Text: domain.CreditAgreementText(m.Name, &cp, dates, amounts, waived)}
		if err := a.Credit.CreateAgreement(ctx, g); err != nil {
			return err
		}
		l.Proposed, l.AskedAt, l.UpdatedAt = ver, nil, a.Now()
		if err := a.Credit.SaveLoan(ctx, l); err != nil {
			return err
		}
		return a.creditText(ctx, m, live, l.ID, "", l.Phone, "credit_new_agreement",
			fmt.Sprintf("%s propõe um novo plano para o seu crédito: %d prestações de %s MT. Veja e aceite com um código SMS em %s",
				m.Name, len(amounts), domain.FormatShort(amounts[0]), a.creditURL(l.Token)), true)
	})
	if err != nil {
		return nil, err
	}
	a.Record(ctx, m.ID, actor, "microcredit.agreement_proposed", id, in.Option)
	a.SendSoon(ctx)
	return a.CreditLoan(ctx, m, live, id)
}

// AskNewAgreement: the customer says they can't pay, and the institution is told.
func (a *App) AskNewAgreement(ctx context.Context, token string) error {
	l, _, err := a.creditLoanByToken(ctx, token)
	if err != nil {
		return err
	}
	return a.Tx.Do(ctx, func(ctx context.Context) error {
		cur, err := a.Credit.LockLoan(ctx, l.ID)
		if err != nil {
			return err
		}
		if cur.Status != domain.LoanActive {
			return domain.Invalid("loan_state", "token", "Este crédito não está a correr.")
		}
		if cur.AskedAt != nil || cur.Proposed > 0 {
			return nil
		}
		now := a.Now()
		cur.AskedAt, cur.UpdatedAt = &now, now
		return a.Credit.SaveLoan(ctx, cur)
	})
}

func portsLoanQuery(merchantID string, live bool, productID, search string) ports.LoanQuery {
	return ports.LoanQuery{MerchantID: merchantID, Live: live, ProductID: productID, Search: strings.TrimSpace(search), Limit: 2000}
}

func (a *App) agreementPDF(ctx context.Context, m *domain.Merchant, l *domain.Loan, version int) ([]byte, error) {
	gs, err := a.Credit.Agreements(ctx, l.ID)
	if err != nil {
		return nil, err
	}
	var g *domain.LoanAgreement
	for i := range gs {
		if version == 0 && (gs[i].Status == domain.AgreementAccepted || gs[i].Status == domain.AgreementProposed) || gs[i].Version == version {
			g = &gs[i]
		}
	}
	if g == nil {
		return nil, domain.NotFound("O acordo")
	}
	doc := domain.AgreementDoc{Institution: m.Name, Loan: *l, Agreement: *g, Generated: a.Now()}
	if p, err := a.Credit.Product(ctx, m.ID, l.ProductID); err == nil {
		doc.Product = p.Name
	}
	if l.RequestID != "" {
		if r, err := a.Credit.Request(ctx, m.ID, l.RequestID); err == nil {
			doc.IDNumber = r.IDNumber
		}
	}
	return a.Renderer.CreditAgreement(doc)
}

func (a *App) LoanAgreementPDF(ctx context.Context, m *domain.Merchant, live bool, id string, version int) ([]byte, *domain.Loan, error) {
	if e := domain.RequireMicrocredit(m); e != nil {
		return nil, nil, e
	}
	l, err := a.Credit.Loan(ctx, m.ID, id)
	if err != nil || l.Livemode != live {
		return nil, nil, domain.NotFound("O empréstimo")
	}
	b, err := a.agreementPDF(ctx, m, l, version)
	return b, l, err
}

func (a *App) PublicAgreementPDF(ctx context.Context, token string, version int) ([]byte, *domain.Loan, error) {
	l, m, err := a.creditLoanByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	b, err := a.agreementPDF(ctx, m, l, version)
	return b, l, err
}
