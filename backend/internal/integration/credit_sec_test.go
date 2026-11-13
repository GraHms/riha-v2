package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

type client struct {
	t   *testing.T
	url string
	tok string
}

func (c client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.url+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "riha")
	if c.tok != "" {
		req.AddCookie(&http.Cookie{Name: "riha_session", Value: c.tok})
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// A live request never shows the SMS code; only test mode does, to try the flow.
func TestSecCreditCodeNeverLeaksInLiveMode(t *testing.T) {
	w := creditSetup(t)
	a, ctx, m := w.a, w.ctx, w.m
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	pub := client{t: t, url: srv.URL}
	body := map[string]any{"product_id": w.prod.ID, "amount": 2000000, "term": 4, "day": 12, "name": "Celeste Machava", "phone": "845550142",
		"id_number": "110100456789B", "activity": "Capulanas", "purpose": "Stock", "income": 2500000}
	code, out := pub.do("POST", "/api/public/credit/"+m.Slug+"/requests", body)
	if code != 200 || out["test_code"] == nil {
		t.Fatalf("test mode: %d %v", code, out)
	}
	if _, err := a.CreateCreditProduct(ctx, m, true, app.ProductInput{Name: "Vivo", Min: 500000, Max: 5000000, Freq: domain.FreqMonthly, TermMin: 2, TermMax: 12,
		RateBP: 600, EffortPct: 35, Fields: []string{domain.FieldIncome}}, app.Actor{}); err != nil {
		t.Fatal(err)
	}
	_, prods := pub.do("GET", "/api/public/credit/"+m.Slug, nil)
	live := prods["products"].([]any)[0].(map[string]any)["id"].(string)
	code, out = pub.do("POST", "/api/public/credit/"+m.Slug+"/requests", map[string]any{"product_id": live, "amount": 1000000, "term": 4, "name": "Momade Ussene", "phone": "872204410", "income": 9000000})
	if code != 200 || out["test_code"] != nil || prods["test_mode"] != false {
		t.Fatalf("live mode showed the code: %d %v", code, out)
	}
	code, out = pub.do("POST", "/api/public/credit-r/"+out["token"].(string)+"/code", nil)
	if out["test_code"] != nil {
		t.Fatalf("a live resend showed the code: %d %v", code, out)
	}
}

func TestSecCreditPanelStaysInItsInstitution(t *testing.T) {
	w := creditSetup(t)
	a, ctx := w.a, w.ctx
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	r := w.request(t, "845550142")
	l, err := a.CreateLoan(ctx, w.m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "851184402", Name: "Hortência Macamo", Amount: 1500000, Term: 3, Day: 12, Delivery: domain.DeliverOutside}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	boss, _ := a.Users.ByEmail(ctx, "dona@riha.co.mz")
	other := approved(t, a, ctx, "Outra MC", "outra@example.com")
	if _, err := a.SetMerchantMicrocredit(ctx, boss, other.ID, true, "", app.Actor{}); err != nil {
		t.Fatal(err)
	}
	tok, _, err := a.Login(ctx, "outra@example.com", "segredo123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	c := client{t: t, url: srv.URL, tok: tok}
	for _, p := range []string{"/api/microcredit/requests/" + r.ID, "/api/microcredit/loans/" + l.ID, "/api/microcredit/products/" + w.prod.ID, "/api/microcredit/loans/" + l.ID + "/agreement.pdf"} {
		req, _ := http.NewRequest("GET", srv.URL+p, nil)
		req.Header.Set("X-Riha-Mode", "test")
		req.AddCookie(&http.Cookie{Name: "riha_session", Value: tok})
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("another institution read %s: %d", p, res.StatusCode)
		}
	}
	for _, act := range []string{"cancel", "resend", "write-off"} {
		req, _ := http.NewRequest("POST", srv.URL+"/api/microcredit/loans/"+l.ID+"/"+act, strings.NewReader(`{"note":"x"}`))
		req.Header.Set("X-Riha-Mode", "test")
		req.Header.Set("X-Requested-With", "riha")
		req.AddCookie(&http.Cookie{Name: "riha_session", Value: tok})
		res, _ := http.DefaultClient.Do(req)
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("another institution could %s a loan: %d", act, res.StatusCode)
		}
	}
	if code, _ := (client{t: t, url: srv.URL}).do("GET", "/api/microcredit/loans/"+l.ID, nil); code != http.StatusUnauthorized {
		t.Fatalf("no session read a loan: %d", code)
	}
	if code, _ := c.do("POST", "/api/microcredit/loans/"+l.ID+"/cancel", nil); code == 200 {
		t.Fatal("cancelled another institution's offer")
	}
	if v, _ := a.CreditLoan(ctx, w.m, false, l.ID); v.Loan.Status != domain.LoanOffered {
		t.Fatalf("the loan changed: %s", v.Loan.Status)
	}
}

