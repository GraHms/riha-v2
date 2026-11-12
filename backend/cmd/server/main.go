package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"riha/internal/adapters/crypto"
	"riha/internal/adapters/filestore"
	"riha/internal/adapters/httpapi"
	"riha/internal/adapters/mail"
	"riha/internal/adapters/messaging"
	"riha/internal/adapters/netinfo"
	"riha/internal/adapters/pdf"
	"riha/internal/adapters/postgres"
	"riha/internal/adapters/rails"
	"riha/internal/adapters/rails/mtoca"
	"riha/internal/adapters/rails/u2opia"
	"riha/internal/adapters/sheets"
	"riha/internal/adapters/webhooks"
	"riha/internal/config"
	"riha/internal/core/app"
	"riha/internal/core/domain"
	"riha/internal/core/ports"
	"riha/internal/seed"
)

func main() {
	cfg := config.Load()
	store, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := store.Migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	// The trail is written in the background, and what is left when the server
	// stops is written before it exits.
	trail := postgres.NewTrail(store)
	trailCtx, stopTrail := context.WithCancel(context.Background())
	go trail.Run(trailCtx)
	go netinfo.Load()
	payments, err := paymentRail(cfg, trail.Add)
	if err != nil {
		log.Fatalf("payments: %v", err)
	}
	deps := Deps(store, cfg)
	deps.Rail = payments.Rail
	if cfg.MozeSMSKey != "" {
		sms, err := messaging.NewMozeSMS(messaging.MozeSMSConfig{BaseURL: cfg.MozeSMSURL, APIKey: cfg.MozeSMSKey, APISecret: cfg.MozeSMSSecret,
			Sender: cfg.MozeSMSSender, Senders: cfg.MozeSMSSenders, LowBalance: cfg.MozeSMSLowBalance, Accents: cfg.MozeSMSAccents})
		if err != nil {
			log.Fatalf("sms: %v", err)
		}
		deps.Messenger = messaging.Router{SMS: sms, Outbox: deps.Messenger}
		log.Printf("sms: live SMS go through MozeSMS (sender %q); test-mode messages stay in the outbox", domain.FirstNonEmpty(cfg.MozeSMSSender, "MozeSMS default"))
	} else if cfg.Env == "production" {
		log.Printf("sms: RIHA_MOZESMS_API_KEY is not set, so SMS are not sent, only written to the outbox")
	}
	deps.Trail = trail
	deps.Insights = postgres.Insights{Store: store}
	a := app.New(deps, cfg.PublicURL)
	a.PayURL = cfg.PayURL
	a.AllowPrivateNetworks = cfg.AllowPrivateNetworks
	a.LoginCodes = cfg.Env != "development" && os.Getenv("RIHA_LOGIN_CODES") != "off"
	a.ComingSoon, a.TestComingSoon, a.AbroadCountries = payments.ComingSoon, payments.TestComingSoon, payments.Abroad
	if len(payments.Abroad) > 0 {
		for _, m := range cfg.U2opiaMethods {
			if m == domain.MethodCard || m == domain.MethodAbroad {
				a.AbroadMethods = append(a.AbroadMethods, m)
			}
		}
	}
	if payments.RealMoney {
		a.NoLiveRefunds = true
	}
	a.AdminNotifyEmail = cfg.AdminNotifyEmail
	a.BackupStatusFile = cfg.BackupStatusFile

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(os.Args) > 1 && os.Args[1] == "import-v1" {
		code := importV1(ctx, a, os.Args[2:])
		stop()
		os.Exit(code)
	}
	if err := a.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("admin: %v", err)
	}
	if err := a.SetupIntegrations(ctx); err != nil {
		log.Fatalf("integrations: %v", err)
	}
	if cfg.Seed {
		if cfg.Env == "production" {
			log.Fatal("RIHA_SEED cannot be used with RIHA_ENV=production")
		}
		if err := seed.Run(ctx, a); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}
	a.RunWorkers(ctx)

	srv := &http.Server{Addr: cfg.Addr, Handler: httpapi.New(a, httpapi.Config{StaticDir: cfg.StaticDir, CookieSecure: cfg.CookieSecure, TrustedProxies: cfg.TrustedProxies, CatalogSubdomains: cfg.CatalogSubdomains, SMSWebhookSecret: cfg.MozeSMSWebhookSecret}),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second, MaxHeaderBytes: 64 << 10}
	go func() {
		log.Printf("RIHA API listening on %s (checkout at %s)", cfg.Addr, cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
	stopTrail()
	trail.Wait(5 * time.Second)
}

func Deps(store *postgres.Store, cfg config.Config) app.Deps {
	r := store.Repositories()
	return app.Deps{
		Tx: r.Tx, Users: r.Users, AuthSessions: r.AuthSessions, Merchants: r.Merchants, Documents: r.Documents, Banks: r.Banks,
		Keys: r.Keys, Links: r.Links, Sessions: r.Sessions, Payments: r.Payments, Refunds: r.Refunds, Ledger: r.Ledger,
		Payouts: r.Payouts, Endpoints: r.Endpoints, Domains: r.Domains, Events: r.Events, Deliveries: r.Deliveries,
		RequestLogs: r.RequestLogs, Idempotency: r.Idempotency, Audit: r.Audit, Settings: r.Settings, Reports: r.Reports,
		Plans: r.Plans, Members: r.Members, Subs: r.Subs, Charges: r.Charges, Messages: r.Messages, SubSettings: r.SubSettings, Integrations: r.Integrations,
		Messenger: &messaging.Outbox{Dir: filepath.Join(cfg.OutboxDir, "messages")},
		Sheets:    sheets.Reader{},
		Rail:      rails.NewMock(time.Duration(cfg.SimulatorDelayMs) * time.Millisecond),
		Sender:    webhooks.NewHTTPSender(cfg.AllowPrivateNetworks),
		Files:     files(cfg),
		Hasher:    crypto.Bcrypt{},
		Renderer:  pdf.New(),
		Mailer:    mailer(cfg),
		Emails:    r.Emails,
		Resets:    r.Resets,
	}
}

// paymentSetup is the rail, the methods it can't take yet, and whether it moves real money.
type paymentSetup struct {
	Rail           ports.Rail
	ComingSoon     []string
	TestComingSoon []string
	RealMoney      bool
	Abroad         []domain.AbroadCountry
}

// paymentRail sets up each mode's integrations. With RIHA_MTOCA_ENABLED, the
// wallets on mtoca use its production API for live payments and, unless
// RIHA_MTOCA_TEST_ON is false, its dev API for test payments. With the M-Pesa API's
// keys, live M-Pesa can also go direct, through 900511, and does by default
// unless RIHA_MPESA_VIA_MTOCA is on; the backoffice can send a merchant through
// either. Anything else in test mode is simulated. Production can't start on the
// simulator alone: it would pass fake money off as real.
func paymentRail(cfg config.Config, trace func(domain.TrailEvent)) (paymentSetup, error) {
	sim := rails.NewMock(time.Duration(cfg.SimulatorDelayMs) * time.Millisecond)
	test := rails.Router{Via: map[string]rails.Via{domain.IntegrationSimulator: {Rail: sim}}, Default: map[string]string{}, Fallback: domain.IntegrationSimulator}
	live := rails.Router{Via: map[string]rails.Via{}, Default: map[string]string{}}
	if cfg.MtocaEnabled {
		if err := checkMtocaURLs(cfg); err != nil {
			return paymentSetup{}, err
		}
		channels := map[string]string{}
		for method, channel := range cfg.MtocaChannels {
			if domain.IsWallet(method) {
				channels[method] = channel
			}
		}
		liveClient, err := mtoca.New(mtoca.Config{BaseURL: cfg.MtocaLiveURL, Token: cfg.MtocaLiveToken, Channels: channels, Debug: cfg.MtocaDebug, Trace: trace})
		if err != nil {
			return paymentSetup{}, err
		}
		testClient, err := mtoca.New(mtoca.Config{BaseURL: cfg.MtocaTestURL, Token: cfg.MtocaTestToken, Channels: channels, Debug: cfg.MtocaDebug, Trace: trace})
		if err != nil {
			return paymentSetup{}, err
		}
		methods := []string{}
		for _, method := range domain.AllMethods {
			if channels[method] == "" {
				continue
			}
			methods = append(methods, method)
			if method != domain.MethodMpesa || cfg.MpesaViaMtoca {
				live.Default[method] = domain.IntegrationMtoca
				if cfg.MtocaTestOn {
					test.Default[method] = domain.IntegrationMtoca
				}
			}
			log.Printf("payments: mtoca takes %s on channel %s", method, channels[method])
		}
		live.Via[domain.IntegrationMtoca] = rails.Via{Rail: liveClient, Methods: methods}
		if cfg.MtocaTestOn {
			test.Via[domain.IntegrationMtoca] = rails.Via{Rail: testClient, Methods: methods}
		}
	}
	if cfg.MpesaViaMtoca && live.Default[domain.MethodMpesa] == "" {
		return paymentSetup{}, errors.New("RIHA_MPESA_VIA_MTOCA needs RIHA_MTOCA_ENABLED, and mpesa in RIHA_MTOCA_CHANNELS")
	}
	if cfg.MpesaAPIKey != "" {
		direct, err := rails.NewMpesa(rails.MpesaConfig{BaseURL: cfg.MpesaBaseURL, C2BPort: cfg.MpesaC2BPort, QueryPort: cfg.MpesaQueryPort, APIKey: cfg.MpesaAPIKey,
			PublicKey: cfg.MpesaPublicKey, ServiceProviderCode: cfg.MpesaProviderCode})
		if err != nil {
			return paymentSetup{}, err
		}
		live.Via[domain.IntegrationMpesaDirect] = rails.Via{Rail: direct, Methods: []string{domain.MethodMpesa}}
		if live.Default[domain.MethodMpesa] == "" {
			live.Default[domain.MethodMpesa] = domain.IntegrationMpesaDirect
		}
	} else if live.Default[domain.MethodMpesa] == "" && cfg.Env == "production" {
		return paymentSetup{}, errors.New("production needs RIHA_MPESA_API_KEY, RIHA_MPESA_PUBLIC_KEY and RIHA_MPESA_SERVICE_PROVIDER_CODE, or RIHA_MPESA_VIA_MTOCA")
	}
	for method, code := range live.Default {
		log.Printf("payments: live %s goes through %s by default", method, code)
	}
	abroad, err := u2opiaRail(cfg, test, trace)
	if err != nil {
		return paymentSetup{}, err
	}
	// Test mode offers what live mode does, so a merchant tests what they'll
	// get, plus the methods being tried in test first: those U2opia takes.
	testSoon := func(soon []string) []string {
		out := []string{}
		for _, m := range soon {
			if test.Default[m] != domain.IntegrationU2opia {
				out = append(out, m)
			}
		}
		return out
	}
	if len(live.Via) == 0 {
		// The simulator takes everything but a payment from abroad, which
		// only U2opia's page can take.
		soon := []string{domain.MethodAbroad}
		only := rails.Single(domain.IntegrationSimulator, sim)
		return paymentSetup{Rail: rails.Modes{Test: test, Live: only}, ComingSoon: soon, TestComingSoon: testSoon(soon), Abroad: abroad}, nil
	}
	var soon []string
	for _, method := range append(append([]string{}, domain.AllMethods...), domain.OptInMethods...) {
		if live.RouteFor("", method, true) == "" {
			soon = append(soon, method)
		}
	}
	return paymentSetup{Rail: rails.Modes{Test: test, Live: live}, ComingSoon: soon, TestComingSoon: testSoon(soon), RealMoney: true, Abroad: abroad}, nil
}

// u2opiaRail puts U2opia behind cards and wallets from abroad, in test mode
// only, for the countries RIHA_U2OPIA_RATES gives a rate for.
func u2opiaRail(cfg config.Config, test rails.Router, trace func(domain.TrailEvent)) ([]domain.AbroadCountry, error) {
	if !cfg.U2opiaEnabled {
		return nil, nil
	}
	countries := map[string]u2opia.Country{}
	for code, rate := range cfg.U2opiaRates {
		c, ok := u2opia.Known[strings.ToUpper(code)]
		v, err := strconv.ParseFloat(rate, 64)
		if !ok || err != nil || v <= 0 {
			return nil, fmt.Errorf("RIHA_U2OPIA_RATES: %q=%q is not a known country with a positive rate", code, rate)
		}
		c.PerMZN = v
		countries[c.Code] = c
	}
	if len(countries) == 0 {
		return nil, errors.New("RIHA_U2OPIA_ENABLED needs RIHA_U2OPIA_RATES, the countries to offer and their rates")
	}
	client, err := u2opia.New(u2opia.Config{BaseURL: cfg.U2opiaTestURL, PartnerCode: cfg.U2opiaPartnerCode, Countries: countries, Trace: trace})
	if err != nil {
		return nil, err
	}
	methods := []string{}
	for _, m := range cfg.U2opiaMethods {
		if m == domain.MethodCard || m == domain.MethodAbroad {
			methods = append(methods, m)
			test.Default[m] = domain.IntegrationU2opia
			log.Printf("payments: test %s goes through U2opia (%d countries)", m, len(countries))
		}
	}
	test.Via[domain.IntegrationU2opia] = rails.Via{Rail: client, Methods: methods}
	var out []domain.AbroadCountry
	for _, c := range client.Countries() {
		out = append(out, domain.AbroadCountry{Code: c.Code, Name: c.Name, Dial: c.Dial, Currency: c.Currency})
	}
	return out, nil
}

// checkMtocaURLs refuses the mix-ups that move money wrongly: test payments on the
// production API charge people, and live payments on dev credit merchants for nothing.
func checkMtocaURLs(cfg config.Config) error {
	liveURL, testURL := strings.TrimRight(cfg.MtocaLiveURL, "/"), strings.TrimRight(cfg.MtocaTestURL, "/")
	if liveURL == testURL {
		return errors.New("RIHA_MTOCA_LIVE_URL and RIHA_MTOCA_TEST_URL must differ")
	}
	if cfg.Env == "production" {
		if strings.Contains(liveURL, ".dev.") {
			return errors.New("RIHA_MTOCA_LIVE_URL must be mtoca's production API")
		}
		if !strings.HasPrefix(liveURL, "https://") && !privateHost(liveURL) {
			return errors.New("RIHA_MTOCA_LIVE_URL must use https, unless it's on a private network")
		}
		if strings.Contains(testURL, "api.payments.mtoca.net") {
			return errors.New("RIHA_MTOCA_TEST_URL must not be mtoca's production API")
		}
	}
	return nil
}

// privateHost is a container name on the Docker network, or a private address.
func privateHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return false
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		return ip.IsPrivate() || ip.IsLoopback()
	}
	return !strings.Contains(u.Hostname(), ".")
}

