// Command server runs the Riha payment gateway API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"riha/internal/api"
	"riha/internal/config"
	"riha/internal/db"
	"riha/internal/providers"
	"riha/internal/seed"
	"riha/internal/service"
)

func main() {
	cfg := config.Load()
	gdb, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	if err := db.Migrate(gdb); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o750); err != nil {
		log.Fatalf("uploads: %v", err)
	}
	svc := service.New(gdb, cfg, providers.NewMock(time.Duration(cfg.SimulatorDelayMs)*time.Millisecond))
	if cfg.Seed {
		if err := seed.Run(svc); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	svc.RunWorkers(ctx)

	srv := &http.Server{Addr: cfg.Addr, Handler: api.New(svc), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("Riha API listening on %s (checkout at %s)", cfg.Addr, cfg.PublicURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shut, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shut)
}
