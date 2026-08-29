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
	redisadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/redis"
	"github.com/ghanageo/ghanageo/services/api/internal/adapters/search/typesense"
	appdataset "github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
	gqlserver "github.com/ghanageo/ghanageo/services/api/internal/transport/graphql"
	grpcserver "github.com/ghanageo/ghanageo/services/api/internal/transport/grpc"
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

	const datasetVersion = "2026.08.1-seed"

	geo := appgeo.NewService(
		mongoadapter.NewRegionRepo(store),
		mongoadapter.NewDistrictRepo(store),
		mongoadapter.NewPlaceRepo(store),
		mongoadapter.NewRedirectRepo(store),
		datasetVersion,
	)

	searchSvc := appsearch.NewService(
		typesense.New(cfg.TypesenseURL, cfg.TypesenseKey),
		mongoadapter.NewRegionRepo(store),
		mongoadapter.NewDistrictRepo(store),
		mongoadapter.NewPlaceRepo(store),
		datasetVersion,
	)

	// Fair-use limiting. Read APIs fail OPEN: a limiter outage must not take
	// down a free public service (agent_plan.md §24).
	limiter, err := redisadapter.NewLimiter(cfg.RedisURL, true)
	if err != nil {
		return err
	}
	defer limiter.Close()
	if err := limiter.Ping(ctx); err != nil {
		log.Warn("redis unreachable at startup; fair-use limiting will run degraded", "err", err)
	}

	authenticator := auth.New(
		mongoadapter.NewKeyRepo(store),
		limiterAdapter{limiter},
		cfg.Env == "sandbox",
	)

	datasetSvc := appdataset.NewService(
		mongoadapter.NewDatasetRepo(store),
		cfg.ExportDir,
	)

	// One mux so REST and GraphQL share a port, the same authenticator and the
	// same fair-use accounting. A separate GraphQL server would be a second
	// place for those to drift.
	restHandler := rest.New(geo, searchSvc, log, cfg.AllowedOrigins).
		WithAuth(authenticator).
		WithStore(store).
		WithGraphQL(gqlserver.NewHandler(geo, searchSvc)).
		WithDatasets(datasetSvc).
		Routes()

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           restHandler,
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

	// gRPC on its own port, over the SAME application services. It is a third
	// transport, not a second implementation — the published contract in
	// proto/ has been claimed on the marketing site, so it has to be real.
	grpcSrv := grpcserver.NewServer(geo, searchSvc, store, log)
	go func() {
		if err := grpcserver.Serve(ctx, ":"+cfg.GRPCPort, grpcSrv, log); err != nil {
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

// limiterAdapter bridges the Redis limiter's Decision to the auth package's,
// so neither package needs to import the other.
type limiterAdapter struct{ l *redisadapter.Limiter }

func (a limiterAdapter) Allow(
	ctx context.Context, id identity.Identity, al identity.Allowance, cost identity.CostClass,
) (auth.Decision, error) {
	d, err := a.l.Allow(ctx, id, al, cost)
	if err != nil {
		return auth.Decision{}, err
	}
	return auth.Decision{
		Allowed: d.Allowed, Remaining: d.Remaining, Limit: d.Limit,
		RetryAfter: d.RetryAfter, Degraded: d.Degraded,
	}, nil
}
