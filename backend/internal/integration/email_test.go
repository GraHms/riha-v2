package integration

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"riha/internal/adapters/httpapi"
	"riha/internal/adapters/rails"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

func subjects(t *testing.T, a *app.App, ctx context.Context) []string {
	t.Helper()
	mails.sent = nil
	a.SendDueEmails(ctx, 100)
	out := []string{}
	for _, e := range mails.sent {
		out = append(out, e.To+" | "+e.Subject)
	}
	return out
}

func hasMail(list []string, to, subject string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, to+" | ") && strings.Contains(s, subject) {
			return true
		}
	}
	return false
}

func livePay(t *testing.T, a *app.App, ctx context.Context, m *domain.Merchant, email string) *domain.Payment {
	t.Helper()
	cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 50000, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: domain.MethodMpesa, Phone: "841234567", Email: email})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAccountEmails(t *testing.T) {
	a, ctx := setup(t)
	a.AdminNotifyEmail = "ops@riha.test"
	_, m, err := a.Signup(ctx, app.SignupInput{BusinessName: "Café <b>Sol</b>", Name: "Ana Mabota", Email: "ana@sol.co.mz", Phone: "841234567", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}
	got := subjects(t, a, ctx)
	if !hasMail(got, "ana@sol.co.mz", "Bem-vindo à RIHA") {
		t.Fatalf("welcome: %v", got)
	}
	if !strings.Contains(mails.sent[0].HTML, "Olá Ana") || strings.Contains(mails.sent[0].HTML, "<b>Sol</b>") {
		t.Fatalf("welcome html: %s", mails.sent[0].HTML)
	}
	m.NUIT, m.BusinessType = "400123456", "company"
	if err := a.SubmitKyc(ctx, m, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	got = subjects(t, a, ctx)
	if !hasMail(got, "ana@sol.co.mz", "Recebemos os dados") || !hasMail(got, "ops@riha.test", "Nova conta para rever") {
		t.Fatalf("kyc: %v", got)
	}
	if _, err := a.Decide(ctx, m.ID, "info", "Falta o alvará", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); len(got) != 1 || !strings.Contains(mails.sent[0].Text, "Falta o alvará") {
		t.Fatalf("info: %v", got)
	}
	if _, err := a.Decide(ctx, m.ID, "reject", "nota interna: fraude", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); len(got) != 1 || strings.Contains(mails.sent[0].Text, "fraude") {
		t.Fatalf("a rejection must not reveal the internal note: %v %s", got, mails.sent[0].Text)
	}
	for decision, want := range map[string]string{"approve": "foi aprovada", "suspend": "foi suspensa", "reactivate": "foi reativada"} {
		if _, err := a.Decide(ctx, m.ID, decision, "", "admin"); err != nil {
			t.Fatal(err)
		}
		if got := subjects(t, a, ctx); !hasMail(got, "ana@sol.co.mz", want) {
			t.Fatalf("%s: %v", decision, got)
		}
		if decision == "approve" {
			if _, err := a.Decide(ctx, m.ID, decision, "", "admin"); err != nil {
				t.Fatal(err)
			}
			if got := subjects(t, a, ctx); len(got) != 0 {
				t.Fatalf("approving twice must not email twice: %v", got)
			}
		}
	}
}

var resetLink = regexp.MustCompile(`/recuperar/nova#([A-Za-z0-9]+)`)

func TestPasswordReset(t *testing.T) {
	a, ctx := setup(t)
	u, _, err := a.Signup(ctx, app.SignupInput{BusinessName: "Sol", Name: "Ana", Email: "ana@sol.co.mz", Phone: "841234567", Password: "segredo123"})
	if err != nil {
		t.Fatal(err)
	}
	subjects(t, a, ctx)
	session, err := a.NewSession(ctx, u, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := a.RequestPasswordReset(ctx, "ninguem@sol.co.mz"); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("unknown address got mail: %v", got)
	}
	if err := a.RequestPasswordReset(ctx, " ANA@sol.co.mz "); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "ana@sol.co.mz", "Reponha a sua palavra-passe") {
		t.Fatalf("reset mail: %v", got)
	}
	m := resetLink.FindStringSubmatch(mails.sent[0].Text)
	if m == nil {
		t.Fatalf("no link: %s", mails.sent[0].Text)
	}
	token := m[1]

	if err := a.ResetPassword(ctx, "falso", "nova-senha-123"); err == nil {
		t.Fatal("made up token accepted")
	}
	if err := a.ResetPassword(ctx, token, "curta"); err == nil {
		t.Fatal("weak password accepted")
	}
	if err := a.ResetPassword(ctx, token, "nova-senha-123"); err != nil {
		t.Fatalf("a refused password must not spend the token: %v", err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "ana@sol.co.mz", "foi alterada") {
		t.Fatalf("changed notice: %v", got)
	}
	if err := a.ResetPassword(ctx, token, "outra-senha-123"); err == nil {
		t.Fatal("token used twice")
	}
	if _, _, err := a.Login(ctx, "ana@sol.co.mz", "segredo123", "", ""); err == nil {
		t.Fatal("old password still works")
	}
	if _, _, err := a.Login(ctx, "ana@sol.co.mz", "nova-senha-123", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.UserForSession(ctx, session); err == nil {
		t.Fatal("reset must sign out other devices")
	}

	for i := 0; i < app.MaxResetsPerHour+2; i++ {
		if err := a.RequestPasswordReset(ctx, "ana@sol.co.mz"); err != nil {
			t.Fatal(err)
		}
	}
	// One request already counted above.
	if got := subjects(t, a, ctx); len(got) != app.MaxResetsPerHour-1 {
		t.Fatalf("hourly cap: %d mails", len(got))
	}
}

func TestResetTokenExpires(t *testing.T) {
	a, ctx := setup(t)
	if _, _, err := a.Signup(ctx, app.SignupInput{BusinessName: "Sol", Name: "Ana", Email: "ana@sol.co.mz", Phone: "841234567", Password: "segredo123"}); err != nil {
		t.Fatal(err)
	}
	subjects(t, a, ctx)
	if err := a.RequestPasswordReset(ctx, "ana@sol.co.mz"); err != nil {
		t.Fatal(err)
	}
	subjects(t, a, ctx)
	token := resetLink.FindStringSubmatch(mails.sent[0].Text)[1]
	now := a.Now
	a.Now = func() time.Time { return now().Add(app.PasswordResetTTL + 1) }
	if err := a.ResetPassword(ctx, token, "nova-senha-123"); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestPaymentAndPayoutEmails(t *testing.T) {
	a, ctx := setup(t)
	a.AdminNotifyEmail = "ops@riha.test"
	m := approved(t, a, ctx, "Loja", "loja@sol.co.mz")
	subjects(t, a, ctx)

	pay(t, a, ctx, m, 50000, "841234567")
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("test mode payments must stay silent: %v", got)
	}

	p := livePay(t, a, ctx, m, "cliente@mail.co.mz")
	got := subjects(t, a, ctx)
	if !hasMail(got, "cliente@mail.co.mz", "Recibo do pagamento a Loja") || !hasMail(got, "loja@sol.co.mz", "Recebeu 500,00 MT") {
		t.Fatalf("payment: %v", got)
	}
	r, err := a.Refund(ctx, m.ID, true, p.ID, 10000, "", "ana")
	if err != nil || r == nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "cliente@mail.co.mz", "Reembolso de Loja") || !hasMail(got, "loja@sol.co.mz", "Reembolso de 100,00 MT") {
		t.Fatalf("refund: %v", got)
	}

	off := false
	if err := a.UpdateProfile(ctx, m, app.ProfileInput{EmailNotifications: &off}); err != nil {
		t.Fatal(err)
	}
	livePay(t, a, ctx, m, "cliente@mail.co.mz")
	if got := subjects(t, a, ctx); len(got) != 1 || !hasMail(got, "cliente@mail.co.mz", "Recibo") {
		t.Fatalf("the toggle silences the merchant, not the customer: %v", got)
	}
	noEmail := livePay(t, a, ctx, m, "")
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("no customer address, merchant off: %v", got)
	}
	if _, err := a.SendReceipt(ctx, m.ID, true, noEmail.ID, ""); err == nil {
		t.Fatal("receipt without an address")
	}
	if to, err := a.SendReceipt(ctx, m.ID, true, noEmail.ID, "novo@mail.co.mz"); err != nil || to != "novo@mail.co.mz" {
		t.Fatal(to, err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "novo@mail.co.mz", "Recibo") {
		t.Fatalf("manual receipt: %v", got)
	}
	if _, err := a.SendReceipt(ctx, "acct_outro", true, noEmail.ID, ""); err == nil {
		t.Fatal("another merchant's payment")
	}

	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: app.Banks[0], Holder: "Loja", NIB: "000800001234567890123"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("the first bank account is no alarm: %v", got)
	}
	if _, err := a.SaveBankAccount(ctx, m, app.BankInput{BankName: app.Banks[0], Holder: "Loja", NIB: "000800001234567899999"}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "loja@sol.co.mz", "conta bancária") {
		t.Fatalf("bank change: %v", got)
	}

	po, err := a.RequestPayout(ctx, m.ID, true, 20000, "ana")
	if err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "loja@sol.co.mz", po.Ref()) || !hasMail(got, "ops@riha.test", "Novo levantamento") {
		t.Fatalf("payout: %v", got)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActApprove, "", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "loja@sol.co.mz", "enviado ao banco") {
		t.Fatalf("sent: %v", got)
	}
	if _, err := a.TransitionPayout(ctx, po.ID, domain.ActFail, "NIB inválido", "admin"); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "loja@sol.co.mz", "não concluído") || !strings.Contains(mails.sent[0].Text, "NIB inválido") {
		t.Fatalf("failed: %v", got)
	}

	if _, _, err := a.RollSecretKey(ctx, m, true, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if got := subjects(t, a, ctx); !hasMail(got, "loja@sol.co.mz", "nova chave API") {
		t.Fatalf("key: %v", got)
	}
}

