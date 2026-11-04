// Package seed creates demo data for a fresh database: a super admin, the
// Café Maputo merchant with a few weeks of history, an approval queue and a
// payout queue.
package seed

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"riha/internal/ids"
	"riha/internal/models"
	"riha/internal/money"
	"riha/internal/service"
)

const (
	AdminEmail       = "admin@riha.co.mz"
	AdminPassword    = "riha-admin-2026"
	MerchantEmail    = "ola@cafemaputo.co.mz"
	MerchantPassword = "cafe-maputo-2026"
)

func Run(s *service.Service) error {
	var n int64
	s.DB.Model(&models.User{}).Count(&n)
	if n > 0 {
		return nil
	}
	log.Printf("seed: creating demo data")
	rng := rand.New(rand.NewSource(42))
	now := time.Now().UTC()

	hash, _ := service.HashPassword(AdminPassword)
	if err := s.DB.Create(&models.User{ID: ids.New("usr"), Email: AdminEmail, PasswordHash: hash, Name: "Nélia Chissano", Role: models.RoleAdmin, AdminTitle: "Operações"}).Error; err != nil {
		return err
	}
	s.SetDefaultFeeBps(money.DefaultFeeBps)

	// --- Café Maputo (the demo merchant)
	u, m, err := s.Signup(service.SignupInput{BusinessName: "Café Maputo", Name: "Ana Mabota", Email: MerchantEmail, Phone: "841234567", Password: MerchantPassword})
	if err != nil {
		return err
	}
	approved := now.AddDate(0, 0, -20)
	submitted := approved.Add(-20 * time.Hour)
	if err := s.DB.Model(m).Updates(map[string]any{"legal_name": "Café Maputo, Lda", "business_type": "company", "nuit": "400123456", "city": "Maputo",
		"expected_volume": "50 – 150 mil MT/mês", "status": models.MerchantActive, "approved_at": approved, "kyc_submitted_at": submitted,
		"success_url": "https://cafemaputo.co.mz/obrigado", "cancel_url": "https://cafemaputo.co.mz/carrinho", "avatar_color": "#FFD21F",
		"statement_descriptor": "RIHA*CAFE MAPUTO", "created_at": approved.AddDate(0, 0, -2)}).Error; err != nil {
		return err
	}
	s.DB.First(m, "id = ?", m.ID)
	s.DB.Create(&models.BankAccount{ID: ids.New("ba"), MerchantID: m.ID, BankName: "BCI", Holder: "Café Maputo, Lda", NIB: "000800004821130101135", Verified: true})
	for _, d := range []string{"cafemaputo.co.mz", "loja.cafemaputo.co.mz"} {
		s.AddDomain(m.ID, d)
	}
	for _, live := range []bool{true, false} {
		s.DB.Create(&models.WebhookEndpoint{ID: ids.New("we"), MerchantID: m.ID, Livemode: live, URL: "https://cafemaputo.co.mz/api/riha",
			Events: models.StringList{"payment.succeeded", "payment.failed", "payout.paid"}, Active: false})
	}
	// Webhook endpoints are seeded disabled because the demo domains do not exist.

	mk := func(name string, amount int64, custom bool, kind, loc string, live bool, age int) *models.PaymentLink {
		l := &models.PaymentLink{ID: ids.New("plink"), MerchantID: m.ID, Livemode: live, Name: name, Slug: service.Slugify(name), Amount: amount, CustomAmount: custom,
			Reusable: true, Methods: models.StringList{"mpesa", "emola", "card"}, Active: true, Kind: kind, CreatedBy: u.ID, CreatedVia: "dashboard", Location: loc,
			CreatedAt: now.AddDate(0, 0, -age)}
		if !live {
			l.Slug += "-teste"
		}
		s.DB.Create(l)
		return l
	}
	bolo := mk("Bolo de chocolate · 1 kg", 125000, false, "link", "", true, 18)
	bolo.Slug = "bolo"
	s.DB.Model(bolo).Update("slug", "bolo")
	cabaz := mk("Cabaz de café moído", 89000, false, "link", "", true, 17)
	workshop := mk("Workshop de barista · 18 nov", 250000, false, "link", "", true, 10)
	gorjeta := mk("Gorjeta", 0, true, "link", "", true, 15)
	balcao := mk("Balcão Sommerschield", 0, true, "qr", "Loja da Sommerschield", true, 19)
	natal := mk("Encomenda de Natal", 450000, false, "link", "", true, 2)
	exp := now.AddDate(0, 1, 20)
	s.DB.Model(natal).Update("expires_at", exp)
	promo := mk("Promoção de outubro", 60000, false, "link", "", true, 35)
	s.DB.Model(promo).Updates(map[string]any{"active": false})
	testLink := mk("Bolo de chocolate · 1 kg", 125000, false, "link", "", false, 10)
	_ = testLink

	keyID := "key_3f9a"
	s.DB.Create(&models.APIKey{ID: keyID, MerchantID: m.ID, Livemode: true, Kind: models.KeySecret, Hash: ids.Hash(ids.Token("sk_live_", 32)), Last4: "3f9a", CreatedBy: u.ID, CreatedAt: approved.Add(time.Hour)})

	names := []string{"Ana Machava", "Carlos Nhantumbo", "Fátima Cossa", "Júlio Sitoe", "Rosa Langa", "Tomás Muianga", "Helena Tembe", "Inês Matsinhe", "Paulo Chissano", "Lurdes Mondlane", "Edson Bila", "Célia Nhaca"}
	bps := s.FeeBpsFor(m)
	order := 2060
	addPayment := func(when time.Time, amount int64, method, status, src string, link *models.PaymentLink, live bool) {
		p := models.Payment{ID: ids.New("pay"), Reference: ids.PaymentReference(), MerchantID: m.ID, Livemode: live, Status: status, Amount: amount, FeeBps: bps, Method: method, CreatedAt: when, UpdatedAt: when, SourceType: src, Metadata: models.JSONMap{}}
		p.Fee, p.Net = money.Split(amount, bps)
		cs := models.CheckoutSession{ID: ids.Moded("cs", live), MerchantID: m.ID, Livemode: live, Amount: amount, Currency: "MZN", Status: models.SessionOpen,
			ExpiresAt: when.Add(time.Hour), SourceType: src, PaymentMethodTypes: models.StringList{"mpesa", "emola", "card"}, Metadata: models.JSONMap{}, Locale: "pt-MZ", CreatedAt: when, UpdatedAt: when}
		switch method {
		case "card":
			p.CardBrand, p.CardLast4 = []string{"Visa", "Mastercard"}[rng.Intn(2)], fmt.Sprintf("%04d", rng.Intn(10000))
		case "mpesa":
			p.PhonePrefix, p.PhoneLast4 = []string{"84", "85"}[rng.Intn(2)], fmt.Sprintf("%04d", 1000+rng.Intn(8999))
		default:
			p.PhonePrefix, p.PhoneLast4 = []string{"86", "87"}[rng.Intn(2)], fmt.Sprintf("%04d", 1000+rng.Intn(8999))
		}
		if src == models.SourceAPI {
			order++
			p.CustomerName = fmt.Sprintf("Encomenda #%d", order)
			p.ClientReferenceID = fmt.Sprintf("pedido_%d", order)
			p.Description = p.CustomerName
			p.Metadata = models.JSONMap{"order_id": fmt.Sprint(order), "canal": "loja_online", "entrega": "cidade_maputo"}
			p.APIKeyID = &keyID
			p.OriginDomain, p.OriginIP, p.OriginUserAgent = "loja.cafemaputo.co.mz", "41.220.12.88", "WooCommerce/8.9 riha-woo/1.2.0"
			cs.CallbackURL = "https://loja.cafemaputo.co.mz/webhooks/riha"
			cs.SuccessURL = "https://loja.cafemaputo.co.mz/ok?s={CHECKOUT_SESSION_ID}"
			cs.ClientReferenceID, cs.APIKeyID = p.ClientReferenceID, &keyID
			cs.OriginDomain, cs.OriginIP, cs.OriginUserAgent = p.OriginDomain, p.OriginIP, p.OriginUserAgent
		} else {
			p.CustomerName = names[rng.Intn(len(names))]
			p.PaymentLinkID = &link.ID
			p.Description = link.Name
			p.CreatedBy = u.ID
			p.Channel = []string{"whatsapp", "whatsapp", "sms", "outro"}[rng.Intn(4)]
			if src == models.SourceQR {
				p.Channel = "qr"
			}
			cs.PaymentLinkID, cs.Channel, cs.CreatedBy, cs.Description = &link.ID, p.Channel, u.ID, link.Name
			cs.SuccessURL = m.SuccessURL
		}
		cs.Description = firstNon(cs.Description, p.Description)
		p.SessionID = cs.ID
		switch status {
		case models.PaymentSucceeded:
			t := when.Add(40 * time.Second)
			p.SucceededAt = &t
			p.OperatorReference = ids.Short(10)
			cs.Status, cs.PaymentID, cs.CompletedAt = models.SessionComplete, &p.ID, &t
		case models.PaymentFailed:
			t := when.Add(50 * time.Second)
			p.FailedAt, p.FailureCode = &t, "pin_incorrect"
			p.FailureMessage = "PIN errado três vezes. O cliente pode tentar de novo com o mesmo link."
			cs.Status = models.SessionExpired
		case models.PaymentProcessing:
			cs.ExpiresAt = now.Add(50 * time.Minute)
		}
		s.DB.Create(&cs)
		s.DB.Create(&p)
		if status == models.PaymentSucceeded {
			s.DB.Create(&models.LedgerEntry{ID: ids.New("le"), MerchantID: m.ID, Livemode: live, Type: models.LedgerPayment, Amount: p.Net, ReferenceID: p.ID, Description: "Pagamento " + p.Reference, CreatedAt: when})
			if src == models.SourceAPI {
				ev := models.Event{ID: ids.New("evt"), MerchantID: m.ID, Livemode: live, Type: "checkout.session.completed", ObjectID: cs.ID, Data: "{}", CreatedAt: when}
				s.DB.Create(&ev)
				s.DB.Create(&models.WebhookDelivery{ID: ids.New("whd"), EventID: ev.ID, MerchantID: m.ID, Livemode: live, EventType: ev.Type, Target: models.TargetCallback,
					URL: cs.CallbackURL, Status: models.DeliverySucceeded, Attempts: 1, LastStatusCode: 200, LastDurationMs: 182, CreatedAt: when, UpdatedAt: when})
			}
		}
	}
	links := []*models.PaymentLink{bolo, cabaz, cabaz, workshop, gorjeta, bolo}
	for d := 40; d >= 0; d-- {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -d)
		count := 2 + rng.Intn(5)
		if d == 0 {
			count = 3
		}
		for i := 0; i < count; i++ {
			when := day.Add(time.Duration(8*3600+rng.Intn(11*3600)) * time.Second)
			if when.After(now) {
				when = now.Add(-time.Duration(10+rng.Intn(100)) * time.Minute)
			}
			r := rng.Float64()
			method := "mpesa"
			if r > 0.58 && r <= 0.85 {
				method = "emola"
			} else if r > 0.85 {
				method = "card"
			}
			status := models.PaymentSucceeded
			if rng.Float64() < 0.06 {
				status = models.PaymentFailed
				method = "mpesa"
			}
			src := models.SourceLink
			link := links[rng.Intn(len(links))]
			var amount int64
			switch {
			case rng.Float64() < 0.22:
				src = models.SourceAPI
				method = []string{"card", "card", "mpesa"}[rng.Intn(3)]
				amount = int64(150000 + rng.Intn(500000))
				amount -= amount % 1000
			case rng.Float64() < 0.12:
				src = models.SourceQR
				link = balcao
				amount = int64(30000 + rng.Intn(120000))
				amount -= amount % 1000
			default:
				amount = link.Amount
				if link.CustomAmount {
					amount = int64(10000 + rng.Intn(50000))
					amount -= amount % 1000
				}
			}
			addPayment(when, amount, method, status, src, link, true)
		}
	}
	addPayment(now.Add(-7*time.Minute), 78000, "emola", models.PaymentProcessing, models.SourceLink, cabaz, true)
	for i := 0; i < 6; i++ {
		addPayment(now.Add(-time.Duration(i*5+1)*time.Hour), 125000, []string{"mpesa", "card", "emola"}[i%3], models.PaymentSucceeded, []string{models.SourceLink, models.SourceAPI}[i%2], testLink, false)
	}

	// Café Maputo payout history.
	addPayout := func(mid string, amount int64, ageH int, status string, bank, holder, nib string) {
		var num int64
		s.DB.Raw("SELECT nextval('payout_number_seq')").Scan(&num)
		req := now.Add(-time.Duration(ageH) * time.Hour)
		p := models.Payout{ID: ids.New("po"), Number: num, MerchantID: mid, Livemode: true, Amount: amount, Status: status, BankName: bank, Holder: holder, NIB: nib,
			RequestedAt: req, DueAt: req.Add(service.PayoutSLA), CreatedAt: req, UpdatedAt: req, RequestedBy: "seed"}
		if status == models.PayoutSent || status == models.PayoutPaid {
			t := req.Add(20 * time.Hour)
			p.SentAt, p.ReviewedAt = &t, &t
		}
		if status == models.PayoutPaid {
			t := req.Add(50 * time.Hour)
			p.PaidAt, p.ClosedAt = &t, &t
		}
		s.DB.Create(&p)
		s.DB.Create(&models.LedgerEntry{ID: ids.New("le"), MerchantID: mid, Livemode: true, Type: models.LedgerPayout, Amount: -amount, ReferenceID: p.ID, Description: "Levantamento " + service.PayoutRef(&p), CreatedAt: req})
	}
	nibCM := "000800004821130101135"
	addPayout(m.ID, 1200000, 24*28, models.PayoutPaid, "BCI", "Café Maputo, Lda", nibCM)
	addPayout(m.ID, 1400000, 24*21, models.PayoutPaid, "BCI", "Café Maputo, Lda", nibCM)
	addPayout(m.ID, 1800000, 24*14, models.PayoutPaid, "BCI", "Café Maputo, Lda", nibCM)
	addPayout(m.ID, 2000000, 24*7, models.PayoutPaid, "BCI", "Café Maputo, Lda", nibCM)
	addPayout(m.ID, 1500000, 44, models.PayoutReview, "BCI", "Café Maputo, Lda", nibCM)

	// --- Other active merchants with live balances and payouts in the queue.
	others := []struct {
		name, city, bank, nib, main string
		vol                         int64
		payouts                     [][3]any // amount, age hours, status
	}{
		{"Txopela Rides", "Maputo", "Millennium bim", "000100007731000012345", "mpesa", 184030000, [][3]any{{int64(12000000), 63, models.PayoutReview}, {int64(9800000), 150, models.PayoutPaid}}},
		{"Kanimambo Store", "Matola", "BCI", "000800001190000054321", "card", 64020000, [][3]any{{int64(8450000), 66, models.PayoutReview}, {int64(6120000), 160, models.PayoutPaid}}},
		{"Mercado Online MZ", "Maputo", "Standard Bank", "000300002208000011111", "card", 51280000, [][3]any{{int64(4620000), 39, models.PayoutNew}}},
		{"Farmácia Polana", "Maputo", "Absa", "000200005520000022222", "mpesa", 29840000, [][3]any{{int64(3200000), 25, models.PayoutNew}}},
		{"Mahala Eventos", "Beira", "Moza Banco", "003400000042000033333", "emola", 12090000, [][3]any{{int64(1850000), 19, models.PayoutNew}}},
		{"Livraria Sumburero", "Maputo", "BCI", "000800006634000044444", "mpesa", 3400000, [][3]any{{int64(740000), 3, models.PayoutNew}}},
		{"Pastelaria Nkomati", "Maputo", "Millennium bim", "000100003150000055555", "mpesa", 9100000, [][3]any{{int64(2230000), 30, models.PayoutSent}}},
	}
	for i, o := range others {
		om := models.Merchant{ID: ids.New("acct"), Slug: service.Slugify(o.name), Name: o.name, LegalName: o.name + ", Lda", BusinessType: "company", NUIT: fmt.Sprintf("40%07d", 1000000+i*7919),
			City: o.city, Email: "financas@" + service.Slugify(o.name) + ".co.mz", Phone: "+25884" + fmt.Sprintf("%07d", 3000000+i*1117), OwnerName: names[i], Status: models.MerchantActive,
			AvatarColor: []string{"#E2F3DE", "#FFD21F", "#EDECE6", "#FFF0D6"}[i%4], SigningSecretTest: ids.Token("whsec_", 32), SigningSecretLive: ids.Token("whsec_", 32),
			ApprovedAt: ptr(now.AddDate(0, -(i + 2), 0)), CreatedAt: now.AddDate(0, -(i + 2), -3), StatementDescriptor: "RIHA*" + service.Slugify(o.name)}
		if i == 0 {
			v := 450 // negotiated: Txopela Rides pays 4,5%
			om.FeeBps = &v
		}
		s.DB.Create(&om)
		s.DB.Create(&models.BankAccount{ID: ids.New("ba"), MerchantID: om.ID, BankName: o.bank, Holder: om.LegalName, NIB: o.nib, Verified: true})
		feeBps := s.FeeBpsFor(&om)
		// Aggregate history as a handful of large payments.
		var credited int64
		for k := 0; k < 8; k++ {
			amt := o.vol / 8
			when := now.Add(-time.Duration(k*80+rng.Intn(60)) * time.Hour)
			p := models.Payment{ID: ids.New("pay"), Reference: ids.PaymentReference(), MerchantID: om.ID, Livemode: true, Status: models.PaymentSucceeded, Amount: amt, FeeBps: feeBps,
				Method: o.main, SourceType: models.SourceAPI, CustomerName: "Cliente", CreatedAt: when, SucceededAt: &when, SessionID: "cs_live_seed", Metadata: models.JSONMap{}}
			p.Fee, p.Net = money.Split(amt, feeBps)
			s.DB.Create(&p)
			s.DB.Create(&models.LedgerEntry{ID: ids.New("le"), MerchantID: om.ID, Livemode: true, Type: models.LedgerPayment, Amount: p.Net, ReferenceID: p.ID, CreatedAt: when})
			credited += p.Net
		}
		for _, po := range o.payouts {
			addPayout(om.ID, po[0].(int64), po[1].(int), po[2].(string), o.bank, om.LegalName, o.nib)
		}
	}
	// A suspended merchant.
	s.DB.Create(&models.Merchant{ID: ids.New("acct"), Slug: "lojas-xiluva", Name: "Lojas Xiluva", City: "Inhambane", Status: models.MerchantSuspended, AvatarColor: "#EDECE6",
		SigningSecretTest: ids.Token("whsec_", 32), SigningSecretLive: ids.Token("whsec_", 32), CreatedAt: now.AddDate(0, -6, 0), ReviewNote: "Picos de volume suspeitos."})

	// --- Approval queue.
	apps := []struct {
		name, owner, typ, nuit, city, phone, email, vol, bank, holder, nib, color string
		ageH                                                                      int
	}{
		{"Mercearia Tembe", "Helena Tembe", "individual", "401552019", "Matola", "+258856604412", "helena@merceariatembe.co.mz", "50 – 150 mil MT/mês", "Millennium bim", "Helena Tembe", "000100003301000077777", "#FFD21F", 2},
		{"Txova Entregas", "Rui Macuácua", "company", "400873215", "Maputo", "+258843012290", "rui@txova.co.mz", "300 – 600 mil MT/mês", "BCI", "Rui Macuácua", "000800007731200088888", "#E2F3DE", 5},
		{"Loja da Graça", "Graça Mabunda", "individual", "402118640", "Beira", "+258865540193", "graca.mabunda@gmail.com", "Até 50 mil MT/mês", "BCI", "Graça Mabunda", "000800000921400099999", "#FFF0D6", 26},
		{"Ndzava Moda", "Lúcia Ndzava", "individual", "409000117", "Nampula", "+258872208817", "ndzavamoda@gmail.com", "150 – 300 mil MT/mês", "FNB", "L. Ndzava", "001900008817000012121", "#FFE3E1", 30},
		{"Escola de Condução Xai-Xai", "Abel Sitoe", "company", "400665302", "Xai-Xai", "+258847781120", "geral@ecxaixai.co.mz", "50 – 150 mil MT/mês", "Moza Banco", "Escola de Condução Xai-Xai, Lda", "003400004410900013131", "#EDECE6", 50},
		{"Kuxa Tech", "Inês Matsinhe", "company", "400120884", "Maputo", "+258849001002", "financas@kuxatech.co.mz", "Mais de 1 M MT/mês", "Standard Bank", "Kuxa Tech, SA", "000300001002600014141", "#FFD21F", 72},
	}
	for _, a := range apps {
		legal := a.name
		if a.typ == "company" && a.name != "Kuxa Tech" {
			legal = a.name + ", Lda"
		} else if a.name == "Kuxa Tech" {
			legal = "Kuxa Tech, SA"
		}
		sub := now.Add(-time.Duration(a.ageH) * time.Hour)
		am := models.Merchant{ID: ids.New("acct"), Slug: service.Slugify(a.name), Name: a.name, LegalName: legal, BusinessType: a.typ, NUIT: a.nuit, City: a.city,
			Phone: a.phone, Email: a.email, OwnerName: a.owner, ExpectedVolume: a.vol, AvatarColor: a.color, Status: models.MerchantInReview, KycSubmittedAt: &sub,
			SigningSecretTest: ids.Token("whsec_", 32), SigningSecretLive: ids.Token("whsec_", 32), CreatedAt: sub.Add(-time.Hour)}
		s.DB.Create(&am)
		hash, _ := service.HashPassword(ids.Token("", 20))
		s.DB.Create(&models.User{ID: ids.New("usr"), Email: a.email, PasswordHash: hash, Name: a.owner, Role: models.RoleMerchant, MerchantID: &am.ID})
		s.DB.Create(&models.BankAccount{ID: ids.New("ba"), MerchantID: am.ID, BankName: a.bank, Holder: a.holder, NIB: a.nib})
		kinds := []string{"id_front", "id_back", "registration", "nib_proof"}
		for _, k := range kinds {
			if a.name == "Ndzava Moda" && k == "registration" {
				continue
			}
			s.DB.Create(&models.KycDocument{ID: ids.New("doc"), MerchantID: am.ID, Kind: k, FileName: map[string]string{"id_front": "bi-frente.jpg", "id_back": "bi-verso.jpg", "registration": service.Slugify(a.name) + ".pdf", "nib_proof": "nib.pdf"}[k],
				StoredPath: "", Size: int64(120000 + rng.Intn(900000)), ContentType: "image/jpeg"})
		}
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

func firstNon(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

var _ = gorm.ErrRecordNotFound
