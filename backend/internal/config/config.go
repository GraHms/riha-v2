package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Env                  string
	Addr                 string
	DatabaseURL          string
	PublicURL            string
	PayURL               string // where payment pages are shared from; the main site if unset
	StaticDir            string
	UploadDir            string
	OutboxDir            string
	CookieSecure         bool
	Seed                 bool
	CatalogSubdomains    bool
	BackupStatusFile     string
	SimulatorDelayMs     int
	AllowPrivateNetworks bool
	TrustedProxies       []string
	AdminEmail           string
	AdminPassword        string
	MailFrom             string
	AdminNotifyEmail     string
	MpesaBaseURL         string
	MpesaC2BPort         string
	MpesaQueryPort       string
	MpesaAPIKey          string
	MpesaPublicKey       string
	MpesaProviderCode    string
	SMTPAddr             string
	SMTPUser             string
	SMTPPassword         string
	Storage              string
	R2AccountID          string
	R2AccessKey          string
	R2SecretKey          string
	R2Bucket             string
	R2Prefix             string
	R2Endpoint           string
	MtocaEnabled         bool
	MtocaLiveURL         string
	MtocaTestURL         string
	MtocaLiveToken       string
	MtocaTestToken       string
	MtocaChannels        map[string]string
	MtocaDebug           bool
	MpesaViaMtoca        bool
	MtocaTestOn          bool
	// MozeSMS sends live SMS when its key and secret are set; without them, SMS stay
	// in the outbox.
	MozeSMSURL           string
	MozeSMSKey           string
	MozeSMSSecret        string
	MozeSMSSender        string
	MozeSMSSenders       []string
	MozeSMSLowBalance    float64
	MozeSMSAccents       bool
	MozeSMSWebhookSecret string
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func flag(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v == "true" || v == "1"
}

func Load() Config {
	delay, _ := strconv.Atoi(env("RIHA_SIM_DELAY_MS", "4000"))
	dev := env("RIHA_ENV", "production") == "development"
	public := env("RIHA_PUBLIC_URL", "http://localhost:5173")
	var proxies []string
	for _, p := range strings.Split(env("RIHA_TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return Config{
		Env:               env("RIHA_ENV", "production"),
		Addr:              env("RIHA_ADDR", ":8080"),
		DatabaseURL:       env("DATABASE_URL", "postgres://riha:riha@localhost:5432/riha?sslmode=disable"),
		PublicURL:         public,
		PayURL:            env("RIHA_PAY_URL", public),
		StaticDir:         env("RIHA_STATIC_DIR", ""),
		UploadDir:         env("RIHA_UPLOAD_DIR", "./data/uploads"),
		OutboxDir:         env("RIHA_OUTBOX_DIR", "./data/outbox"),
		CookieSecure:      flag("RIHA_COOKIE_SECURE", strings.HasPrefix(public, "https://")),
		Seed:              flag("RIHA_SEED", false),
		CatalogSubdomains: flag("RIHA_CATALOG_SUBDOMAINS", false),
		// Written by the server's nightly backup into the data volume; in production
		// the alerts say when it is missing or old.
		BackupStatusFile:     env("RIHA_BACKUP_STATUS_FILE", map[bool]string{true: "/app/data/backup/last-success", false: ""}[env("RIHA_ENV", "production") == "production"]),
		SimulatorDelayMs:     delay,
		AllowPrivateNetworks: flag("RIHA_ALLOW_PRIVATE_NETWORKS", dev),
		TrustedProxies:       proxies,
		AdminEmail:           env("RIHA_ADMIN_EMAIL", ""),
		AdminPassword:        env("RIHA_ADMIN_PASSWORD", ""),
		MailFrom:             env("RIHA_MAIL_FROM", "RIHA <no-reply@rihagateway.com>"),
		AdminNotifyEmail:     env("RIHA_ADMIN_NOTIFY_EMAIL", ""),
		MpesaBaseURL:         env("RIHA_MPESA_BASE_URL", "https://api.vm.co.mz"),
		MpesaC2BPort:         env("RIHA_MPESA_C2B_PORT", "18352"),
		MpesaQueryPort:       env("RIHA_MPESA_QUERY_PORT", "18353"),
		MpesaAPIKey:          env("RIHA_MPESA_API_KEY", ""),
		MpesaPublicKey:       env("RIHA_MPESA_PUBLIC_KEY", ""),
		MpesaProviderCode:    env("RIHA_MPESA_SERVICE_PROVIDER_CODE", ""),
		SMTPAddr:             env("RIHA_SMTP_ADDR", ""),
		SMTPUser:             env("RIHA_SMTP_USER", ""),
		SMTPPassword:         env("RIHA_SMTP_PASSWORD", ""),
		Storage:              storage(dev),
		R2AccountID:          env("RIHA_R2_ACCOUNT_ID", ""),
		R2AccessKey:          env("RIHA_R2_ACCESS_KEY_ID", ""),
		R2SecretKey:          env("RIHA_R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:             env("RIHA_R2_BUCKET", ""),
		R2Prefix:             env("RIHA_R2_PREFIX", ""),
		R2Endpoint:           env("RIHA_R2_ENDPOINT", ""),
		MtocaEnabled:         flag("RIHA_MTOCA_ENABLED", false),
		MtocaLiveURL:         env("RIHA_MTOCA_LIVE_URL", "https://api.payments.mtoca.net"),
		MtocaTestURL:         env("RIHA_MTOCA_TEST_URL", "https://api.payments.dev.mtoca.net"),
		MtocaLiveToken:       env("RIHA_MTOCA_LIVE_TOKEN", ""),
		MtocaTestToken:       env("RIHA_MTOCA_TEST_TOKEN", ""),
		MtocaChannels:        pairs(env("RIHA_MTOCA_CHANNELS", "mpesa=MPESA,emola=EMOLA,bim=BIM")),
		MtocaDebug:           flag("RIHA_MTOCA_DEBUG", false),
		MpesaViaMtoca:        flag("RIHA_MPESA_VIA_MTOCA", false),
		MtocaTestOn:          flag("RIHA_MTOCA_TEST_ON", true),
		MozeSMSURL:           env("RIHA_MOZESMS_URL", "https://api.mozesms.com"),
		MozeSMSKey:           env("RIHA_MOZESMS_API_KEY", ""),
		MozeSMSSecret:        env("RIHA_MOZESMS_API_SECRET", ""),
		MozeSMSSender:        env("RIHA_MOZESMS_SENDER", ""),
		MozeSMSSenders:       list(env("RIHA_MOZESMS_SENDERS", "")),
		MozeSMSLowBalance:    number(env("RIHA_MOZESMS_LOW_BALANCE", "100")),
		MozeSMSAccents:       flag("RIHA_MOZESMS_ACCENTS", false),
		MozeSMSWebhookSecret: env("RIHA_MOZESMS_WEBHOOK_SECRET", ""),
	}
}

func storage(dev bool) string {
	if v := strings.ToLower(env("RIHA_STORAGE", "")); v == "r2" || v == "local" {
		return v
	}
	if os.Getenv("RIHA_R2_BUCKET") != "" || !dev {
		return "r2"
	}
	return "local"
}

func pairs(s string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if ok && strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
			out[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	return out
}

func list(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func number(v string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
	return f
}
