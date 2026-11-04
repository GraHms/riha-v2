package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	rail, realMoney, err := paymentRail(cfg)
	if err != nil {
		log.Fatalf("payments: %v", err)
	}
	deps := Deps(store, cfg)
	deps.Rail = rail
	a := app.New(deps, cfg.PublicURL)
	a.PayURL = cfg.PayURL
	a.AllowPrivateNetworks = cfg.AllowPrivateNetworks
	if realMoney {
		a.ComingSoon = []string{domain.MethodEmola, domain.MethodCard}
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

	srv := &http.Server{Addr: cfg.Addr, Handler: httpapi.New(a, httpapi.Config{StaticDir: cfg.StaticDir, CookieSecure: cfg.CookieSecure, TrustedProxies: cfg.TrustedProxies}),
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

// paymentRail sends live money to M-Pesa, while test mode stays on the simulator.
// Production can't start on the simulator: it would pass fake money off as real.
func paymentRail(cfg config.Config) (ports.Rail, bool, error) {
	test := rails.NewMock(time.Duration(cfg.SimulatorDelayMs) * time.Millisecond)
	if cfg.MpesaAPIKey == "" {
		if cfg.Env == "production" {
			return nil, false, errors.New("production needs RIHA_MPESA_API_KEY, RIHA_MPESA_PUBLIC_KEY and RIHA_MPESA_SERVICE_PROVIDER_CODE")
		}
		return test, false, nil
	}
	live, err := rails.NewMpesa(rails.MpesaConfig{BaseURL: cfg.MpesaBaseURL, C2BPort: cfg.MpesaC2BPort, QueryPort: cfg.MpesaQueryPort, APIKey: cfg.MpesaAPIKey,
		PublicKey: cfg.MpesaPublicKey, ServiceProviderCode: cfg.MpesaProviderCode})
	if err != nil {
		return nil, false, err
	}
	return rails.Modes{Test: test, Live: live}, true, nil
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
