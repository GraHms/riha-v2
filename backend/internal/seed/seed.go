package seed

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"riha/internal/core/app"
	"riha/internal/core/domain"
)

const (
	AdminEmail       = "admin@riha.co.mz"
	AdminPassword    = "riha-admin-2026"
	MerchantEmail    = "ola@cafemaputo.co.mz"
	MerchantPassword = "cafe-maputo-2026"
)

type seeder struct {
	ctx context.Context
	a   *app.App
	rng *rand.Rand
	now time.Time
	err error
}

func (s *seeder) do(err error) {
	if s.err == nil && err != nil {
		s.err = err
	}
}

func ptr[T any](v T) *T { return &v }

func Run(ctx context.Context, a *app.App) error {
	if _, err := a.Users.ByEmail(ctx, AdminEmail); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	log.Printf("seed: creating demo data")
	s := &seeder{ctx: ctx, a: a, rng: rand.New(rand.NewSource(42)), now: a.Now()}
	s.do(a.CreateAdmin(ctx, AdminEmail, AdminPassword, "Nélia Chissano", "Operações"))
	s.do(a.SetDefaultFeeBps(ctx, domain.DefaultFeeBps))
	s.cafeMaputo()
	s.others()
	s.applicants()
	s.escolaPrimavera()
	return s.err
}

func (s *seeder) payout(mid string, amount int64, ageH int, status, bank, holder, nib string) {
	num, err := s.a.Payouts.NextNumber(s.ctx)
	s.do(err)
	req := s.now.Add(-time.Duration(ageH) * time.Hour)
	p := &domain.Payout{ID: domain.NewID("po"), Number: num, MerchantID: mid, Livemode: true, Amount: amount, Status: status, BankName: bank, Holder: holder, NIB: nib,
		RequestedAt: req, DueAt: req.Add(domain.PayoutSLA), UpdatedAt: req, RequestedBy: "seed"}
	if status == domain.PayoutSent || status == domain.PayoutPaid {
		t := req.Add(20 * time.Hour)
		p.SentAt, p.ReviewedAt = &t, &t
	}
	if status == domain.PayoutPaid {
		t := req.Add(50 * time.Hour)
		p.PaidAt, p.ClosedAt = &t, &t
	}
	s.do(s.a.Payouts.Create(s.ctx, p))
	s.do(s.a.Ledger.Add(s.ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: mid, Livemode: true, Type: domain.LedgerPayout, Amount: -amount, ReferenceID: p.ID, Description: "Levantamento " + p.Ref(), CreatedAt: req}))
}

var names = []string{"Ana Machava", "Carlos Nhantumbo", "Fátima Cossa", "Júlio Sitoe", "Rosa Langa", "Tomás Muianga", "Helena Tembe", "Inês Matsinhe", "Paulo Chissano", "Lurdes Mondlane", "Edson Bila", "Célia Nhaca"}

