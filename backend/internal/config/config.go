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
	StaticDir            string
	UploadDir            string
	OutboxDir            string
	CookieSecure         bool
	Seed                 bool
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
	dev := env("RIHA_ENV", "development") == "development"
	public := env("RIHA_PUBLIC_URL", "http://localhost:5173")
	var proxies []string
	for _, p := range strings.Split(env("RIHA_TRUSTED_PROXIES", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return Config{
		Env:                  env("RIHA_ENV", "development"),
		Addr:                 env("RIHA_ADDR", ":8080"),
		DatabaseURL:          env("DATABASE_URL", "postgres://riha:riha@localhost:5432/riha?sslmode=disable"),
		PublicURL:            public,
		StaticDir:            env("RIHA_STATIC_DIR", ""),
		UploadDir:            env("RIHA_UPLOAD_DIR", "./data/uploads"),
		OutboxDir:            env("RIHA_OUTBOX_DIR", "./data/outbox"),
		CookieSecure:         flag("RIHA_COOKIE_SECURE", strings.HasPrefix(public, "https://")),
		Seed:                 flag("RIHA_SEED", false),
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
	}
}
