package service_test

import (
	"os"
	"testing"
	"time"

	"riha/internal/config"
	"riha/internal/db"
	"riha/internal/models"
	"riha/internal/providers"
	"riha/internal/service"
)

// Integration test against a real Postgres. Set RIHA_TEST_DATABASE_URL to a
// throwaway database, e.g. postgres://riha:riha@localhost:5432/riha_test.
func setup(t *testing.T) *service.Service {
	dsn := os.Getenv("RIHA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("RIHA_TEST_DATABASE_URL not set")
	}
	gdb, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range models.All() {
		gdb.Migrator().DropTable(m)
	}
	if err := db.Migrate(gdb); err != nil {
		t.Fatal(err)
	}
	mock := providers.NewMock(0)
	mock.After = func(_ time.Duration, f func()) { f() } // wallet confirms instantly
	return service.New(gdb, config.Config{PublicURL: "http://localhost:5173"}, mock)
}

func pay(t *testing.T, s *service.Service, m *models.Merchant, live bool, amount int64, phone string) *models.Payment {
	t.Helper()
	cs, err := s.CreateSession(m, live, service.SessionParams{Amount: amount, SuccessURL: "https://loja.co.mz/ok", SourceType: models.SourceAPI})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Pay(cs.ID, service.PayInput{Method: "mpesa", Phone: phone})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMoneyFlow(t *testing.T) {
	s := setup(t)
	_, m, err := s.Signup(service.SignupInput{BusinessName: "Loja Teste", Email: "a@b.co.mz", Phone: "841234567", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}

	// Default fee is 6%.
	p := pay(t, s, m, false, 125000, "841234231")
	if p.Status != models.PaymentSucceeded || p.Fee != 7500 || p.Net != 117500 {
		t.Fatalf("payment %+v", p)
	}
	if b := s.Balance(m.ID, false); b != 117500 {
		t.Fatalf("balance %d", b)
	}

	// Failure numbers do not credit anything.
	f := pay(t, s, m, false, 50000, "840000000")
	if f.Status != models.PaymentFailed || f.FailureCode != providers.FailPinIncorrect {
		t.Fatalf("expected pin_incorrect, got %+v", f)
	}
	if b := s.Balance(m.ID, false); b != 117500 {
		t.Fatalf("failed payment changed balance: %d", b)
	}

	// Negotiated fee applies to new payments only.
	bps := 350
	s.DB.Model(m).Update("fee_bps", bps)
	s.DB.First(m, "id = ?", m.ID)
	p2 := pay(t, s, m, false, 100000, "851234567")
	if p2.Fee != 3500 || p2.FeeBps != 350 {
		t.Fatalf("negotiated fee not applied: %+v", p2)
	}
	var old models.Payment
	s.DB.First(&old, "id = ?", p.ID)
	if old.Fee != 7500 {
		t.Fatal("old payment fee must not change")
	}
	bal := s.Balance(m.ID, false) // 117500 + 96500
	if bal != 214000 {
		t.Fatalf("balance %d", bal)
	}

	// Payouts: need a bank account, cannot exceed balance, reject restores.
	if _, err := s.RequestPayout(m.ID, false, 50000, "u"); err == nil {
		t.Fatal("payout without bank account must fail")
	}
	s.DB.Create(&models.BankAccount{ID: "ba_t", MerchantID: m.ID, BankName: "BCI", Holder: "Loja Teste", NIB: "000800004821130101135"})
	if _, err := s.RequestPayout(m.ID, false, bal+1, "u"); err == nil {
		t.Fatal("payout above balance must fail")
	}
	po, err := s.RequestPayout(m.ID, false, 100000, "u")
	if err != nil {
		t.Fatal(err)
	}
	if b := s.Balance(m.ID, false); b != bal-100000 {
		t.Fatalf("payout not debited: %d", b)
	}
	if _, err := s.TransitionPayout(po.ID, service.ActPaid, "", "admin"); err == nil {
		t.Fatal("cannot mark a new payout as paid")
	}
	if _, err := s.TransitionPayout(po.ID, service.ActReject, "teste", "admin"); err != nil {
		t.Fatal(err)
	}
	if b := s.Balance(m.ID, false); b != bal {
		t.Fatalf("rejected payout not restored: %d", b)
	}
	po2, _ := s.RequestPayout(m.ID, false, 20000, "u")
	s.TransitionPayout(po2.ID, service.ActApprove, "", "admin")
	if p, _ := s.TransitionPayout(po2.ID, service.ActPaid, "", "admin"); p.Status != models.PayoutPaid {
		t.Fatal("payout should be paid")
	}

	// Live payouts require an approved account.
	if _, err := s.RequestPayout(m.ID, true, 10000, "u"); err == nil {
		t.Fatal("live payout for unverified merchant must fail")
	}

	// Refunds: partial then full; cannot exceed remaining.
	before := s.Balance(m.ID, false)
	if _, err := s.Refund(m.ID, false, p.ID, 25000, "", "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refund(m.ID, false, p.ID, 200000, "", "u"); err == nil {
		t.Fatal("refund above remaining must fail")
	}
	if _, err := s.Refund(m.ID, false, p.ID, 0, "", "u"); err != nil {
		t.Fatal(err)
	}
	s.DB.First(&old, "id = ?", p.ID)
	if old.Status != models.PaymentRefunded || s.Balance(m.ID, false) != before-125000 {
		t.Fatalf("refund accounting wrong: %s %d", old.Status, s.Balance(m.ID, false))
	}

	// Each succeeded payment emits events.
	var n int64
	s.DB.Model(&models.Event{}).Where("merchant_id = ? AND type = ?", m.ID, "payment.succeeded").Count(&n)
	if n != 2 {
		t.Fatalf("events %d", n)
	}
}

func TestSessionValidation(t *testing.T) {
	s := setup(t)
	_, m, _ := s.Signup(service.SignupInput{BusinessName: "Loja V", Email: "v@b.co.mz", Phone: "841234567", Password: "segredo123"})
	cases := []service.SessionParams{
		{Amount: 999, SuccessURL: "https://x.co/ok"},
		{Amount: 5000, Currency: "USD", SuccessURL: "https://x.co/ok"},
		{Amount: 5000, SuccessURL: "https://x.co/ok", CallbackURL: "https://not-verified.co/cb"},
		{Amount: 5000, SuccessURL: "https://x.co/ok", LineItems: []models.LineItem{{Name: "a", Quantity: 1, UnitAmount: 4000}}},
		{Amount: 5000, SuccessURL: "https://x.co/ok", PaymentMethodTypes: []string{"paypal"}},
	}
	for i, c := range cases {
		c.SourceType = models.SourceAPI
		if _, err := s.CreateSession(m, false, c); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
	if _, err := s.AddDomain(m.ID, "https://loja.co.mz/x"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSession(m, false, service.SessionParams{Amount: 5000, SuccessURL: "https://loja.co.mz/ok", CallbackURL: "https://api.loja.co.mz/cb", SourceType: models.SourceAPI}); err != nil {
		t.Fatalf("subdomain of verified domain must be allowed: %v", err)
	}
}