func (s *seeder) cafeMaputo() {
	ctx, a, now, rng := s.ctx, s.a, s.now, s.rng
	u, m, err := a.Signup(ctx, app.SignupInput{BusinessName: "Café Maputo", Name: "Ana Mabota", Email: MerchantEmail, Phone: "841234567", Password: MerchantPassword})
	if err != nil {
		s.do(err)
		return
	}
	approved := now.AddDate(0, 0, -20)
	m.LegalName, m.BusinessType, m.NUIT, m.City = "Café Maputo, Lda", "company", "400123456", "Maputo"
	m.ExpectedVolume, m.Status, m.ApprovedAt, m.KycSubmittedAt = "50 – 150 mil MT/mês", domain.MerchantActive, &approved, ptr(approved.Add(-20*time.Hour))
	m.SuccessURL, m.CancelURL, m.AvatarColor, m.StatementDescriptor = "https://cafemaputo.co.mz/obrigado", "https://cafemaputo.co.mz/carrinho", "#FFD21F", "RIHA*CAFE MAPUTO"
	m.CreatedAt = approved.AddDate(0, 0, -2)
	s.do(a.Merchants.Save(ctx, m))
	nib := "000800004821130101135"
	s.do(a.Banks.Save(ctx, &domain.BankAccount{ID: domain.NewID("ba"), MerchantID: m.ID, BankName: "BCI", Holder: "Café Maputo, Lda", NIB: nib, Verified: true, UpdatedAt: now}))
	for _, d := range []string{"cafemaputo.co.mz", "loja.cafemaputo.co.mz"} {
		_, err := a.AddDomain(ctx, m.ID, d)
		s.do(err)
	}
	for _, live := range []bool{true, false} {
		s.do(a.Endpoints.Create(ctx, &domain.WebhookEndpoint{ID: domain.NewID("we"), MerchantID: m.ID, Livemode: live, URL: "https://cafemaputo.co.mz/api/riha",
			Events: []string{"payment.succeeded", "payment.failed", "payout.paid"}, Active: false, CreatedAt: now}))
	}

	mk := func(name, slug string, amount int64, custom bool, kind, loc string, live bool, age int) *domain.PaymentLink {
		l := &domain.PaymentLink{ID: domain.NewID("plink"), MerchantID: m.ID, Livemode: live, Name: name, Slug: slug, Amount: amount, CustomAmount: custom,
			Reusable: true, Methods: domain.AllMethods, Active: true, Kind: kind, CreatedBy: u.ID, CreatedVia: "dashboard", Location: loc, CreatedAt: now.AddDate(0, 0, -age)}
		s.do(a.Links.Create(ctx, l))
		return l
	}
	bolo := mk("Bolo de chocolate · 1 kg", "bolo", 125000, false, "link", "", true, 18)
	cabaz := mk("Cabaz de café moído", "cabaz-de-cafe-moido", 89000, false, "link", "", true, 17)
	workshop := mk("Workshop de barista · 18 nov", "workshop-de-barista-18-nov", 250000, false, "link", "", true, 10)
	gorjeta := mk("Gorjeta", "gorjeta", 0, true, "link", "", true, 15)
	balcao := mk("Balcão Sommerschield", "balcao-sommerschield", 0, true, "qr", "Loja da Sommerschield", true, 19)
	natal := mk("Encomenda de Natal", "encomenda-de-natal", 450000, false, "link", "", true, 2)
	natal.ExpiresAt = ptr(now.AddDate(0, 1, 20))
	s.do(a.Links.Save(ctx, natal))
	promo := mk("Promoção de outubro", "promocao-de-outubro", 60000, false, "link", "", true, 35)
	promo.Active = false
	s.do(a.Links.Save(ctx, promo))
	testLink := mk("Bolo de chocolate · 1 kg", "bolo-teste", 125000, false, "link", "", false, 10)

	keyID := "key_3f9a"
	s.do(a.Keys.Create(ctx, &domain.APIKey{ID: keyID, MerchantID: m.ID, Livemode: true, Kind: domain.KeySecret, Hash: domain.Hash(domain.Token("sk_live_", 32)), Last4: "3f9a", CreatedBy: u.ID, CreatedAt: approved.Add(time.Hour)}))

	bps := a.FeeBpsFor(ctx, m)
	order := 2060
	add := func(when time.Time, amount int64, method, status, src string, link *domain.PaymentLink, live bool) {
		p := &domain.Payment{ID: domain.NewID("pay"), Reference: domain.PaymentReference(), MerchantID: m.ID, Livemode: live, Status: status, Amount: amount, FeeBps: bps, Method: method, CreatedAt: when}
		p.Fee, p.Net = domain.Split(amount, bps)
		p.SourceType = src
		cs := &domain.CheckoutSession{ID: domain.ModeID("cs", live), MerchantID: m.ID, Livemode: live, Amount: amount, Currency: "MZN", Status: domain.SessionOpen,
			ExpiresAt: when.Add(time.Hour), PaymentMethodTypes: domain.AllMethods, Locale: "pt-MZ", CreatedAt: when}
		switch method {
		case domain.MethodCard:
			p.CardBrand, p.CardLast4 = []string{"Visa", "Mastercard"}[rng.Intn(2)], fmt.Sprintf("%04d", rng.Intn(10000))
		case domain.MethodMpesa:
			p.PhonePrefix, p.PhoneLast4 = []string{"84", "85"}[rng.Intn(2)], fmt.Sprintf("%04d", 1000+rng.Intn(8999))
		default:
			p.PhonePrefix, p.PhoneLast4 = []string{"86", "87"}[rng.Intn(2)], fmt.Sprintf("%04d", 1000+rng.Intn(8999))
		}
		if src == domain.SourceAPI {
			order++
			p.CustomerName = fmt.Sprintf("Encomenda #%d", order)
			p.ClientReferenceID = fmt.Sprintf("pedido_%d", order)
			p.Description = p.CustomerName
			p.Metadata = map[string]string{"order_id": fmt.Sprint(order), "canal": "loja_online", "entrega": "cidade_maputo"}
			p.APIKeyID = &keyID
			p.Origin.Domain, p.Origin.IP, p.Origin.UserAgent = "loja.cafemaputo.co.mz", "41.220.12.88", "WooCommerce/8.9 riha-woo/1.2.0"
			cs.CallbackURL = "https://loja.cafemaputo.co.mz/webhooks/riha"
			cs.SuccessURL = "https://loja.cafemaputo.co.mz/ok?s={CHECKOUT_SESSION_ID}"
			cs.ClientReferenceID = p.ClientReferenceID
		} else {
			p.CustomerName = names[rng.Intn(len(names))]
			p.PaymentLinkID = &link.ID
			p.Description = link.Name
			p.CreatedBy = u.ID
			p.Channel = []string{"whatsapp", "whatsapp", "sms", "outro"}[rng.Intn(4)]
			if src == domain.SourceQR {
				p.Channel = "qr"
			}
			cs.SuccessURL = m.SuccessURL
		}
		cs.Origin = p.Origin
		cs.Description = p.Description
		p.SessionID = cs.ID
		switch status {
		case domain.PaymentSucceeded:
			t := when.Add(40 * time.Second)
			p.SucceededAt, p.OperatorReference = &t, domain.ShortCode(10)
			cs.Status, cs.PaymentID, cs.CompletedAt = domain.SessionComplete, &p.ID, &t
		case domain.PaymentFailed:
			t := when.Add(50 * time.Second)
			p.FailedAt, p.FailureCode, p.FailureMessage = &t, domain.FailPinIncorrect, domain.FailureMessages[domain.FailPinIncorrect]
			cs.Status = domain.SessionExpired
		case domain.PaymentProcessing:
			cs.ExpiresAt = now.Add(50 * time.Minute)
		}
		s.do(a.Sessions.Create(ctx, cs))
		s.do(a.Payments.Create(ctx, p))
		if status != domain.PaymentSucceeded {
			return
		}
		s.do(a.Ledger.Add(ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: m.ID, Livemode: live, Type: domain.LedgerPayment, Amount: p.Net, ReferenceID: p.ID, Description: "Pagamento " + p.Reference, CreatedAt: when}))
		if src == domain.SourceAPI {
			ev := &domain.Event{ID: domain.NewID("evt"), MerchantID: m.ID, Livemode: live, Type: "checkout.session.completed", ObjectID: cs.ID, Data: []byte("{}"), CreatedAt: when}
			s.do(a.Events.Create(ctx, ev))
			s.do(a.Deliveries.Create(ctx, &domain.Delivery{ID: domain.NewID("whd"), EventID: ev.ID, MerchantID: m.ID, Livemode: live, EventType: ev.Type, Target: domain.TargetCallback,
				URL: cs.CallbackURL, Status: domain.DeliverySucceeded, Attempts: 1, LastStatusCode: 200, LastDurationMs: 182, CreatedAt: when, UpdatedAt: when}))
		}
	}
	pool := []*domain.PaymentLink{bolo, cabaz, cabaz, workshop, gorjeta, bolo}
	for d := 40; d >= 0; d-- {
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -d)
		count := 2 + rng.Intn(5)
		if d == 0 {
			count = 3
		}
		for i := 0; i < count; i++ {
			when := dayStart.Add(time.Duration(8*3600+rng.Intn(11*3600)) * time.Second)
			if when.After(now) {
				when = now.Add(-time.Duration(10+rng.Intn(100)) * time.Minute)
			}
			r := rng.Float64()
			method := domain.MethodMpesa
			if r > 0.58 && r <= 0.85 {
				method = domain.MethodEmola
			} else if r > 0.85 {
				method = domain.MethodCard
			}
			status := domain.PaymentSucceeded
			if rng.Float64() < 0.06 {
				status, method = domain.PaymentFailed, domain.MethodMpesa
			}
			src, link := domain.SourceLink, pool[rng.Intn(len(pool))]
			var amount int64
			switch {
			case rng.Float64() < 0.22:
				src = domain.SourceAPI
				method = []string{domain.MethodCard, domain.MethodCard, domain.MethodMpesa}[rng.Intn(3)]
				amount = int64(150000 + rng.Intn(500000))
				amount -= amount % 1000
			case rng.Float64() < 0.12:
				src, link = domain.SourceQR, balcao
				amount = int64(30000 + rng.Intn(120000))
				amount -= amount % 1000
			default:
				amount = link.Amount
				if link.CustomAmount {
					amount = int64(10000 + rng.Intn(50000))
					amount -= amount % 1000
				}
			}
			add(when, amount, method, status, src, link, true)
		}
	}
	add(now.Add(-7*time.Minute), 78000, domain.MethodEmola, domain.PaymentProcessing, domain.SourceLink, cabaz, true)
	for i := 0; i < 6; i++ {
		add(now.Add(-time.Duration(i*5+1)*time.Hour), 125000, domain.AllMethods[i%3], domain.PaymentSucceeded, []string{domain.SourceLink, domain.SourceAPI}[i%2], testLink, false)
	}
	for _, x := range []struct {
		amount int64
		age    int
		status string
	}{{1200000, 24 * 28, domain.PayoutPaid}, {1400000, 24 * 21, domain.PayoutPaid}, {1800000, 24 * 14, domain.PayoutPaid}, {2000000, 24 * 7, domain.PayoutPaid}, {1500000, 44, domain.PayoutReview}} {
		s.payout(m.ID, x.amount, x.age, x.status, "BCI", "Café Maputo, Lda", nib)
	}
}

