// Command api serves the GhanaGeo public API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
	"github.com/ghanageo/ghanageo/services/api/internal/transport/rest"
)

func main() {
	cfg := config.Load()
	log := newLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func newLogger(level string) *slog.Logger {
	lv := slog.LevelInfo
	if level == "debug" {
		lv = slog.LevelDebug
	}
	// Structured JSON logs with request_id / operation / latency (Spec 20).
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := mongoadapter.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.Close(shutdownCtx)
	}()

	if err := mongoadapter.Migrate(ctx, store.DB()); err != nil {
		return err
	}

	geo := appgeo.NewService(
		mongoadapter.NewRegionRepo(store),
		mongoadapter.NewDistrictRepo(store),
		mongoadapter.NewPlaceRepo(store),
		mongoadapter.NewRedirectRepo(store),
		"2026.08.1-seed",
	)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           rest.New(geo, log, cfg.AllowedOrigins).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // request-size limit (Spec 12.4)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
