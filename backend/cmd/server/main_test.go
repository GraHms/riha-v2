package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"riha/internal/config"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
)

func publicKey(t *testing.T) string {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(der)
}

// counted stands in for one of mtoca's two APIs, so the test sees which one was called.
func counted(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/payments" {
			hits.Add(1)
		}
		_, _ = w.Write([]byte(`{"vendorTransactionId_NEP":"NEP1","paymentStatus":"P"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func charge(t *testing.T, r ports.Rail, method, phone string, live bool) {
	t.Helper()
	req := domain.ChargeRequest{PaymentID: "pay_1", Method: method, Amount: 5000, Phone: phone, Live: live, Reference: "123456789012"}
	if _, err := r.Initiate(context.Background(), req, func(domain.Outcome) {}); err != nil {
		t.Fatalf("%s live=%v: %v", method, live, err)
	}
}

func base(t *testing.T) config.Config {
	return config.Config{Env: "production", MpesaBaseURL: "https://mpesa.invalid", MpesaC2BPort: "1", MpesaQueryPort: "2", MpesaAPIKey: "k", MpesaPublicKey: publicKey(t), MpesaProviderCode: "1",
		MtocaTestOn: true, MtocaLiveURL: "https://api.payments.mtoca.net", MtocaTestURL: "https://api.payments.dev.mtoca.net", MtocaChannels: map[string]string{"mpesa": "MPESA", "emola": "EMOLA", "bim": "BIM", "card": "NOPE"}}
}

// With no mtoca settings, a deploy changes nothing: only M-Pesa takes money.
func TestMtocaIsOffUntilEnabled(t *testing.T) {
	p, err := paymentRail(base(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"emola", "bim", "card", "abroad"}; !reflect.DeepEqual(p.ComingSoon, want) || !p.RealMoney {
		t.Fatalf("coming soon %v, real money %v", p.ComingSoon, p.RealMoney)
	}
	q := p.Rail.(ports.RailReferencer)
	for _, live := range []bool{true, false} {
		if n := q.NewReference("mpesa", live); !strings.HasPrefix(n, "RIHA") {
			t.Fatalf("live=%v: M-Pesa must not be set up for mtoca: %q", live, n)
		}
	}
}

func TestEachModeUsesItsOwnMtoca(t *testing.T) {
	prd, prdHits := counted(t)
	dev, devHits := counted(t)
	cfg := base(t)
	cfg.Env, cfg.MtocaEnabled, cfg.MtocaLiveURL, cfg.MtocaTestURL = "development", true, prd.URL, dev.URL
	p, err := paymentRail(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"card", "abroad"}; !reflect.DeepEqual(p.ComingSoon, want) {
		t.Fatalf("coming soon %v", p.ComingSoon)
	}
	charge(t, p.Rail, "emola", "861234567", false)
	eventually(t, func() bool { return devHits.Load() == 1 && prdHits.Load() == 0 }, "a test payment must only reach dev")
	charge(t, p.Rail, "bim", "821234567", true)
	eventually(t, func() bool { return devHits.Load() == 1 && prdHits.Load() == 1 }, "a live payment must only reach production")
	// With the flag off, test M-Pesa stays simulated and live M-Pesa stays direct.
	charge(t, p.Rail, "mpesa", "841234567", false)
	time.Sleep(100 * time.Millisecond)
	if devHits.Load() != 1 || prdHits.Load() != 1 {
		t.Fatal("test M-Pesa must stay on the simulator")
	}
	q := p.Rail.(ports.RailReferencer)
	if n := q.NewReference("mpesa", true); !strings.HasPrefix(n, "RIHA") {
		t.Fatalf("live M-Pesa must stay direct: %q", n)
	}
	if n := q.NewReference("emola", true); len(n) != 12 {
		t.Fatalf("e-Mola must be set up for mtoca: %q", n)
	}
}

func TestMpesaFlagMovesItToMtoca(t *testing.T) {
	prd, prdHits := counted(t)
	dev, devHits := counted(t)
	cfg := base(t)
	cfg.Env, cfg.MtocaEnabled, cfg.MpesaViaMtoca, cfg.MtocaLiveURL, cfg.MtocaTestURL = "development", true, true, prd.URL, dev.URL
	cfg.MpesaAPIKey = ""
	p, err := paymentRail(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	charge(t, p.Rail, "mpesa", "841234567", true)
	charge(t, p.Rail, "mpesa", "841234567", false)
	eventually(t, func() bool { return prdHits.Load() == 1 && devHits.Load() == 1 }, "each mode must reach its own API")
	if want := []string{"card", "abroad"}; !reflect.DeepEqual(p.ComingSoon, want) {
		t.Fatalf("coming soon %v", p.ComingSoon)
	}
}

// While dev isn't usable, test payments can stay simulated and live ones still use mtoca.
func TestTestModeCanStaySimulated(t *testing.T) {
	prd, prdHits := counted(t)
	dev, devHits := counted(t)
	cfg := base(t)
	cfg.Env, cfg.MtocaEnabled, cfg.MpesaViaMtoca, cfg.MtocaTestOn, cfg.MtocaLiveURL, cfg.MtocaTestURL = "development", true, true, false, prd.URL, dev.URL
	p, err := paymentRail(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	charge(t, p.Rail, "mpesa", "841234567", false)
	charge(t, p.Rail, "emola", "861234567", false)
	time.Sleep(100 * time.Millisecond)
	if devHits.Load() != 0 || prdHits.Load() != 0 {
		t.Fatalf("test payments must be simulated: dev %d, production %d", devHits.Load(), prdHits.Load())
	}
	charge(t, p.Rail, "mpesa", "841234567", true)
	eventually(t, func() bool { return prdHits.Load() == 1 && devHits.Load() == 0 }, "live M-Pesa must reach production")
}

func TestMpesaFlagNeedsMtoca(t *testing.T) {
	cfg := base(t)
	cfg.MpesaViaMtoca = true
	if _, err := paymentRail(cfg, nil); err == nil {
		t.Error("the flag without mtoca must refuse to start, not quietly stay direct")
	}
	cfg.MtocaEnabled, cfg.MtocaChannels = true, map[string]string{"bim": "BIM"}
	if _, err := paymentRail(cfg, nil); err == nil {
		t.Error("the flag without an mpesa channel must refuse to start")
	}
}

func eventually(t *testing.T, ok func() bool, msg string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestProductionRefusesMixedUpSettings(t *testing.T) {
	cases := map[string]func(*config.Config){
		"same url": func(c *config.Config) { c.MtocaTestURL = c.MtocaLiveURL + "/" },
		"live on dev": func(c *config.Config) {
			c.MtocaLiveURL, c.MtocaTestURL = "https://api.payments.dev.mtoca.net", "https://other.dev.mtoca.net"
		},
		"live not https":   func(c *config.Config) { c.MtocaLiveURL = "http://api.payments.mtoca.net" },
		"test on real api": func(c *config.Config) { c.MtocaTestURL = "https://api.payments.mtoca.net/test" },
		"public http":      func(c *config.Config) { c.MtocaLiveURL = "http://1.1.1.1:8080" },
	}
	for name, change := range cases {
		cfg := base(t)
		cfg.MtocaEnabled = true
		change(&cfg)
		if _, err := paymentRail(cfg, nil); err == nil {
			t.Errorf("%s must refuse to start", name)
		}
	}
	cfg := base(t)
	cfg.MpesaAPIKey = ""
	if _, err := paymentRail(cfg, nil); err == nil {
		t.Error("production with no real rail must refuse to start")
	}
	cfg = base(t)
	cfg.MtocaEnabled = true
	if _, err := paymentRail(cfg, nil); err != nil {
		t.Errorf("the real addresses must start: %v", err)
	}
	for _, live := range []string{"http://mtoca:8080", "http://172.30.5.2:8080"} {
		cfg.MtocaLiveURL, cfg.MtocaTestURL = live, "http://mtoca-dev:8080"
		if _, err := paymentRail(cfg, nil); err != nil {
			t.Errorf("%s is private and must start: %v", live, err)
		}
	}
}

// With mtoca and the M-Pesa API both set up, live M-Pesa goes direct by default
// and mtoca can still take it for a merchant sent through it; the flag swaps
// which is the default.
func TestMpesaCanGoEitherWay(t *testing.T) {
	prd, prdHits := counted(t)
	dev, _ := counted(t)
	cfg := base(t)
	cfg.Env, cfg.MtocaEnabled, cfg.MtocaLiveURL, cfg.MtocaTestURL = "development", true, prd.URL, dev.URL
	p, err := paymentRail(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := p.Rail.(ports.RailRouting)
	if got := r.Routes(true)["mpesa"]; !reflect.DeepEqual(got, []string{domain.IntegrationMpesaDirect, domain.IntegrationMtoca}) {
		t.Fatalf("live M-Pesa routes %v", got)
	}
	if got := r.Routes(true)["emola"]; !reflect.DeepEqual(got, []string{domain.IntegrationMtoca}) {
		t.Fatalf("live e-Mola routes %v", got)
	}
	if r.RouteFor("", "mpesa", true) != domain.IntegrationMpesaDirect || r.RouteFor(domain.IntegrationMtoca, "mpesa", true) != domain.IntegrationMtoca ||
		r.RouteFor(domain.IntegrationMpesaDirect, "emola", true) != domain.IntegrationMtoca || r.RouteFor("", "mpesa", false) != domain.IntegrationSimulator {
		t.Fatal("routes")
	}
	if n := r.ReferenceVia(domain.IntegrationMtoca, "mpesa", true); len(n) != 12 {
		t.Fatalf("M-Pesa through mtoca takes mtoca's reference: %q", n)
	}
	if n := r.ReferenceVia("", "mpesa", true); !strings.HasPrefix(n, "RIHA") {
		t.Fatalf("M-Pesa direct takes its own reference: %q", n)
	}
	req := domain.ChargeRequest{PaymentID: "pay_1", Method: "mpesa", Amount: 5000, Phone: "841234567", Live: true, Reference: "123456789012", Integration: domain.IntegrationMtoca}
	if _, err := p.Rail.Initiate(context.Background(), req, func(domain.Outcome) {}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { return prdHits.Load() == 1 }, "M-Pesa sent through mtoca must reach mtoca")

	cfg.MpesaViaMtoca = true
	p, err = paymentRail(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r = p.Rail.(ports.RailRouting)
	if r.RouteFor("", "mpesa", true) != domain.IntegrationMtoca || r.RouteFor(domain.IntegrationMpesaDirect, "mpesa", true) != domain.IntegrationMpesaDirect {
		t.Fatal("with the flag, mtoca is the default and direct still an option")
	}
}

// U2opia takes cards and wallets from abroad in test mode only, for the
// countries it has a rate for; live mode still can't take either.
func TestU2opiaIsTestModeOnly(t *testing.T) {
	cfg := base(t)
	cfg.U2opiaEnabled, cfg.U2opiaPartnerCode, cfg.U2opiaTestURL = true, "RIH-TEST", "https://u2opia.invalid/api-manager"
	cfg.U2opiaMethods, cfg.U2opiaCardCountry = []string{"card", "abroad"}, "NG"
	cfg.U2opiaRates = map[string]string{"za": "0.28", "ng": "24.3"}
	p, err := paymentRail(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"emola", "bim", "card", "abroad"}; !reflect.DeepEqual(p.ComingSoon, want) {
		t.Fatalf("live coming soon %v", p.ComingSoon)
	}
	if want := []string{"emola", "bim"}; !reflect.DeepEqual(p.TestComingSoon, want) {
		t.Fatalf("test coming soon %v", p.TestComingSoon)
	}
	r := p.Rail.(ports.RailRouting)
	if r.RouteFor("", "card", false) != domain.IntegrationU2opia || r.RouteFor("", "abroad", false) != domain.IntegrationU2opia ||
		r.RouteFor("", "mpesa", false) != domain.IntegrationSimulator || r.RouteFor("", "card", true) != "" || r.RouteFor("", "abroad", true) != "" {
		t.Fatal("U2opia takes card and abroad in test mode only")
	}
	if len(p.Abroad) != 2 || p.Abroad[0].Code != "NG" || p.Abroad[1].Code != "ZA" {
		t.Fatalf("countries %+v", p.Abroad)
	}
	// Cards go to a country with a rate, or RIHA doesn't start.
	cfg.U2opiaCardCountry = "AE"
	if _, err := paymentRail(cfg, nil); err == nil {
		t.Fatal("a card country without a rate must stop the start")
	}
	cfg.U2opiaCardCountry = "NG"
	for _, rates := range []map[string]string{{}, {"mz": "1"}, {"za": "-1"}, {"za": "x"}} {
		cfg.U2opiaRates = rates
		if _, err := paymentRail(cfg, nil); err == nil {
			t.Fatalf("rates %v must stop the start", rates)
		}
	}
}
