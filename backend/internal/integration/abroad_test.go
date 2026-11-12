package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"riha/internal/adapters/rails"
	"riha/internal/adapters/rails/u2opia"
	"riha/internal/core/app"
	"riha/internal/core/domain"
)

// u2opiaStandIn answers like U2opia's staging: a token, then a payment page.
func u2opiaStandIn(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/api/auth/token") {
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"token": "t", "refreshToken": "r", "expiresIn": 86400, "refreshTokenExpiresIn": 604800}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"responseCode": "200", "transactionId": "TX" + r.Header.Get("x-country-id"), "expiresIn": 600,
			"redirectionUrl": "https://wallet-staging.example/pay?q=1"})
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/api-manager"
}

func abroadApp(t *testing.T) (*app.App, context.Context, *domain.Merchant) {
	a, ctx := setup(t)
	za := u2opia.Known["ZA"]
	za.PerMZN = 0.28
	client, err := u2opia.New(u2opia.Config{BaseURL: u2opiaStandIn(t), PartnerCode: "RIH-TEST", Countries: map[string]u2opia.Country{"ZA": za}})
	if err != nil {
		t.Fatal(err)
	}
	sim := a.Rail
	a.Rail = rails.Modes{
		Test: rails.Router{Via: map[string]rails.Via{domain.IntegrationSimulator: {Rail: sim}, domain.IntegrationU2opia: {Rail: client, Methods: []string{"card", "abroad"}}},
			Default: map[string]string{"card": domain.IntegrationU2opia, "abroad": domain.IntegrationU2opia}, Fallback: domain.IntegrationSimulator},
		Live: rails.Single(domain.IntegrationSimulator, sim),
	}
	a.AbroadMethods = []string{"card", "abroad"}
	a.AbroadCountries = []domain.AbroadCountry{{Code: "ZA", Name: za.Name, Dial: za.Dial, Currency: za.Currency}}
	a.ComingSoon, a.TestComingSoon = []string{"abroad"}, []string{}
	a.PayURL = "https://pay.example"
	return a, ctx, approved(t, a, ctx, "Loja Global", "global@example.com")
}

// A customer abroad pays with their own wallet: sent to U2opia's page, back
// to the checkout, and paid when U2opia's callback says so.
func TestPayFromAbroad(t *testing.T) {
	a, ctx, m := abroadApp(t)
	cs, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: 50000, Description: "Bilhete", PaymentMethodTypes: []string{"mpesa", "abroad"}, Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	if !domain.Contains(cs.PaymentMethodTypes, "abroad") {
		t.Fatalf("test mode with U2opia takes payments from abroad: %v", cs.PaymentMethodTypes)
	}
	var de *domain.Error
	if _, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "abroad", Phone: "+27821234567"}); !errors.As(err, &de) || de.Code != "country_required" {
		t.Fatalf("the country is needed: %v", err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "abroad", Country: "za", Phone: "+27 82 123 4567"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentProcessing || p.RedirectURL != "https://wallet-staging.example/pay?q=1" || p.RailID != "TXZA" || p.Country != "ZA" ||
		p.PhonePrefix != "ZA" || p.PhoneLast4 != "4567" || p.ContactPhone != "" {
		t.Fatalf("payment %+v", p)
	}
	if to := a.AbroadReturn(ctx, "TXZA", "SUCCESS"); to != "https://pay.example/c/"+cs.ID {
		t.Fatalf("back from U2opia to %q", to)
	}
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentProcessing {
		t.Fatalf("coming back says nothing about the payment: %s", got.Status)
	}
	a.AbroadCallback(ctx, app.AbroadResult{TransactionID: "nobody", Status: 2})
	a.AbroadCallback(ctx, app.AbroadResult{TransactionID: "TXZA", Status: 1})
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentProcessing {
		t.Fatalf("a status that isn't final changes nothing: %s", got.Status)
	}
	a.AbroadCallback(ctx, app.AbroadResult{TransactionID: "TXZA", Status: 2, Reference: "WALLET_REF"})
	got, _ := a.Payments.ByID(ctx, p.ID)
	if got.Status != domain.PaymentSucceeded || got.OperatorReference != "WALLET_REF" || got.RedirectURL != "" {
		t.Fatalf("paid %+v", got)
	}
	if b := a.Balance(ctx, m.ID, false); b != got.Net {
		t.Fatalf("balance %d", b)
	}
}

// A failed payment fails; live mode can't take one at all, and wouldn't
// believe a callback about one.
func TestAbroadRefusalsAndLiveMode(t *testing.T) {
	a, ctx, m := abroadApp(t)
	cs, err := a.CreateSession(ctx, m, false, app.SessionParams{Amount: 50000, PaymentMethodTypes: []string{"card"}, Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.Pay(ctx, cs.ID, app.PayInput{Method: "card", Country: "ZA", Phone: "821234567"})
	if err != nil || p.RedirectURL == "" {
		t.Fatalf("a card in test mode goes through U2opia: %+v %v", p, err)
	}
	a.AbroadCallback(ctx, app.AbroadResult{TransactionID: p.RailID, Status: 3})
	if got, _ := a.Payments.ByID(ctx, p.ID); got.Status != domain.PaymentFailed || got.FailureCode != domain.FailRejected {
		t.Fatalf("failed %+v", got)
	}

	live, err := a.CreateSession(ctx, m, true, app.SessionParams{Amount: 50000, PaymentMethodTypes: []string{"mpesa", "abroad"}, Origin: domain.Origin{SourceType: domain.SourceDashboard}})
	if err != nil {
		t.Fatal(err)
	}
	if domain.Contains(live.PaymentMethodTypes, "abroad") {
		t.Fatalf("live mode can't take payments from abroad yet: %v", live.PaymentMethodTypes)
	}
	if _, err := a.Pay(ctx, live.ID, app.PayInput{Method: "abroad", Country: "ZA", Phone: "821234567"}); err == nil {
		t.Fatal("paying from abroad in live mode must be refused")
	}
	// A forged callback naming a live payment's rail id changes nothing.
	lp, err := a.Pay(ctx, live.ID, app.PayInput{Method: "mpesa", Phone: "841234567"})
	if err != nil {
		t.Fatal(err)
	}
	before := lp.Status
	a.AbroadCallback(ctx, app.AbroadResult{TransactionID: lp.RailID, Status: 2})
	if got, _ := a.Payments.ByID(ctx, lp.ID); got.Status != before {
		t.Fatalf("a U2opia callback can't touch a payment that isn't from abroad: %s", got.Status)
	}
}