type payoutSeed struct {
	amount int64
	age    int
	status string
}

func (s *seeder) others() {
	ctx, a, now, rng := s.ctx, s.a, s.now, s.rng
	others := []struct {
		name, city, bank, nib, main string
		vol                         int64
		payouts                     []payoutSeed
	}{
		{"Txopela Rides", "Maputo", "Millennium bim", "000100007731000012345", "mpesa", 184030000, []payoutSeed{{12000000, 63, domain.PayoutReview}, {9800000, 150, domain.PayoutPaid}}},
		{"Kanimambo Store", "Matola", "BCI", "000800001190000054321", "card", 64020000, []payoutSeed{{8450000, 66, domain.PayoutReview}, {6120000, 160, domain.PayoutPaid}}},
		{"Mercado Online MZ", "Maputo", "Standard Bank", "000300002208000011111", "card", 51280000, []payoutSeed{{4620000, 39, domain.PayoutNew}}},
		{"Farmácia Polana", "Maputo", "Absa", "000200005520000022222", "mpesa", 29840000, []payoutSeed{{3200000, 25, domain.PayoutNew}}},
		{"Mahala Eventos", "Beira", "Moza Banco", "003400000042000033333", "emola", 12090000, []payoutSeed{{1850000, 19, domain.PayoutNew}}},
		{"Livraria Sumburero", "Maputo", "BCI", "000800006634000044444", "mpesa", 3400000, []payoutSeed{{740000, 3, domain.PayoutNew}}},
		{"Pastelaria Nkomati", "Maputo", "Millennium bim", "000100003150000055555", "mpesa", 9100000, []payoutSeed{{2230000, 30, domain.PayoutSent}}},
	}
	for i, o := range others {
		slug := domain.Slugify(o.name)
		om := &domain.Merchant{ID: domain.NewID("acct"), Slug: slug, Name: o.name, LegalName: o.name + ", Lda", BusinessType: "company", NUIT: fmt.Sprintf("40%07d", 1000000+i*7919),
			City: o.city, Email: "financas@" + slug + ".co.mz", Phone: fmt.Sprintf("+25884%07d", 3000000+i*1117), OwnerName: names[i], Status: domain.MerchantActive,
			AvatarColor: []string{"#E2F3DE", "#FFD21F", "#EDECE6", "#FFF0D6"}[i%4], SigningSecretTest: domain.Token("whsec_", 32), SigningSecretLive: domain.Token("whsec_", 32),
			ApprovedAt: ptr(now.AddDate(0, -(i + 2), 0)), CreatedAt: now.AddDate(0, -(i + 2), -3), StatementDescriptor: domain.Descriptor(o.name)}
		if i == 0 {
			om.FeeBps = ptr(450)
		}
		s.do(a.Merchants.Create(ctx, om))
		s.do(a.Banks.Save(ctx, &domain.BankAccount{ID: domain.NewID("ba"), MerchantID: om.ID, BankName: o.bank, Holder: om.LegalName, NIB: o.nib, Verified: true, UpdatedAt: now}))
		fee := a.FeeBpsFor(ctx, om)
		for k := 0; k < 8; k++ {
			amt := o.vol / 8
			when := now.Add(-time.Duration(k*80+rng.Intn(60)) * time.Hour)
			p := &domain.Payment{ID: domain.NewID("pay"), Reference: domain.PaymentReference(), MerchantID: om.ID, Livemode: true, Status: domain.PaymentSucceeded, Amount: amt, FeeBps: fee,
				Method: o.main, CustomerName: "Cliente", CreatedAt: when, SucceededAt: &when, SessionID: "cs_live_seed"}
			p.SourceType = domain.SourceAPI
			p.Fee, p.Net = domain.Split(amt, fee)
			s.do(a.Payments.Create(ctx, p))
			s.do(a.Ledger.Add(ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: om.ID, Livemode: true, Type: domain.LedgerPayment, Amount: p.Net, ReferenceID: p.ID, CreatedAt: when}))
		}
		for _, po := range o.payouts {
			s.payout(om.ID, po.amount, po.age, po.status, o.bank, om.LegalName, o.nib)
		}
	}
	s.do(a.Merchants.Create(ctx, &domain.Merchant{ID: domain.NewID("acct"), Slug: "lojas-xiluva", Name: "Lojas Xiluva", City: "Inhambane", Status: domain.MerchantSuspended, AvatarColor: "#EDECE6",
		SigningSecretTest: domain.Token("whsec_", 32), SigningSecretLive: domain.Token("whsec_", 32), CreatedAt: now.AddDate(0, -6, 0), ReviewNote: "Picos de volume suspeitos."}))
}

