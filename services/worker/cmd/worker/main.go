// Command worker consumes GhanaGeo's transactional outbox.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	"github.com/ghanageo/ghanageo/services/api/internal/adapters/search/typesense"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/outbox"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/observability"
	"github.com/ghanageo/ghanageo/services/api/worker/internal/runner"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("worker stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := config.Load()
	telemetry, err := observability.New(ctx, "ghanageo-worker", cfg.DatasetVersion, log)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = telemetry.Shutdown(closeCtx)
	}()
	store, err := mongoadapter.Connect(ctx, cfg.MongoURI, cfg.MongoDB, telemetry.MongoMonitor())
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = store.Close(closeCtx)
	}()
	queue := mongoadapter.NewOutboxRepo(store)
	if len(os.Args) > 1 && os.Args[1] == "health" {
		depth, err := queue.Depth(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("pending=%d processing=%d dead=%d\n", depth.Pending, depth.Processing, depth.Dead)
		return nil
	}
	if err := mongoadapter.Migrate(ctx, store.DB()); err != nil {
		return err
	}

	searchService := appsearch.NewService(
		typesense.New(cfg.TypesenseURL, cfg.TypesenseKey, telemetry),
		mongoadapter.NewRegionRepo(store), mongoadapter.NewDistrictRepo(store), mongoadapter.NewPlaceRepo(store),
		cfg.DatasetVersion,
	)
	handle := func(ctx context.Context, event outbox.Event) error {
		telemetry.ObserveETLLag(time.Since(event.CreatedAt))
		switch event.Topic {
		case outbox.TopicDatasetPublished:
			count, err := searchService.Reindex(ctx)
			if err == nil {
				telemetry.ObserveIndexLag(time.Since(event.CreatedAt))
				log.Info("search index rebuilt", "event_id", event.ID, "dataset_version", event.Payload["version"], "documents", count)
			}
			return err
		default:
			return fmt.Errorf("unsupported outbox topic %q", event.Topic)
		}
	}
	host, _ := os.Hostname()
	workerID := fmt.Sprintf("%s-%d", host, os.Getpid())
	processor := runner.New(queue, handle, log, workerID,
		durationEnv("WORKER_POLL_INTERVAL", time.Second),
		durationEnv("WORKER_LEASE", 5*time.Minute),
		durationEnv("WORKER_RETRY_BASE", 5*time.Second),
		intEnv("WORKER_MAX_ATTEMPTS", 5),
	)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", telemetry.Handler())
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	metricsServer := &http.Server{Addr: ":" + stringEnv("WORKER_METRICS_PORT", "9091"), Handler: metricsMux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("worker metrics listening", "addr", metricsServer.Addr)
		if serveErr := metricsServer.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			log.Error("worker metrics stopped", "err", serveErr)
			stop()
		}
	}()
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(closeCtx)
	}()
	go observeQueueDepth(ctx, queue, telemetry, log)
	log.Info("worker started", "worker_id", workerID, "dataset_version", cfg.DatasetVersion)
	return processor.Run(ctx)
}

func observeQueueDepth(ctx context.Context, queue interface {
	Depth(context.Context) (outbox.Depth, error)
}, telemetry *observability.Telemetry, log *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		depth, err := queue.Depth(ctx)
		if err != nil {
			log.Warn("observe outbox depth", "err", err)
		} else {
			telemetry.SetQueueDepth(depth.Pending, depth.Processing, depth.Dead)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	if value := os.Getenv(name); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}

func intEnv(name string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(name)); err == nil && value > 0 {
		return value
	}
	return fallback
}

func stringEnv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