type brokenMailer struct{ err error }

func (b brokenMailer) Send(context.Context, domain.Email) error { return b.err }

func TestEmailsRetryAndRollBack(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja", "loja@sol.co.mz")
	subjects(t, a, ctx)

	if _, err := a.RequestPayout(ctx, m.ID, true, 1<<40, "ana"); err == nil {
		t.Fatal("payout above the balance")
	}
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("a refused change must not email: %v", got)
	}

	if err := a.RequestPasswordReset(ctx, "loja@sol.co.mz"); err != nil {
		t.Fatal(err)
	}
	good := a.Mailer
	a.Mailer = brokenMailer{errors.New("servidor em baixo")}
	if n := a.SendDueEmails(ctx, 10); n != 1 {
		t.Fatalf("claimed %d", n)
	}
	a.Mailer = good
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("retried straight away: %v", got)
	}
	now := a.Now
	a.Now = func() time.Time { return now().Add(domain.RetrySchedule[0] + 1) }
	if got := subjects(t, a, ctx); !hasMail(got, "loja@sol.co.mz", "Reponha") {
		t.Fatalf("not retried: %v", got)
	}
	if got := subjects(t, a, ctx); len(got) != 0 {
		t.Fatalf("sent twice: %v", got)
	}
}

// answerRail stands in for M-Pesa: nothing completes by itself, as when the answer is lost.
type answerRail struct {
	outcome domain.Outcome
	known   bool
	err     error
	queries []string
	sent    []domain.ChargeRequest
}