func (s *seeder) applicants() {
	ctx, a, now, rng := s.ctx, s.a, s.now, s.rng
	apps := []struct {
		name, legal, owner, typ, nuit, city, phone, email, vol, bank, holder, nib, color string
		ageH                                                                             int
	}{
		{"Mercearia Tembe", "Mercearia Tembe", "Helena Tembe", "individual", "401552019", "Matola", "+258856604412", "helena@merceariatembe.co.mz", "50 – 150 mil MT/mês", "Millennium bim", "Helena Tembe", "000100003301000077777", "#FFD21F", 2},
		{"Txova Entregas", "Txova Entregas, Lda", "Rui Macuácua", "company", "400873215", "Maputo", "+258843012290", "rui@txova.co.mz", "300 – 600 mil MT/mês", "BCI", "Rui Macuácua", "000800007731200088888", "#E2F3DE", 5},
		{"Loja da Graça", "Loja da Graça", "Graça Mabunda", "individual", "402118640", "Beira", "+258865540193", "graca.mabunda@gmail.com", "Até 50 mil MT/mês", "BCI", "Graça Mabunda", "000800000921400099999", "#FFF0D6", 26},
		{"Ndzava Moda", "Ndzava Moda", "Lúcia Ndzava", "individual", "409000117", "Nampula", "+258872208817", "ndzavamoda@gmail.com", "150 – 300 mil MT/mês", "FNB", "L. Ndzava", "001900008817000012121", "#FFE3E1", 30},
		{"Escola de Condução Xai-Xai", "Escola de Condução Xai-Xai, Lda", "Abel Sitoe", "company", "400665302", "Xai-Xai", "+258847781120", "geral@ecxaixai.co.mz", "50 – 150 mil MT/mês", "Moza Banco", "Escola de Condução Xai-Xai, Lda", "003400004410900013131", "#EDECE6", 50},
		{"Kuxa Tech", "Kuxa Tech, SA", "Inês Matsinhe", "company", "400120884", "Maputo", "+258849001002", "financas@kuxatech.co.mz", "Mais de 1 M MT/mês", "Standard Bank", "Kuxa Tech, SA", "000300001002600014141", "#FFD21F", 72},
	}
	for _, l := range []struct {
		name, owner, email, phone, city string
		ageH                            int
	}{
		{"Padaria Zambeze", "Marta Uamusse", "marta@padariazambeze.co.mz", "+258845550101", "Tete", 4},
		{"Tchova Moda", "Sérgio Langa", "sergio.langa@gmail.com", "+258865550202", "Maputo", 20},
	} {
		created := now.Add(-time.Duration(l.ageH) * time.Hour)
		m := &domain.Merchant{ID: domain.NewID("acct"), Slug: domain.Slugify(l.name), Name: l.name, OwnerName: l.owner, Email: l.email, Phone: l.phone, City: l.city,
			Status: domain.MerchantLead, AvatarColor: "#E2F3DE", StatementDescriptor: domain.Descriptor(l.name),
			SigningSecretTest: domain.Token("whsec_", 32), SigningSecretLive: domain.Token("whsec_", 32), CreatedAt: created}
		s.do(a.Merchants.Create(ctx, m))
		hash, err := a.Hasher.Hash(domain.Token("", 20))
		s.do(err)
		s.do(a.Users.Create(ctx, &domain.User{ID: domain.NewID("usr"), Email: l.email, PasswordHash: hash, Name: l.owner, Role: domain.RoleMerchant, MerchantID: &m.ID, CreatedAt: created}))
	}
	for _, x := range apps {
		sub := now.Add(-time.Duration(x.ageH) * time.Hour)
		m := &domain.Merchant{ID: domain.NewID("acct"), Slug: domain.Slugify(x.name), Name: x.name, LegalName: x.legal, BusinessType: x.typ, NUIT: x.nuit, City: x.city,
			Phone: x.phone, Email: x.email, OwnerName: x.owner, ExpectedVolume: x.vol, AvatarColor: x.color, Status: domain.MerchantInReview, KycSubmittedAt: &sub,
			SigningSecretTest: domain.Token("whsec_", 32), SigningSecretLive: domain.Token("whsec_", 32), CreatedAt: sub.Add(-time.Hour)}
		s.do(a.Merchants.Create(ctx, m))
		hash, err := a.Hasher.Hash(domain.Token("", 20))
		s.do(err)
		s.do(a.Users.Create(ctx, &domain.User{ID: domain.NewID("usr"), Email: x.email, PasswordHash: hash, Name: x.owner, Role: domain.RoleMerchant, MerchantID: &m.ID, CreatedAt: sub}))
		s.do(a.Banks.Save(ctx, &domain.BankAccount{ID: domain.NewID("ba"), MerchantID: m.ID, BankName: x.bank, Holder: x.holder, NIB: x.nib, UpdatedAt: now}))
		files := map[string]string{"id_front": "bi-frente.jpg", "id_back": "bi-verso.jpg", "registration": domain.Slugify(x.name) + ".pdf", "nib_proof": "nib.pdf"}
		for _, k := range []string{"id_front", "id_back", "registration", "nib_proof"} {
			if x.name == "Ndzava Moda" && k == "registration" {
				continue
			}
			s.do(a.Documents.Create(ctx, &domain.Document{ID: domain.NewID("doc"), MerchantID: m.ID, Kind: k, FileName: files[k], Size: int64(120000 + rng.Intn(900000)), ContentType: "image/jpeg", CreatedAt: sub}))
		}
	}
}
