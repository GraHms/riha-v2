package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"riha/internal/adapters/crypto"
	"riha/internal/adapters/filestore"
	"riha/internal/adapters/postgres"
	"riha/internal/adapters/rails"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

type nullSender struct{}

func (nullSender) Send(context.Context, string, map[string]string, []byte) (int, error) {
	return 200, nil
}

func setup(t *testing.T) (*app.App, context.Context) {
	dsn := os.Getenv("RIHA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("RIHA_TEST_DATABASE_URL not set")
	}
	store, err := postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DropAll(); err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	mock := rails.NewMock(0)
	mock.After = func(_ time.Duration, f func()) { f() }
	r := store.Repositories()
	a := app.New(app.Deps{
		Tx: r.Tx, Users: r.Users, AuthSessions: r.AuthSessions, Merchants: r.Merchants, Documents: r.Documents, Banks: r.Banks,
		Keys: r.Keys, Links: r.Links, Sessions: r.Sessions, Payments: r.Payments, Refunds: r.Refunds, Ledger: r.Ledger,
		Payouts: r.Payouts, Endpoints: r.Endpoints, Domains: r.Domains, Events: r.Events, Deliveries: r.Deliveries,
		RequestLogs: r.RequestLogs, Idempotency: r.Idempotency, Audit: r.Audit, Settings: r.Settings, Reports: r.Reports,
		Rail: mock, Sender: nullSender{}, Files: filestore.Local{Root: t.TempDir()}, Hasher: crypto.Bcrypt{},
	}, "http://localhost:5173")
	return a, context.Background()
}

