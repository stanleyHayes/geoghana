// Command api serves the GhanaGeo public API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	redisadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/redis"
	"github.com/ghanageo/ghanageo/services/api/internal/adapters/search/typesense"
	appaccount "github.com/ghanageo/ghanageo/services/api/internal/app/account"
	appdataset "github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	appdeveloper "github.com/ghanageo/ghanageo/services/api/internal/app/developer"
	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/observability"
	passkeyrp "github.com/ghanageo/ghanageo/services/api/internal/platform/passkey"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/securityalert"
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
	serveHTTP, serveGRPC, err := enabledTransports(cfg.ServeMode)
	if err != nil {
		return err
	}

	telemetry, err := observability.New(ctx, "ghanageo-api", cfg.DatasetVersion, log)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Shutdown(shutdownCtx)
	}()

	store, err := mongoadapter.Connect(ctx, cfg.MongoURI, cfg.MongoDB, telemetry.MongoMonitor())
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
		cfg.DatasetVersion,
	)

	searchSvc := appsearch.NewService(
		typesense.New(cfg.TypesenseURL, cfg.TypesenseKey, telemetry),
		mongoadapter.NewRegionRepo(store),
		mongoadapter.NewDistrictRepo(store),
		mongoadapter.NewPlaceRepo(store),
		cfg.DatasetVersion,
	)

	// Fair-use limiting. Read APIs fail OPEN: a limiter outage must not take
	// down a free public service (agent_plan.md §24).
	limiter, err := redisadapter.NewLimiter(cfg.RedisURL, true, telemetry)
	if err != nil {
		return err
	}
	defer limiter.Close()
	if err := limiter.Ping(ctx); err != nil {
		log.Warn("redis unreachable at startup; fair-use limiting will run degraded", "err", err)
	}

	securityAlerts := securityalert.New(cfg.SecurityAlertWebhookURL, cfg.SecurityAlertWebhookSecret, log)
	authenticator := auth.New(
		mongoadapter.NewKeyRepo(store),
		limiterAdapter{limiter},
		cfg.Env == "sandbox",
	).WithSecurityAlerts(securityAlerts)

	// Steward writes: permission-checked in the domain and audited, including
	// the attempts that are refused.
	geo = geo.WithMutations(mongoadapter.NewAuditRepo(store))

	datasetSvc := appdataset.NewService(
		mongoadapter.NewDatasetRepo(store),
		cfg.ExportDir,
	)

	accountSvc := appaccount.NewService(
		mongoadapter.NewAccountRepo(store),
		mongoadapter.NewSessionRepo(store),
		mongoadapter.NewOneTimeTokenRepo(store),
		mongoadapter.NewAuditRepo(store),
		nil, // no mailer yet: tokens are logged at WARN for local use
		log,
		"GhanaGeo",
	).WithSecurityAlerts(securityAlerts)
	developerSvc := appdeveloper.NewService(
		mongoadapter.NewOrganizationRepo(store), mongoadapter.NewApplicationRepo(store),
		mongoadapter.NewKeyRepo(store), mongoadapter.NewOrganizationInvitationRepo(store),
		mongoadapter.NewAuditRepo(store),
	)
	usageRepo := mongoadapter.NewUsageRepo(store)

	// WebAuthn. A misconfigured RPID silently breaks every ceremony, so a
	// failure here is logged loudly and passkeys are simply unavailable
	// rather than half-working.
	if rp, perr := passkeyrp.New(passkeyrp.Config{
		RPID:        cfg.PasskeyRPID,
		DisplayName: "GhanaGeo",
		Origins:     cfg.PasskeyOrigins,
	}); perr != nil {
		log.Error("passkeys disabled: invalid WebAuthn configuration", "err", perr)
	} else {
		accountSvc = accountSvc.WithPasskeys(rp, mongoadapter.NewChallengeRepo(store))
		log.Info("passkeys enabled", "rpId", cfg.PasskeyRPID, "origins", cfg.PasskeyOrigins)
	}

	// One mux so REST and GraphQL share a port, the same authenticator and the
	// same fair-use accounting. A separate GraphQL server would be a second
	// place for those to drift.
	restHandler := rest.New(geo, searchSvc, log, cfg.AllowedOrigins).
		WithAuth(authenticator).
		WithStore(store).
		WithGraphQL(gqlserver.NewHandlerWithTelemetry(geo, searchSvc, datasetSvc, telemetry)).
		WithDatasets(datasetSvc).
		WithAccounts(accountSvc).
		WithDeveloper(developerSvc).
		WithUsage(usageRepo).
		WithTelemetry(telemetry).
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

	errCh := make(chan error, 2)
	if serveHTTP {
		go func() {
			log.Info("listening", "transport", "http", "addr", srv.Addr, "env", cfg.Env)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	// gRPC on its own port, over the SAME application services. It is a third
	// transport, not a second implementation — the published contract in
	// proto/ has been claimed on the marketing site, so it has to be real.
	if serveGRPC {
		grpcSrv := grpcserver.NewServer(geo, searchSvc, store, log)
		go func() {
			log.Info("listening", "transport", "grpc", "addr", ":"+cfg.GRPCPort, "env", cfg.Env)
			if err := grpcserver.Serve(ctx, ":"+cfg.GRPCPort, grpcSrv, authenticator, log, usageRepo, telemetry); err != nil {
				errCh <- err
			}
		}()
	}

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

func enabledTransports(mode string) (httpEnabled, grpcEnabled bool, err error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "all", "":
		return true, true, nil
	case "http":
		return true, false, nil
	case "grpc":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("unsupported GHANAGEO_SERVE_MODE %q (want all, http or grpc)", mode)
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
