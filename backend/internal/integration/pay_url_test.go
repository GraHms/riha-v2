package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"riha/internal/adapters/httpapi"
	"riha/internal/core/app"
)

// Customers open these from a text message, and previews and browsers need a real
// answer. They must also stay out of search, because they name a customer.
func TestSubscriptionPagesAreRealPages(t *testing.T) {
	a, _ := setup(t)
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/index.html", []byte(`<html><head><!--seo:head--><title>x</title><!--/seo:head--></head><body><div id="root"></div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := httpapi.New(a, httpapi.Config{StaticDir: dir})
	for _, path := range []string{"/i/qualquertoken", "/s/loja/plano", "/a/loja"} {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "noindex") {
			t.Fatalf("%s: %d, noindex %v", path, w.Code, strings.Contains(w.Body.String(), "noindex"))
		}
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nao/existe/mesmo", nil))
	if w.Code != 404 {
		t.Fatalf("other paths stay real 404s: %d", w.Code)
	}
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	if !strings.Contains(w.Body.String(), "Disallow: /i/") {
		t.Fatalf("robots: %s", w.Body.String())
	}
}

// Links are handed out on the payment address, while the same pages stay
// reachable on the main site for links already shared.
func TestLinksUseThePaymentAddress(t *testing.T) {
	a, ctx := setup(t)
	a.PublicURL, a.PayURL = "https://rihagateway.com", "https://pay.rihagateway.com"
	m := approved(t, a, ctx, "Loja Pay", "lojapay@example.com")
	secret, _, err := a.RollSecretKey(ctx, m, false, app.Actor{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/index.html", []byte(`<html><head><!--seo:head--><title>x</title><!--/seo:head--></head><body><div id="root"></div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	engine := httpapi.New(a, httpapi.Config{StaticDir: dir})
	b, _ := json.Marshal(map[string]any{"amount": 50000, "success_url": "https://loja.co.mz/ok"})
	r := httptest.NewRequest(http.MethodPost, "/v1/checkout/sessions", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, r)
	var s map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &s)
	id, _ := s["id"].(string)
	if s["url"] != "https://pay.rihagateway.com/c/"+id {
		t.Fatalf("checkout url: %v", s["url"])
	}
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/c/"+id, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Loja Pay") {
		t.Fatalf("the same page must open whichever address it was shared on: %d", w.Code)
	}
}