func mailer(cfg config.Config) ports.Mailer {
	if cfg.SMTPAddr != "" {
		return mail.SMTP{Addr: cfg.SMTPAddr, Username: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.MailFrom}
	}
	return mail.Outbox{Dir: cfg.OutboxDir, From: cfg.MailFrom}
}

func files(cfg config.Config) ports.FileStore {
	local := filestore.Local{Root: cfg.UploadDir}
	rc := filestore.R2Config{AccountID: cfg.R2AccountID, AccessKey: cfg.R2AccessKey, SecretKey: cfg.R2SecretKey, Bucket: cfg.R2Bucket, Prefix: cfg.R2Prefix, Endpoint: cfg.R2Endpoint}
	if cfg.Storage != "r2" {
		router := filestore.Router{Primary: local, Local: local}
		if rc.Enabled() {
			if r2, err := filestore.NewR2(rc); err == nil {
				router.Remote = r2
			}
		}
		log.Printf("storage: local disk at %s", cfg.UploadDir)
		return router
	}
	r2, err := filestore.NewR2(rc)
	if err != nil {
		log.Fatalf("storage: %v. Set RIHA_R2_ACCOUNT_ID, RIHA_R2_BUCKET, RIHA_R2_ACCESS_KEY_ID and RIHA_R2_SECRET_ACCESS_KEY, or RIHA_STORAGE=local", err)
	}
	if err := r2.Check(context.Background()); err != nil {
		log.Fatalf("storage: cannot reach R2 bucket %s: %v", cfg.R2Bucket, err)
	}
	log.Printf("storage: Cloudflare R2 bucket %s", cfg.R2Bucket)
	return filestore.Router{Primary: r2, Remote: r2, Local: local}
}
