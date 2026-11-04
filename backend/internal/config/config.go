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
		MailFrom:             env("RIHA_MAIL_FROM", "Riha <no-reply@riha.co.mz>"),
		SMTPAddr:             env("RIHA_SMTP_ADDR", ""),
		SMTPUser:             env("RIHA_SMTP_USER", ""),
		SMTPPassword:         env("RIHA_SMTP_PASSWORD", ""),
	}
}