func pay(t *testing.T, a *app.App, ctx context.Context, m *domain.Merchant, amount int64, phone string) *domain.Payment {
	t.Helper()
	cs, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: amount, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: domain.MethodMpesa, Phone: phone})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMoneyFlow(t *testing.T) {
	a, ctx := setup(t)
	_, m, err := a.Signup(ctx, app.SignupInput{BusinessName: "Loja Teste", Email: "a@b.co.mz", Phone: "841234567", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}

	p := pay(t, a, ctx, m, 125000, "841234231")
	if p.Status != domain.PaymentSucceeded || p.Fee != 7500 || p.Net != 117500 {
		t.Fatalf("payment %+v", p)
	}
	if b := a.Balance(ctx, m.ID, false); b != 117500 {
		t.Fatalf("balance %d", b)
	}

	f := pay(t, a, ctx, m, 50000, "840000000")
	if f.Status != domain.PaymentFailed || f.FailureCode != domain.FailPinIncorrect {
		t.Fatalf("expected pin_incorrect, got %+v", f)
	}
	if b := a.Balance(ctx, m.ID, false); b != 117500 {
		t.Fatalf("failed payment changed balance: %d", b)
	}

	bps := 350
	if err := a.SetMerchantFee(ctx, m.ID, &bps, app.Actor{ID: "admin", Role: domain.RoleAdmin}); err != nil {
		t.Fatal(err)
	}
	m, _ = a.Merchants.ByID(ctx, m.ID)
	p2 := pay(t, a, ctx, m, 100000, "851234567")
	if p2.Fee != 3500 || p2.FeeBps != 350 || p2.Status != domain.PaymentSucceeded {
		t.Fatalf("negotiated fee not applied: %+v", p2)
	}
	if old, _ := a.Payments.ByID(ctx, p.ID); old.Fee != 7500 {
		t.Fatal("old payment fee must not change")
	}
	bal := a.Balance(ctx, m.ID, false)
	if bal != 214000 {
		t.Fatalf("balance %d", bal)
	}

	if _, err := a.RequestPayout(ctx, m.ID, false, 50000, "u"); err == nil {
		t.Fatal("payout without bank account must fail")
	}
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "BCI", Holder: "Loja Teste", NIB: "000800004821130101135"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RequestPayout(ctx, m.ID, false, bal+1, "u"); err == nil {
		t.Fatal("payout above balance must fail")
	}
	po, err := a.RequestPayout(ctx, m.ID, false, 100000, "u")
	if err != nil {
		t.Fatal(err)
	}
	if b := a.Balance(ctx, m.ID, false); b != bal-100000 {
		t.Fatalf("payout not debited: %d", b)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActPaid, "", "admin"); err == nil {
		t.Fatal("cannot mark a new payout as paid")
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActReject, "teste", "admin"); err != nil {
		t.Fatal(err)
	}
	if b := a.Balance(ctx, m.ID, false); b != bal {
		t.Fatalf("rejected payout not restored: %d", b)
	}
	po2, _ := a.RequestPayout(ctx, m.ID, false, 20000, "u")
	if _, err := a.TransitionPayout(ctx, po2.ID, domain.ActApprove, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if x, _ := a.TransitionPayout(ctx, po2.ID, domain.ActPaid, "", "admin"); x.Status != domain.PayoutPaid {
		t.Fatal("payout should be paid")
	}
	if _, err := a.RequestPayout(ctx, m.ID, true, 10000, "u"); err == nil {
		t.Fatal("live payout for unverified merchant must fail")
	}

	before := a.Balance(ctx, m.ID, false)
	if _, err := a.Refund(ctx, m.ID, false, p.ID, 25000, "", "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Refund(ctx, m.ID, false, p.ID, 200000, "", "u"); err == nil {
		t.Fatal("refund above remaining must fail")
	}
	if _, err := a.Refund(ctx, m.ID, false, p.ID, 0, "", "u"); err != nil {
		t.Fatal(err)
	}
	old, _ := a.Payments.ByID(ctx, p.ID)
	if old.Status != domain.PaymentRefunded || a.Balance(ctx, m.ID, false) != before-125000 {
		t.Fatalf("refund accounting wrong: %s %d", old.Status, a.Balance(ctx, m.ID, false))
	}

	evs, _ := a.Events.ByObjects(ctx, m.ID, []string{p.ID, p2.ID})
	succeeded := 0
	for _, e := range evs {
		if e.Type == "payment.succeeded" {
			succeeded++
		}
	}
	if succeeded != 2 {
		t.Fatalf("events %d", succeeded)
	}
}

func TestSessionValidation(t *testing.T) {
	a, ctx := setup(t)
	_, m, _ := a.Signup(ctx, app.SignupInput{BusinessName: "Loja V", Email: "v@b.co.mz", Phone: "841234567", Password: "segredo123"})
	api := domain.Origin{SourceType: domain.SourceAPI}
	cases := []app.SessionParams{
		{Amount: 999, SuccessURL: "https://x.co/ok"},
		{Amount: 5000, Currency: "USD", SuccessURL: "https://x.co/ok"},
		{Amount: 5000, SuccessURL: "https://x.co/ok", CallbackURL: "https://not-verified.co/cb"},
		{Amount: 5000, SuccessURL: "https://x.co/ok", LineItems: []domain.LineItem{{Name: "a", Quantity: 1, UnitAmount: 4000}}},
		{Amount: 5000, SuccessURL: "https://x.co/ok", PaymentMethodTypes: []string{"paypal"}},
		{Amount: 5000},
	}
	for i, c := range cases {
		c.Origin = api
		if _, err := a.CreateSession(ctx, m, false, c); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
	if _, err := a.AddDomain(ctx, m.ID, "https://loja.co.mz/x"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: 5000, SuccessURL: "https://loja.co.mz/ok", CallbackURL: "https://api.loja.co.mz/cb", Origin: api}); err != nil {
		t.Fatalf("subdomain of verified domain must be allowed: %v", err)
	}
}

func TestWebhookDelivery(t *testing.T) {
	a, ctx := setup(t)
	_, m, _ := a.Signup(ctx, app.SignupInput{BusinessName: "Loja W", Email: "w@b.co.mz", Phone: "841234567", Password: "segredo123"})
	if _, err := a.CreateEndpoint(ctx, m.ID, false, "http://localhost:9/hook", []string{"payment.succeeded"}); err != nil {
		t.Fatal(err)
	}
	pay(t, a, ctx, m, 5000, "841234567")
	if n := a.DeliverDue(ctx, 20); n != 1 {
		t.Fatalf("expected 1 delivery, got %d", n)
	}
	ds := a.RecentDeliveries(ctx, m.ID, false)
	if len(ds) != 1 || ds[0].Status != domain.DeliverySucceeded || ds[0].Attempts != 1 {
		t.Fatalf("delivery %+v", ds)
	}
}
