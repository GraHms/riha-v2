package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"riha/internal/adapters/crypto"
	"riha/internal/adapters/filestore"
	"riha/internal/adapters/httpapi"
	"riha/internal/adapters/mail"
	"riha/internal/adapters/messaging"
	"riha/internal/adapters/pdf"
	"riha/internal/adapters/postgres"
	"riha/internal/adapters/rails"
	"riha/internal/adapters/rails/mtoca"
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
	payments, err := paymentRail(cfg)
	if err != nil {
		log.Fatalf("payments: %v", err)
	}
	deps := Deps(store, cfg)
	deps.Rail = payments.Rail
	a := app.New(deps, cfg.PublicURL)
	a.PayURL = cfg.PayURL
	a.AllowPrivateNetworks = cfg.AllowPrivateNetworks
	if payments.RealMoney {
		a.ComingSoon = payments.ComingSoon
		a.NoLiveRefunds = true
	}
	a.AdminNotifyEmail = cfg.AdminNotifyEmail

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := a.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatalf("admin: %v", err)
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

	srv := &http.Server{Addr: cfg.Addr, Handler: httpapi.New(a, httpapi.Config{StaticDir: cfg.StaticDir, CookieSecure: cfg.CookieSecure, TrustedProxies: cfg.TrustedProxies, CatalogSubdomains: cfg.CatalogSubdomains}),
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
}

func Deps(store *postgres.Store, cfg config.Config) app.Deps {
	r := store.Repositories()
	return app.Deps{
		Tx: r.Tx, Users: r.Users, AuthSessions: r.AuthSessions, Merchants: r.Merchants, Documents: r.Documents, Banks: r.Banks,
		Keys: r.Keys, Links: r.Links, Sessions: r.Sessions, Payments: r.Payments, Refunds: r.Refunds, Ledger: r.Ledger,
		Payouts: r.Payouts, Endpoints: r.Endpoints, Domains: r.Domains, Events: r.Events, Deliveries: r.Deliveries,
		RequestLogs: r.RequestLogs, Idempotency: r.Idempotency, Audit: r.Audit, Settings: r.Settings, Reports: r.Reports,
		Plans: r.Plans, Members: r.Members, Subs: r.Subs, Charges: r.Charges, Messages: r.Messages, SubSettings: r.SubSettings,
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
	Rail       ports.Rail
	ComingSoon []string
	RealMoney  bool
}

// paymentRail picks a rail per mode and method. Live M-Pesa goes direct unless
// RIHA_MPESA_VIA_MTOCA is on. With RIHA_MTOCA_ENABLED, the wallets on mtoca use its
// production API for live payments and, unless RIHA_MTOCA_TEST_ON is false, its dev
// API for test payments. Anything else in test mode is simulated. Production can't start on the simulator alone: it would
// pass fake money off as real.
func paymentRail(cfg config.Config) (paymentSetup, error) {
	sim := rails.NewMock(time.Duration(cfg.SimulatorDelayMs) * time.Millisecond)
	test := rails.Router{ByMethod: map[string]ports.Rail{}, Fallback: sim}
	live := rails.Router{ByMethod: map[string]ports.Rail{}}
	if cfg.MtocaEnabled {
		if err := checkMtocaURLs(cfg); err != nil {
			return paymentSetup{}, err
		}
		channels := map[string]string{}
		for method, channel := range cfg.MtocaChannels {
			if domain.IsWallet(method) && (method != domain.MethodMpesa || cfg.MpesaViaMtoca) {
				channels[method] = channel
			}
		}
		liveClient, err := mtoca.New(mtoca.Config{BaseURL: cfg.MtocaLiveURL, Token: cfg.MtocaLiveToken, Channels: channels, Debug: cfg.MtocaDebug})
		if err != nil {
			return paymentSetup{}, err
		}
		testClient, err := mtoca.New(mtoca.Config{BaseURL: cfg.MtocaTestURL, Token: cfg.MtocaTestToken, Channels: channels, Debug: cfg.MtocaDebug})
		if err != nil {
			return paymentSetup{}, err
		}
		for method, channel := range channels {
			live.ByMethod[method] = liveClient
			if cfg.MtocaTestOn {
				test.ByMethod[method] = testClient
			}
			log.Printf("payments: %s goes through mtoca channel %s", method, channel)
		}
	}
	if cfg.MpesaViaMtoca && live.ByMethod[domain.MethodMpesa] == nil {
		return paymentSetup{}, errors.New("RIHA_MPESA_VIA_MTOCA needs RIHA_MTOCA_ENABLED, and mpesa in RIHA_MTOCA_CHANNELS")
	}
	if live.ByMethod[domain.MethodMpesa] == nil {
		if cfg.MpesaAPIKey != "" {
			direct, err := rails.NewMpesa(rails.MpesaConfig{BaseURL: cfg.MpesaBaseURL, C2BPort: cfg.MpesaC2BPort, QueryPort: cfg.MpesaQueryPort, APIKey: cfg.MpesaAPIKey,
				PublicKey: cfg.MpesaPublicKey, ServiceProviderCode: cfg.MpesaProviderCode})
			if err != nil {
				return paymentSetup{}, err
			}
			live.ByMethod[domain.MethodMpesa] = direct
		} else if cfg.Env == "production" {
			return paymentSetup{}, errors.New("production needs RIHA_MPESA_API_KEY, RIHA_MPESA_PUBLIC_KEY and RIHA_MPESA_SERVICE_PROVIDER_CODE, or RIHA_MPESA_VIA_MTOCA")
		}
	}
	if len(live.ByMethod) == 0 {
		return paymentSetup{Rail: sim}, nil
	}
	var soon []string
	for _, method := range domain.AllMethods {
		if live.ByMethod[method] == nil {
			soon = append(soon, method)
		}
	}
	return paymentSetup{Rail: rails.Modes{Test: test, Live: live}, ComingSoon: soon, RealMoney: true}, nil
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
