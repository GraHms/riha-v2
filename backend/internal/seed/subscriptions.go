package seed

import (
	"fmt"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

const (
	SchoolEmail    = "escola@primavera.co.mz"
	SchoolPassword = "primavera-2026"
)

type pupil struct {
	name, turma, payer, phone string
	plans                     []string
	join                      string
	card                      bool
	pending                   bool
}

func day(s string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02", s, domain.Maputo)
	return t
}

func (s *seeder) escolaPrimavera() {
	ctx, a := s.ctx, s.a
	real := a.Now
	today := domain.LocalDate(real())
	if today.Before(day("2026-03-01")) {
		return
	}
	year := today.Year()
	if today.Month() < time.March {
		year--
	}
	ymd := func(md string) time.Time { return day(fmt.Sprintf("%d-%s", year, md)) }
	clock := ymd("01-20").Add(10 * time.Hour).UTC()
	a.Now = func() time.Time { return clock }
	defer func() { a.Now = real }()

	u, m, err := a.Signup(ctx, app.SignupInput{BusinessName: "Escola Primavera", Name: "Ana Cossa", Email: SchoolEmail, Phone: "843310022", Password: SchoolPassword})
	if err != nil {
		s.do(err)
		return
	}
	m.LegalName, m.BusinessType, m.NUIT, m.City = "Escola Primária Primavera, Lda", "company", "400778812", "Maputo"
	m.ExpectedVolume, m.Status, m.ApprovedAt, m.KycSubmittedAt = "150 – 500 mil MT/mês", domain.MerchantActive, &clock, &clock
	m.AvatarColor, m.StatementDescriptor = "#FFD21F", "RIHA*ESC PRIMAVERA"
	s.do(a.Merchants.Save(ctx, m))
	s.do(a.Banks.Save(ctx, &domain.BankAccount{ID: domain.NewID("ba"), MerchantID: m.ID, BankName: "Millennium bim", Holder: "Escola Primária Primavera, Lda", NIB: "000100003150000777712", Verified: true, UpdatedAt: clock}))
	owner := app.Actor{ID: u.ID, Role: "merchant"}
	set := domain.DefaultSubscriptionSettings(m)
	set.SenderID, set.FinalAction = "EPrimavera", domain.FinalKeep
	_, err = a.SaveSubSettings(ctx, m, set, owner)
	s.do(err)

	plan := func(in app.PlanInput) *domain.Plan {
		p, err := a.CreatePlan(ctx, m, true, in, owner)
		s.do(err)
		return p
	}
	school := 0x7FE
	p14 := plan(app.PlanInput{Name: "Propina · 1ª a 4ª classe", Amount: 250000, Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed, BillingDay: 5, Months: school, Missed: domain.MissedDebt, Fields: []string{"Nome do aluno", "Turma"}})
	p57 := plan(app.PlanInput{Name: "Propina · 5ª a 7ª classe", Amount: 300000, Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed, BillingDay: 5, Months: school, Missed: domain.MissedDebt, Fields: []string{"Nome do aluno", "Turma"}})
	tr := plan(app.PlanInput{Name: "Transporte escolar", Amount: 120000, Interval: domain.IntervalMonth, Anchor: domain.AnchorSignup, Months: school, Missed: domain.MissedForfeit, Fields: []string{"Nome do aluno", "Turma"}})
	plan(app.PlanInput{Name: "Natação", Amount: 80000, Interval: domain.IntervalMonth, Anchor: domain.AnchorFixed, BillingDay: 1, Months: domain.AllMonths, Missed: domain.MissedForfeit, Fields: []string{"Nome do aluno"}, Draft: true})
	if s.err != nil || p14 == nil || p57 == nil || tr == nil {
		return
	}
	ids := map[string]string{"p14": p14.ID, "p57": p57.ID, "tr": tr.ID}

	pupils := []pupil{
		{"Kelvin Mabunda", "3ª B", "Tânia Mabunda", "841234567", []string{"p14"}, "02-02", false, false},
		{"Ana Sitoe", "1ª A", "Carlos Sitoe", "852201145", []string{"p14", "tr"}, "02-02", false, false},
		{"Délcio Macuácua", "4ª A", "Rosa Macuácua", "864109921", []string{"p14"}, "02-02", false, false},
		{"Nádia Chissano", "2ª B", "Júlio Chissano", "847780032", []string{"p14"}, "02-02", false, false},
		{"Ivan Nhantumbo", "6ª A", "Sara Nhantumbo", "873016620", []string{"p57"}, "02-02", false, false},
		{"Yuran Tembe", "3ª A", "Lúcia Tembe", "849123307", []string{"p14"}, "02-03", true, false},
		{"Leila Mondlane", "5ª B", "Fátima Mondlane", "850147788", []string{"p57"}, "02-03", false, false},
		{"Telma Bila", "1ª B", "Inês Bila", "845052281", []string{"p14"}, "03-02", false, false},
		{"Ricardo Langa", "4ª B", "Marta Langa", "846612203", []string{"p14"}, "02-02", false, false},
		{"Joana Mucavele", "5ª A", "Elsa Mucavele", "870331245", []string{"p57"}, "02-02", false, false},
		{"Sérgio Matsinhe", "7ª A", "Alberto Matsinhe", "842209876", []string{"p57", "tr"}, "02-04", false, false},
		{"Carla Nhaca", "2ª A", "Graça Nhaca", "861234098", []string{"p14"}, "02-02", false, false},
		{"Edson Cumbe", "6ª B", "Rui Cumbe", "848877665", []string{"p57"}, "04-02", false, false},
		{"Lurdes Massingue", "3ª A", "Teresa Massingue", "856654321", []string{"p14", "tr"}, "02-02", false, false},
		{"Tomás Guambe", "1ª A", "Felisberto Guambe", "843321100", []string{"p14"}, "02-02", true, false},
		{"Rita Chambal", "7ª B", "Helena Chambal", "872214455", []string{"p57"}, "02-02", false, false},
		{"Amélia Zandamela", "4ª A", "João Zandamela", "844455667", []string{"p14"}, "05-04", false, false},
		{"Benedito Mussa", "5ª B", "Anita Mussa", "865566778", []string{"p57", "tr"}, "02-02", false, false},
		{"Celso Bila", "2ª B", "Inês Bila", "845052281", []string{"p14"}, "03-02", false, false},
		{"Hélio Cossa", "2ª A", "Pedro Cossa", "866629999", []string{"tr"}, "10-27", false, true},
		{"Mércia Nhantumbo", "1ª B", "Sara Nhantumbo", "873016620", []string{"p14"}, "10-26", false, true},
		{"Paulo Tivane", "3ª B", "Ilda Tivane", "847001122", []string{"p14"}, "10-28", false, true},
		{"Zito Macamo", "6ª A", "Abel Macamo", "851002233", []string{"p57"}, "10-29", false, true},
	}
	unpaid := map[string]bool{"Kelvin Mabunda|10": true, "Ivan Nhantumbo|9": true, "Ivan Nhantumbo|10": true, "Telma Bila|10": true}
	outside := map[string]string{"Kelvin Mabunda|8": "cash", "Délcio Macuácua|10": "transfer", "Carla Nhaca|6": "deposit", "Amélia Zandamela|10": "cash",
		"Benedito Mussa|10": "cash", "Edson Cumbe|9": "cash", "Rita Chambal|10": "transfer"}
	late := map[string]int{"Telma Bila|7": 4, "Ricardo Langa|9": 3, "Joana Mucavele|10": 1}
	byName := map[string]*pupil{}
	for i := range pupils {
		byName[pupils[i].name] = &pupils[i]
	}

	pay := func(mem *domain.Member, token string, chargeIDs []string) {
		p := byName[mem.Name]
		in := app.InvoicePay{ChargeIDs: chargeIDs, Accept: true, UseLast: true, Method: domain.WalletFor(mem.Phone)}
		if p != nil && p.card {
			in = app.InvoicePay{ChargeIDs: chargeIDs, Accept: true, Method: domain.MethodCard, CardNumber: "4242424242424242", CardExpiry: "12/30", CardCVC: "123"}
		}
		pm, err := a.PayInvoice(ctx, token, in)
		if err != nil {
			s.do(fmt.Errorf("seed pay %s: %w", mem.Name, err))
			return
		}
		if pm.Status == domain.PaymentProcessing {
			s.do(a.CompletePayment(ctx, pm.ID, domain.Outcome{Success: true, OperatorReference: "MZ" + domain.ShortCode(8)}))
		}
	}
	live := true
	for d := ymd("02-01"); !d.After(today); d = d.AddDate(0, 0, 1) {
		clock = d.Add(9*time.Hour + 30*time.Minute).UTC()
		md := d.Format("01-02")
		for _, p := range pupils {
			if p.join != md {
				continue
			}
			var planIDs []string
			for _, k := range p.plans {
				planIDs = append(planIDs, ids[k])
			}
			mem, err := a.InviteMember(ctx, m, true, app.MemberInput{PlanIDs: planIDs, Phone: p.phone, PayerName: p.payer, Fields: map[string]string{"Nome do aluno": p.name, "Turma": p.turma}}, owner)
			if err != nil {
				s.do(fmt.Errorf("seed invite %s: %w", p.name, err))
				continue
			}
			if p.pending {
				continue
			}
			cs := a.ChargesFor(ctx, ports.ChargeQuery{MemberID: mem.ID, Statuses: domain.UnpaidChargeStatuses})
			if len(cs) > 0 {
				pay(mem, cs[0].Token, nil)
			}
		}
		if md == "08-20" {
			for _, mem := range s.members(m.ID, "Nádia Chissano") {
				for _, sub := range a.SubscriptionList(ctx, m.ID, true, mem.ID, nil, 5) {
					s.do(a.PauseSubscription(ctx, m, true, sub.ID, ymd("02-01").AddDate(1, 0, 0), owner))
				}
			}
		}
		a.BillingTick(ctx)
		open := a.ChargesFor(ctx, ports.ChargeQuery{MerchantID: m.ID, Live: &live, Statuses: domain.UnpaidChargeStatuses})
		group := map[string][]domain.Charge{}
		for _, c := range open {
			if !c.DueAt.After(clock) {
				group[c.MemberID] = append(group[c.MemberID], c)
			}
		}
		for memberID, cs := range group {
			mem := a.MemberByID(ctx, memberID)
			if mem == nil || mem.Status == domain.SubInvited {
				continue
			}
			var ids []string
			token := ""
			for _, c := range cs {
				key := fmt.Sprintf("%s|%d", mem.Name, int(c.DueAt.In(domain.Maputo).Month()))
				if unpaid[key] {
					continue
				}
				if method, ok := outside[key]; ok {
					_, err := a.MarkPaidOutside(ctx, m, true, mem.ID, app.OutsideInput{ChargeIDs: []string{c.ID}, Method: method, Date: clock}, owner, "Ana Cossa")
					s.do(err)
					continue
				}
				wait := late[key]
				if wait == 0 {
					wait = int(s.rng.Int63n(2))
				}
				if domain.LocalDate(clock).Sub(domain.LocalDate(c.DueAt)) < time.Duration(wait)*24*time.Hour {
					continue
				}
				ids = append(ids, c.ID)
				token = c.Token
			}
			if len(ids) > 0 {
				pay(mem, token, ids)
			}
		}
	}
	clock = real()
	for _, mem := range s.members(m.ID, "Yuran Tembe") {
		for _, sub := range a.SubscriptionList(ctx, m.ID, true, mem.ID, nil, 5) {
			_, err := a.SkipNext(ctx, m, true, sub.ID, owner)
			s.do(err)
		}
	}
	a.SendQueued(ctx)
}

func (s *seeder) members(merchantID, name string) []domain.Member {
	ms, _ := s.a.MemberList(s.ctx, ports.MemberQuery{MerchantID: merchantID, Live: true, Search: name, Statuses: append([]string{domain.SubInvited}, domain.LiveSubStatuses...)})
	return ms
}