func TestSecCreditUploadsAreWhatTheySay(t *testing.T) {
	w := creditSetup(t)
	a := w.a
	srv := httptest.NewServer(httpapi.New(a, httpapi.Config{}))
	defer srv.Close()
	r, _, err := a.StartCreditRequest(w.ctx, w.m.Slug, app.CreditRequestInput{ProductID: w.prod.ID, Amount: 2000000, Term: 4, Day: 12, Name: "Celeste Machava",
		Phone: "845550142", IDNumber: "110100456789B", Activity: "x", Purpose: "y", Income: 2500000})
	if err != nil {
		t.Fatal(err)
	}
	send := func(name string, content []byte, kind string) int {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		_ = mw.WriteField("kind", kind)
		fw, _ := mw.CreateFormFile("file", name)
		_, _ = fw.Write(content)
		mw.Close()
		req, _ := http.NewRequest("POST", srv.URL+"/api/public/credit-r/"+r.Token+"/documents", &b)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("X-Requested-With", "riha")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	png := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), make([]byte, 100)...)
	if c := send("bi.png", []byte("<script>alert(1)</script>"), "id_front"); c == 200 {
		t.Fatal("a script named .png was stored")
	}
	if c := send("bi.html", []byte("<html></html>"), "id_front"); c == 200 {
		t.Fatal("an html file was stored")
	}
	if c := send("bi.png", png, "passport"); c == 200 {
		t.Fatal("an unknown kind was stored")
	}
	if c := send("bi.png", png, "id_front"); c != 200 {
		t.Fatalf("a real photo was refused: %d", c)
	}
	if c := send("bi.png", png, "id_front"); c != 200 {
		t.Fatalf("second photo: %d", c)
	}
}

// Revoking Microcrédito stops the institution, not the borrowers: they still see
// and pay what they owe, and the reminders keep going; nothing new can start.
func TestCreditRevokeKeepsBorrowersAbleToPay(t *testing.T) {
	w := creditSetup(t)
	a, ctx, m := w.a, w.ctx, w.m
	l, err := a.CreateLoan(ctx, m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "851184402", Name: "Hortência Macamo", Amount: 1500000, Term: 3, Day: 12, Delivery: domain.DeliverOutside}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	l = w.accept(t, l.Token)
	pending, err := a.CreateLoan(ctx, m, false, app.OfferInput{ProductID: w.prod.ID, Phone: "847301188", Name: "Zacarias Muianga", Amount: 1500000, Term: 3, Day: 12, Delivery: domain.DeliverOutside}, w.owner, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	boss, _ := a.Users.ByEmail(ctx, "dona@riha.co.mz")
	if _, err := a.SetMerchantMicrocredit(ctx, boss, m.ID, false, "fim do contrato", app.Actor{}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreditPage(ctx, l.Token, false); err != nil {
		t.Fatalf("the borrower lost the page: %v", err)
	}
	if p, err := a.PayLoan(ctx, l.Token, app.LoanPay{}); err != nil || p.Status != domain.PaymentSucceeded {
		t.Fatalf("the borrower could not pay: %v", err)
	}
	if _, err := a.SendAgreementCode(ctx, pending.Token, true); err == nil {
		t.Fatal("a new loan could still be accepted after the revoke")
	}
	w.at(time.Date(2027, 1, 13, 10, 0, 0, 0, domain.Maputo))
	a.CreditTick(ctx)
	if msgs, _ := a.Messages.ByMember(ctx, l.ID, 50); len(msgs) == 0 || !strings.Contains(msgs[0].Body, "/cr/") {
		t.Fatalf("no reminder after the revoke: %+v", msgs)
	}
}
