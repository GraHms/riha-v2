package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	PublicURL    string // where the frontend is served (checkout links)
	APIURL       string // public base URL of the REST API
	StaticDir    string // built frontend (optional)
	UploadDir    string
	CookieSecure bool
	Seed         bool
	// SimulatorDelayMs controls how long the mock wallet waits before the
	// "customer" confirms the PIN on their phone.
	SimulatorDelayMs int
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func Load() Config {
	delay, _ := strconv.Atoi(env("RIHA_SIM_DELAY_MS", "4000"))
	return Config{
		Addr:             env("RIHA_ADDR", ":8080"),
		DatabaseURL:      env("DATABASE_URL", "postgres://riha:riha@localhost:5432/riha?sslmode=disable"),
		PublicURL:        env("RIHA_PUBLIC_URL", "http://localhost:5173"),
		APIURL:           env("RIHA_API_URL", "http://localhost:8080"),
		StaticDir:        env("RIHA_STATIC_DIR", ""),
		UploadDir:        env("RIHA_UPLOAD_DIR", "./data/uploads"),
		CookieSecure:     env("RIHA_COOKIE_SECURE", "false") == "true",
		Seed:             env("RIHA_SEED", "true") == "true",
		SimulatorDelayMs: delay,
	}
}
