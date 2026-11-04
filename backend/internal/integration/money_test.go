package integration

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"riha/internal/adapters/crypto"
	"riha/internal/adapters/filestore"
	"riha/internal/adapters/messaging"
	"riha/internal/adapters/pdf"
	"riha/internal/adapters/postgres"
	"riha/internal/adapters/rails"
	"riha/internal/adapters/sheets"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

type outbox struct{ sent []domain.Email }

func (o *outbox) Send(_ context.Context, e domain.Email) error {
	o.sent = append(o.sent, e)
	return nil
}

var mails = &outbox{}

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
		Plans: r.Plans, Members: r.Members, Subs: r.Subs, Charges: r.Charges, Messages: r.Messages, SubSettings: r.SubSettings,
		Messenger: &messaging.Outbox{}, Sheets: sheets.Reader{},
		Rail: mock, Sender: nullSender{}, Files: filestore.Local{Root: t.TempDir()}, Hasher: crypto.Bcrypt{},
		Renderer: pdf.New(), Mailer: mails,
	}, "http://localhost:5173")
	a.AllowPrivateNetworks = true
	return a, context.Background()
}

func approved(t *testing.T, a *app.App, ctx context.Context, name, email string) *domain.Merchant {
	t.Helper()
	_, m, err := a.Signup(ctx, app.SignupInput{BusinessName: name, Email: email, Phone: "841234567", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != domain.MerchantLead {
		t.Fatalf("signup must create a lead, got %s", m.Status)
	}
	m, err = a.Decide(ctx, m.ID, "approve", "", "admin")
	if err != nil {
		t.Fatal(err)
	}
	return m
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
	m := approved(t, a, ctx, "Loja Teste", "a@b.co.mz")

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
	m := approved(t, a, ctx, "Loja V", "v@b.co.mz")
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
	m := approved(t, a, ctx, "Loja W", "w@b.co.mz")
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

func TestLeadsCannotOperate(t *testing.T) {
	a, ctx := setup(t)
	_, m, err := a.Signup(ctx, app.SignupInput{BusinessName: "Lead", Email: "lead@b.co.mz", Phone: "841234567", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveKyc(ctx, m, app.KycInput{BusinessType: "individual", NUIT: "400123456"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SubmitKyc(ctx, m, app.Actor{}); err != nil {
		t.Fatalf("submitting without documents must be allowed: %v", err)
	}
	name := "Bolo"
	amount := int64(5000)
	if _, err := a.CreateLink(ctx, m, false, app.LinkParams{Name: &name, Amount: &amount}, "u", "dashboard"); err == nil {
		t.Fatal("lead must not create links")
	}
	if _, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: 5000, SuccessURL: "https://x.co/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}}); err == nil {
		t.Fatal("lead must not create sessions")
	}
	if _, _, err := a.RollSecretKey(ctx, m, false, app.Actor{}); err == nil {
		t.Fatal("lead must not get API keys")
	}
	if _, err := a.RequestPayout(ctx, m.ID, false, 10000, "u"); err == nil {
		t.Fatal("lead must not request payouts")
	}
	m, _ = a.Decide(ctx, m.ID, "approve", "", "admin")
	secret, _, err := a.RollSecretKey(ctx, m, false, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Decide(ctx, m.ID, "suspend", "", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.AuthenticateSecret(ctx, secret); err == nil {
		t.Fatal("suspended merchant API keys must be rejected")
	}
}

func TestPayoutEvidenceAndStatement(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja P", "p@b.co.mz")
	m.Status = domain.MerchantActive
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: "BCI", Holder: "Loja P", NIB: "000800004821130101135"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if err := a.Ledger.Add(ctx, &domain.LedgerEntry{ID: domain.NewID("le"), MerchantID: m.ID, Livemode: true, Type: domain.LedgerPayment, Amount: 500000, CreatedAt: a.Now()}); err != nil {
		t.Fatal(err)
	}
	po, err := a.RequestPayout(ctx, m.ID, true, 200000, "u")
	if err != nil {
		t.Fatal(err)
	}
	instr, err := a.PayoutInstructionsPDF(ctx, []string{po.ID})
	if err != nil || !bytes.HasPrefix(instr, []byte("%PDF")) {
		t.Fatalf("instruction pdf: %v", err)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActApprove, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActPaid, "", "admin"); err == nil {
		t.Fatal("live payout must not be paid without a receipt")
	}
	bad := app.Upload{FileName: "x.pdf", Size: 4, Head: []byte("nope"), Body: strings.NewReader("nope")}
	if _, err := a.CompletePayouts(ctx, []string{po.ID}, bad, "admin"); err == nil {
		t.Fatal("receipt content must be validated")
	}
	receipt := []byte("%PDF-1.4\n%recibo\n")
	done, err := a.CompletePayouts(ctx, []string{po.ID}, app.Upload{FileName: "talao.pdf", Size: int64(len(receipt)), Head: receipt[:8], Body: bytes.NewReader(receipt)}, "admin")
	if err != nil || len(done) != 1 || done[0].Status != domain.PayoutPaid || done[0].ReceiptHash == "" {
		t.Fatalf("complete: %v %+v", err, done)
	}
	if _, _, err := a.PayoutStatementPDF(ctx, po.ID, "acct_other"); err == nil {
		t.Fatal("another merchant must not read the statement")
	}
	st, _, err := a.PayoutStatementPDF(ctx, po.ID, m.ID)
	if err != nil || !bytes.HasPrefix(st, []byte("%PDF")) {
		t.Fatalf("statement: %v", err)
	}
	mails.sent = nil
	to, err := a.EmailPayoutStatement(ctx, po.ID, app.Actor{})
	if err != nil || to != "p@b.co.mz" || len(mails.sent) != 1 || len(mails.sent[0].Attachments) != 2 {
		t.Fatalf("email: %v %s %+v", err, to, mails.sent)
	}
}

func TestPrivateWebhookTargetsBlocked(t *testing.T) {
	a, ctx := setup(t)
	a.AllowPrivateNetworks = false
	m := approved(t, a, ctx, "Loja S", "s@b.co.mz")
	for _, u := range []string{"http://localhost/x", "http://127.0.0.1/x", "http://169.254.169.254/latest", "http://10.0.0.5/hook"} {
		if _, err := a.CreateEndpoint(ctx, m.ID, false, u, nil); err == nil {
			t.Errorf("%s must be rejected", u)
		}
	}
	if _, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: 5000, SuccessURL: "https://x.co/ok", CallbackURL: "http://localhost/cb", Origin: domain.Origin{SourceType: domain.SourceAPI}}); err == nil {
		t.Error("localhost callback must be rejected")
	}
}