func (r *answerRail) Initiate(_ context.Context, q domain.ChargeRequest, _ func(domain.Outcome)) (domain.ChargeResult, error) {
	r.sent = append(r.sent, q)
	return domain.ChargeResult{Async: true, Last4: "4567"}, nil
}

func (r *answerRail) Query(_ context.Context, q domain.StatusQuery) (domain.Outcome, bool, error) {
	r.queries = append(r.queries, q.Reference)
	return r.outcome, r.known, r.err
}

func TestRealMoneyRules(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, "Loja", "loja@sol.co.mz")
	rail := &answerRail{}
	sim := rails.NewMock(0)
	sim.After = func(time.Duration, func()) {}
	a.Rail = rails.Modes{Test: sim, Live: rail}
	a.ComingSoon = []string{domain.MethodEmola, domain.MethodBim, domain.MethodCard}
	a.NoLiveRefunds = true
	now := a.Now
	at := func(d time.Duration) { a.Now = func() time.Time { return now().Add(d) } }
	start := func(live bool) *domain.Payment {
		t.Helper()
		at(0)
		cs, err := a.CreateSession(ctx, m, live, app.SessionParams{Amount: 50000, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(cs.PaymentMethodTypes, ",") != "mpesa" {
			t.Fatalf("methods: %v", cs.PaymentMethodTypes)
		}
		p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: domain.MethodMpesa, Phone: "841234567"})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	status := func(p *domain.Payment) string {
		t.Helper()
		got, err := a.Payments.ByID(ctx, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.Status
	}

	t.Run("the reference is saved and sent", func(t *testing.T) {
		p := start(true)
		if !regexp.MustCompile(`^RIHA[A-Z0-9]{16}$`).MatchString(p.RailReference) {
			t.Fatalf("reference %q", p.RailReference)
		}
		last := rail.sent[len(rail.sent)-1]
		if last.Reference != p.RailReference || !last.Live {
			t.Fatalf("%+v", last)
		}
		got, _ := a.Payments.ByID(ctx, p.ID)
		if got.RailReference != p.RailReference {
			t.Fatal("not stored, so it could not be traced after a restart")
		}
		at(6 * time.Minute)
		a.Sweep(ctx) // leave nothing processing for the next case
	})

	t.Run("a lost answer is not a failure while M-Pesa may still answer", func(t *testing.T) {
		rail.known, rail.err = false, errors.New("network down")
		p := start(true)
		at(4 * time.Minute)
		a.Sweep(ctx)
		if status(p) != domain.PaymentProcessing {
			t.Fatalf("query failed, yet the payment ended: %s", status(p))
		}
		before := a.Balance(ctx, m.ID, true)
		rail.err, rail.known, rail.outcome = nil, true, domain.Outcome{Success: true, OperatorReference: "TX1"}
		a.Sweep(ctx)
		if status(p) != domain.PaymentSucceeded {
			t.Fatalf("%s", status(p))
		}
		if a.Balance(ctx, m.ID, true) <= before {
			t.Fatal("not credited")
		}
	})

	t.Run("no record at M-Pesa ends it, but not too soon", func(t *testing.T) {
		rail.known, rail.err = false, nil
		p := start(true)
		at(4 * time.Minute)
		a.Sweep(ctx)
		if status(p) != domain.PaymentProcessing {
			t.Fatal("ended before the customer's time was up")
		}
		at(6 * time.Minute)
		a.Sweep(ctx)
		if status(p) != domain.PaymentFailed {
			t.Fatalf("%s", status(p))
		}
	})

	t.Run("no answer at all is given up on only after 30 minutes", func(t *testing.T) {
		rail.known, rail.err = false, errors.New("network down")
		p := start(true)
		at(29 * time.Minute)
		a.Sweep(ctx)
		if status(p) != domain.PaymentProcessing {
			t.Fatal("gave up too soon")
		}
		at(31 * time.Minute)
		a.Sweep(ctx)
		if status(p) != domain.PaymentFailed {
			t.Fatalf("%s", status(p))
		}
		rail.err = nil
	})

	t.Run("test mode never asks M-Pesa", func(t *testing.T) {
		rail.queries = nil
		p := start(false)
		at(4 * time.Minute)
		a.Sweep(ctx)
		if len(rail.queries) != 0 || status(p) != domain.PaymentFailed {
			t.Fatalf("%v %s", rail.queries, status(p))
		}
	})

	t.Run("methods we don't have are refused", func(t *testing.T) {
		at(0)
		if _, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 50000, PaymentMethodTypes: []string{"card"}, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}}); err == nil {
			t.Fatal("card accepted")
		}
		cs, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 50000, PaymentMethodTypes: []string{"mpesa", "emola"}, SuccessURL: "https://loja.co.mz/ok", Origin: domain.Origin{SourceType: domain.SourceAPI}})
		if err != nil || strings.Join(cs.PaymentMethodTypes, ",") != "mpesa" {
			t.Fatalf("%v %v", err, cs)
		}
		if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: domain.MethodEmola, Phone: "861234567"}); err == nil {
			t.Fatal("e-Mola accepted")
		}
	})

	t.Run("a live refund is refused: no money would move", func(t *testing.T) {
		rail.known, rail.outcome = true, domain.Outcome{Success: true}
		p := start(true)
		at(4 * time.Minute)
		a.Sweep(ctx)
		at(0)
		before := a.Balance(ctx, m.ID, true)
		mails.sent = nil
		if _, err := a.Refund(ctx, m.ID, true, p.ID, 1000, "", "ana"); err == nil {
			t.Fatal("refund accepted")
		}
		if a.Balance(ctx, m.ID, true) != before {
			t.Fatal("balance moved")
		}
	})
}

