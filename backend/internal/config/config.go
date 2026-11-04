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
	Storage              string
	R2AccountID          string
	R2AccessKey          string
	R2SecretKey          string
	R2Bucket             string
	R2Prefix             string
	R2Endpoint           string
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
		Storage:              storage(dev),
		R2AccountID:          env("RIHA_R2_ACCOUNT_ID", ""),
		R2AccessKey:          env("RIHA_R2_ACCESS_KEY_ID", ""),
		R2SecretKey:          env("RIHA_R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:             env("RIHA_R2_BUCKET", ""),
		R2Prefix:             env("RIHA_R2_PREFIX", ""),
		R2Endpoint:           env("RIHA_R2_ENDPOINT", ""),
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
