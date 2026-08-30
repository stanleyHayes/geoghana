package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsExposeBoundedOperationalSignals(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	telemetry, err := New(context.Background(), "test", "v1", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = telemetry.Shutdown(context.Background()) })

	telemetry.ObserveRequest("http", "GET /v1/regions", "200", 20*time.Millisecond)
	telemetry.ObserveDependency("mongo", "find", "ok", 5*time.Millisecond)
	telemetry.ObserveRateLimit("allowed")
	telemetry.ObserveCache("graphql_query", "hit")
	telemetry.SetQueueDepth(2, 1, 0)
	telemetry.ObserveETLLag(500 * time.Millisecond)
	telemetry.ObserveIndexLag(time.Second)

	recorder := httptest.NewRecorder()
	telemetry.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()
	for _, metric := range []string{
		"ghanageo_requests_total", "ghanageo_request_duration_seconds",
		"ghanageo_dependency_duration_seconds", "ghanageo_rate_limit_decisions_total",
		"ghanageo_cache_operations_total", "ghanageo_outbox_depth",
		"ghanageo_etl_lag_seconds", "ghanageo_index_lag_seconds",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics output does not contain %s", metric)
		}
	}
}

func TestAdminMetricsExposeOnlyBoundedAggregates(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	telemetry, err := New(context.Background(), "test", "v1", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = telemetry.Shutdown(context.Background()) })
	telemetry.ObserveRequest("http", "GET /secret/path", "200", time.Millisecond)
	telemetry.ObserveRequest("http", "GET /secret/path", "503", time.Millisecond)
	telemetry.ObserveRequest("grpc", "/private.Service/Call", "InvalidArgument", time.Millisecond)
	telemetry.ObserveCache("private_cache", "hit")
	telemetry.ObserveCache("private_cache", "miss")

	got := telemetry.AdminMetrics()
	if got.RequestCount != 3 || got.ErrorCount != 1 || got.CacheHits != 1 || got.CacheMisses != 1 {
		t.Fatalf("unexpected bounded aggregate: %+v", got)
	}
}
