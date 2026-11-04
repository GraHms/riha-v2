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