func TestLinkPreviewsAndSearchRules(t *testing.T) {
	a, ctx := setup(t)
	m := approved(t, a, ctx, `Loja </title><script>alert(1)</script>`, "loja@sol.co.mz")
	l, err := a.CreateLink(ctx, m, true, app.LinkParams{Name: ptr(`Bolo <img src=x onerror=alert(2)>`), Amount: ptr(int64(125000))}, "ana", "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/index.html", []byte("<html><head><!--seo:head--><title>x</title><!--/seo:head--></head><body><!--seo:body--></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	engine := httpapi.New(a, httpapi.Config{StaticDir: dir})
	a.PublicURL = "https://rihagateway.com"
	get := func(path string) (*httptest.ResponseRecorder, string) {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w, w.Body.String()
	}
	w, page := get("/p/" + m.Slug + "/" + l.Slug)
	if w.Code != 200 || strings.Contains(page, "<script>alert") || strings.Contains(page, "<img src=x") {
		t.Fatalf("names reached the HTML unescaped (%d): %s", w.Code, page)
	}
	for _, want := range []string{domain.Format(125000) + " MT", `og:image" content="https://rihagateway.com/og/p/` + m.Slug + "/" + l.Slug + ".png?v=", "summary_large_image", "noindex"} {
		if !strings.Contains(strings.ReplaceAll(page, " ", " "), want) {
			t.Fatalf("missing %q in %s", want, page)
		}
	}
	if w.Header().Get("X-Robots-Tag") == "" {
		t.Fatal("payment links must stay out of search")
	}
	if img, _ := get("/og/p/" + m.Slug + "/" + l.Slug + ".png"); img.Code != 200 || img.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("image: %d", img.Code)
	}
	w, page = get("/")
	if w.Code != 200 || !strings.Contains(page, "index,follow") || !strings.Contains(page, `rel="canonical" href="https://rihagateway.com/"`) || !strings.Contains(page, "application/ld+json") {
		t.Fatalf("home (%d): %s", w.Code, page)
	}
	for path, code := range map[string]int{"/painel": 200, "/admin/aprovacoes": 200, "/entrar": 200, "/p/nada/nada": 404, "/c/cs_nada": 404, "/nao-existe": 404} {
		w, page := get(path)
		if w.Code != code || !strings.Contains(page, "noindex") || strings.Contains(page, "canonical") {
			t.Fatalf("%s (%d): %s", path, w.Code, page)
		}
	}
	if _, xml := get("/sitemap.xml"); !strings.Contains(xml, "<loc>https://rihagateway.com/docs</loc>") || strings.Contains(xml, "/p/") {
		t.Fatal(xml)
	}
	if _, txt := get("/robots.txt"); !strings.Contains(txt, "Sitemap: https://rihagateway.com/sitemap.xml") || !strings.Contains(txt, "Disallow: /admin") || strings.Contains(txt, "Disallow: /p\n") {
		t.Fatal(txt)
	}
}

func ptr[T any](v T) *T { return &v }
