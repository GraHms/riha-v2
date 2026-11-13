package seed

import (
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

const (
	CreditEmail    = "ola@avante.co.mz"
	CreditPassword = "avante-2026"
)

// avante is a microcredit institution with the loans of the design: some on time,
// some late, one renegotiated, requests waiting for a decision.
func (s *seeder) avante() {
	ctx, a := s.ctx, s.a
	real := a.Now
	today := real()
	clock := today.AddDate(0, 0, -150)
	a.Now = func() time.Time { return clock }
	a.CreditCode = func() string { return "123456" }
	defer func() { a.Now, a.CreditCode = real, nil }()
	at := func(t time.Time) { clock = t }

	u, m, err := a.Signup(ctx, app.SignupInput{BusinessName: "Avante Microcrédito", Name: "Ana Cumbe", Email: CreditEmail, Phone: "843001020", Password: CreditPassword})
	if err != nil {
		s.do(err)
		return
	}
	m.LegalName, m.BusinessType, m.NUIT, m.City = "Avante Microcrédito, SA", "company", "400991122", "Maputo"
	m.ExpectedVolume, m.Status, m.ApprovedAt, m.KycSubmittedAt = "1 – 3 milhões MT/mês", domain.MerchantActive, &clock, &clock
	m.AvatarColor, m.StatementDescriptor = "#FFD21F", "RIHA*AVANTE MC"
	s.do(a.Merchants.Save(ctx, m))
	s.do(a.Banks.Save(ctx, &domain.BankAccount{ID: domain.NewID("ba"), MerchantID: m.ID, BankName: "BCI", Holder: "Avante Microcrédito, SA", NIB: "000800004821130101999", Verified: true, UpdatedAt: clock}))
	admin, err := a.Users.ByEmail(ctx, AdminEmail)
	if err != nil {
		s.do(err)
		return
	}
	if m, err = a.SetMerchantMicrocredit(ctx, admin, m.ID, true, "Contrato de serviço assinado", app.Actor{ID: admin.ID, Role: domain.RoleAdmin}); err != nil {
		s.do(err)
		return
	}
	s.do(a.Ledger.Add(ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: m.ID, Livemode: true, Type: domain.LedgerPayment, Amount: 41230000, ReferenceID: "seed", Description: "Saldo inicial", CreatedAt: clock}))
	set := domain.DefaultCreditSettings()
	set.SenderID = "AvanteMC"
	_, err = a.SaveCreditSettings(ctx, m, set, app.Actor{ID: u.ID})
	s.do(err)
	who := app.Actor{ID: u.ID, Role: domain.RoleMerchant}
	prod := func(in app.ProductInput) *domain.CreditProduct {
		p, err := a.CreateCreditProduct(ctx, m, true, in, who)
		s.do(err)
		return p
	}
	all := []string{domain.FieldID, domain.FieldActivity, domain.FieldIncome, domain.FieldPurpose}
	negocio := prod(app.ProductInput{Name: "Crédito Negócio", Min: 500000, Max: 5000000, Freq: domain.FreqMonthly, TermMin: 2, TermMax: 12, RateBP: 600,
		DayMode: domain.DayAtDelivery, Mora: true, MoraBP: 100, Grace: 2, MoraMaxWeeks: 4, EffortPct: 35, Fields: all})
	salario := prod(app.ProductInput{Name: "Crédito Salário", Min: 300000, Max: 3000000, Freq: domain.FreqMonthly, TermMin: 1, TermMax: 3, RateBP: 500,
		DayMode: domain.DayClient, Mora: true, MoraBP: 100, Grace: 2, MoraMaxWeeks: 4, EffortPct: 35, Fields: all})
	rapido := prod(app.ProductInput{Name: "Crédito Rápido", Min: 100000, Max: 1000000, Freq: domain.FreqWeekly, TermMin: 4, TermMax: 8, RateBP: 200,
		DayMode: domain.DayAtDelivery, EffortPct: 35, Fields: []string{domain.FieldID, domain.FieldActivity, domain.FieldIncome}})
	prod(app.ProductInput{Name: "Crédito Machamba", Min: 1000000, Max: 8000000, Freq: domain.FreqMonthly, TermMin: 6, TermMax: 9, RateBP: 400,
		DayMode: domain.DayAtDelivery, Mora: true, MoraBP: 100, Grace: 5, MoraMaxWeeks: 4, EffortPct: 40, Fields: all, Status: domain.ProductDraft})
	if s.err != nil {
		return
	}

	loan := func(p *domain.CreditProduct, name, phone string, amount int64, term, day int, delivery string, ago int) *domain.Loan {
		if delivery == domain.DeliverRiha && !a.CanDisburse(true) {
			delivery = domain.DeliverOutside
		}
		at(today.AddDate(0, 0, -ago))
		l, err := a.CreateLoan(ctx, m, true, app.OfferInput{ProductID: p.ID, Phone: phone, Name: name, Amount: amount, Term: term, Day: day, Delivery: delivery}, u, who)
		if err != nil {
			s.do(err)
			return nil
		}
		if _, err := a.SendAgreementCode(ctx, l.Token, true); err != nil {
			s.do(err)
			return nil
		}
		at(clock.Add(4 * time.Minute))
		l, err = a.AcceptAgreement(ctx, l.Token, "123456", "41.220.0.1")
		s.do(err)
		if l != nil && delivery == domain.DeliverOutside {
			l, err = a.MarkDelivered(ctx, m, true, l.ID, "Entregue no balcão", who)
			s.do(err)
		}
		return l
	}
	paid := func(l *domain.Loan, n int, how ...string) {
		if l == nil {
			return
		}
		v, err := a.CreditLoan(ctx, m, true, l.ID)
		if err != nil {
			s.do(err)
			return
		}
		for i := 0; i < n && i < len(v.Current); i++ {
			x := v.Current[i]
			method := domain.WalletFor(l.Phone)
			if i < len(how) && how[i] != "" {
				method = how[i]
			}
			at(x.DueAt.Add(time.Duration(2+i%3) * time.Hour))
			if method == "cash" {
				_, err := a.MarkInstalmentPaid(ctx, m, true, l.ID, x.ID, app.MarkPaidInput{Method: "cash"}, u, who)
				s.do(err)
				continue
			}
			cur, err := a.Credit.LockInstalment(ctx, x.ID)
			if err != nil {
				s.do(err)
				return
			}
			t := clock
			cur.Status, cur.PaidAt, cur.Method, cur.UpdatedAt = domain.InstPaid, &t, method, t
			cur.Steps = "remind,link"
			s.do(a.Credit.SaveInstalment(ctx, cur))
			net := cur.Amount - cur.Amount*int64(a.DefaultFeeBps(ctx))/10000
			s.do(a.Ledger.Add(ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: m.ID, Livemode: true, Type: domain.LedgerPayment, Amount: net, ReferenceID: x.ID, Description: "Prestação " + l.Reference, CreatedAt: t}))
		}
	}

	z := loan(negocio, "Zacarias Muianga", "847301188", 3000000, 6, 5, domain.DeliverOutside, 135)
	paid(z, 3, "", domain.MethodEmola, "cash")
	paid(loan(negocio, "Amina Abdula", "862045531", 5000000, 9, 15, domain.DeliverRiha, 62), 1)
	paid(loan(salario, "Arnaldo Sitoe", "846619027", 1200000, 3, 25, domain.DeliverRiha, 52), 1)
	paid(loan(negocio, "Custódio Matusse", "870456610", 2400000, 7, 5, domain.DeliverRiha, 112), 1)
	paid(loan(rapido, "Isaura Chilaule", "853907742", 600000, 6, 0, domain.DeliverRiha, 30), 4)
	n := loan(salario, "Nélia Zandamela", "842773059", 2000000, 3, 25, domain.DeliverRiha, 82)
	paid(n, 1)
	paid(loan(negocio, "Edson Guambe", "869182204", 1500000, 5, 15, domain.DeliverOutside, 125), 3, "", "", "cash")
	paid(loan(rapido, "Rosa Tivane", "856024417", 300000, 4, 0, domain.DeliverRiha, 24), 3)
	old := loan(negocio, "Celeste Machava", "845550142", 1500000, 4, 12, domain.DeliverRiha, 240)
	paid(old, 4, "", "", "", "cash")

	at(today.Add(-90 * time.Minute))
	if n != nil {
		_, err := a.ProposeAgreement(ctx, m, true, n.ID, app.AgreementInput{Option: "extend", ExtraMonths: 2, Day: 25, Waive: true}, u, who)
		s.do(err)
	}
	at(today.AddDate(0, 0, -3))
	_, err = a.CreateLoan(ctx, m, true, app.OfferInput{ProductID: rapido.ID, Phone: "841186203", Name: "Felizarda Cuna", Amount: 500000, Term: 4, Day: 1, Delivery: domain.DeliverOutside}, u, who)
	s.do(err)

	req := func(p *domain.CreditProduct, ago time.Duration, in app.CreditRequestInput) *domain.CreditRequest {
		at(today.Add(-ago))
		in.ProductID = p.ID
		r, _, err := a.StartCreditRequest(ctx, m.Slug, in)
		if err != nil {
			s.do(err)
			return nil
		}
		r, err = a.ConfirmCreditRequest(ctx, r.Token, "123456")
		s.do(err)
		return r
	}
	req(negocio, 3*time.Hour, app.CreditRequestInput{Amount: 2000000, Term: 4, Name: "Celeste Machava", Phone: "845550142", IDNumber: "110100456789B", NUIT: "104882316",
		Address: "Bairro do Zimpeto, Q. 14, Maputo", Activity: "Vendedora de capulanas · Mercado do Zimpeto, há 6 anos", Purpose: "Comprar capulanas para a época de dezembro", Income: 2500000})
	req(negocio, 29*time.Hour, app.CreditRequestInput{Amount: 3500000, Term: 8, Name: "Momade Ussene", Phone: "872204410", IDNumber: "110100553311C", Activity: "Alfaiate", Purpose: "Máquina de costura", Income: 2200000})
	req(salario, 22*time.Hour, app.CreditRequestInput{Amount: 1500000, Term: 3, Day: 25, Name: "Benedita Langa", Phone: "849027716", IDNumber: "110100221144D", Activity: "Professora", Purpose: "Propinas", Income: 1900000})
	if g := req(negocio, 20*time.Hour, app.CreditRequestInput{Amount: 1200000, Term: 5, Name: "Graça Nhaca", Phone: "857310098", IDNumber: "110100667788E", Activity: "Salão de cabeleireiro", Purpose: "Secadores novos", Income: 2400000}); g != nil {
		s.do(a.ReviewCreditRequest(ctx, m, true, g.ID, who))
	}
	if o := req(rapido, 17*time.Hour, app.CreditRequestInput{Amount: 800000, Term: 8, Name: "Orlando Mucavele", Phone: "864102291", IDNumber: "110100998877F", Activity: "Motorista de chapa", Income: 1500000}); o != nil {
		s.do(a.ReviewCreditRequest(ctx, m, true, o.ID, who))
	}
	at(today)
	a.CreditTick(ctx)
}
