// Package observability owns tracing and bounded-cardinality operational metrics.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	adminops "github.com/ghanageo/ghanageo/services/api/internal/domain/adminops"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry struct {
	registry     *prometheus.Registry
	provider     *sdktrace.TracerProvider
	requests     *prometheus.CounterVec
	latency      *prometheus.HistogramVec
	deps         *prometheus.HistogramVec
	rate         *prometheus.CounterVec
	cache        *prometheus.CounterVec
	queue        *prometheus.GaugeVec
	etlLag       prometheus.Histogram
	indexLag     prometheus.Histogram
	requestCount atomic.Int64
	errorCount   atomic.Int64
	cacheHits    atomic.Int64
	cacheMisses  atomic.Int64
}

func New(ctx context.Context, serviceName, version string, log *slog.Logger) (*Telemetry, error) {
	provider, err := tracerProvider(ctx, serviceName, version)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	registry := prometheus.NewRegistry()
	t := &Telemetry{
		registry: registry, provider: provider,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ghanageo_requests_total", Help: "Completed requests by protocol, bounded operation and status."}, []string{"protocol", "operation", "status"}),
		latency:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ghanageo_request_duration_seconds", Help: "Request latency by protocol and bounded operation.", Buckets: []float64{.01, .025, .05, .1, .15, .2, .35, .5, 1, 2, 5}}, []string{"protocol", "operation"}),
		deps:     prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ghanageo_dependency_duration_seconds", Help: "Mongo, Redis and search dependency latency.", Buckets: prometheus.DefBuckets}, []string{"dependency", "operation", "result"}),
		rate:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ghanageo_rate_limit_decisions_total", Help: "Fair-use limiter outcomes."}, []string{"result"}),
		cache:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ghanageo_cache_operations_total", Help: "Cache hits and misses by bounded cache name."}, []string{"cache", "result"}),
		queue:    prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "ghanageo_outbox_depth", Help: "Transactional outbox depth by state."}, []string{"status"}),
		etlLag:   prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ghanageo_etl_lag_seconds", Help: "Time from publication enqueue to worker processing start.", Buckets: []float64{.25, .5, 1, 2, 5, 10, 30, 60, 300}}),
		indexLag: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ghanageo_index_lag_seconds", Help: "Time from publication event creation to completed search rebuild.", Buckets: []float64{.25, .5, 1, 2, 5, 10, 30, 60, 300}}),
	}
	registry.MustRegister(t.requests, t.latency, t.deps, t.rate, t.cache, t.queue, t.etlLag, t.indexLag, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	log.Info("observability ready", "service", serviceName, "trace_exporter", exporterName())
	return t, nil
}

func tracerProvider(ctx context.Context, serviceName, version string) (*sdktrace.TracerProvider, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName(serviceName), semconv.ServiceVersion(version),
		attribute.String("deployment.environment", env("GHANAGEO_ENV", "local")),
	))
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}
	opts := []sdktrace.TracerProviderOption{sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample()))}
	switch exporterName() {
	case "otlp":
		exporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	case "stdout":
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("create stdout trace exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}

func exporterName() string {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER")))
	if value == "otlp" || value == "stdout" {
		return value
	}
	return "none"
}

func (t *Telemetry) Shutdown(ctx context.Context) error { return t.provider.Shutdown(ctx) }
func (t *Telemetry) Handler() http.Handler {
	return promhttp.HandlerFor(t.registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func (t *Telemetry) ObserveRequest(protocol, operation, status string, elapsed time.Duration) {
	t.requests.WithLabelValues(protocol, operation, status).Inc()
	t.latency.WithLabelValues(protocol, operation).Observe(elapsed.Seconds())
	t.requestCount.Add(1)
	serverError := strings.HasPrefix(status, "5")
	if protocol == "grpc" {
		switch status {
		case "Internal", "Unknown", "Unavailable", "DataLoss":
			serverError = true
		}
	}
	if serverError {
		t.errorCount.Add(1)
	}
}
func (t *Telemetry) ObserveDependency(dependency, operation, result string, elapsed time.Duration) {
	t.deps.WithLabelValues(dependency, operation, result).Observe(elapsed.Seconds())
}
func (t *Telemetry) ObserveRateLimit(result string) { t.rate.WithLabelValues(result).Inc() }
func (t *Telemetry) ObserveCache(cache, result string) {
	t.cache.WithLabelValues(cache, result).Inc()
	if result == "hit" {
		t.cacheHits.Add(1)
	}
	if result == "miss" {
		t.cacheMisses.Add(1)
	}
}

// AdminMetrics is a bounded process-lifetime aggregate for the authenticated
// health UI. It intentionally exposes neither Prometheus labels nor routes.
func (t *Telemetry) AdminMetrics() adminops.RuntimeMetrics {
	return adminops.RuntimeMetrics{RequestCount: t.requestCount.Load(), ErrorCount: t.errorCount.Load(), CacheHits: t.cacheHits.Load(), CacheMisses: t.cacheMisses.Load()}
}
func (t *Telemetry) SetQueueDepth(pending, processing, dead int64) {
	t.queue.WithLabelValues("pending").Set(float64(pending))
	t.queue.WithLabelValues("processing").Set(float64(processing))
	t.queue.WithLabelValues("dead").Set(float64(dead))
}
func (t *Telemetry) ObserveIndexLag(elapsed time.Duration) { t.indexLag.Observe(elapsed.Seconds()) }
func (t *Telemetry) ObserveETLLag(elapsed time.Duration)   { t.etlLag.Observe(elapsed.Seconds()) }

func TraceID(ctx context.Context) string {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

type mongoSpan struct {
	span trace.Span
}

// MongoMonitor adds child spans and dependency latency without recording
// commands or arguments, which can contain secrets or user query text.
func (t *Telemetry) MongoMonitor() *event.CommandMonitor {
	var spans sync.Map
	key := func(connection string, request int64) string { return fmt.Sprintf("%s/%d", connection, request) }
	return &event.CommandMonitor{
		Started: func(ctx context.Context, event *event.CommandStartedEvent) {
			_, span := otel.Tracer("ghanageo/mongo").Start(ctx, "mongo."+event.CommandName, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attribute.String("db.system", "mongodb"), attribute.String("db.operation.name", event.CommandName)))
			spans.Store(key(event.ConnectionID, event.RequestID), mongoSpan{span: span})
		},
		Succeeded: func(_ context.Context, event *event.CommandSucceededEvent) {
			t.ObserveDependency("mongo", event.CommandName, "ok", event.Duration)
			if value, ok := spans.LoadAndDelete(key(event.ConnectionID, event.RequestID)); ok {
				value.(mongoSpan).span.End()
			}
		},
		Failed: func(_ context.Context, event *event.CommandFailedEvent) {
			t.ObserveDependency("mongo", event.CommandName, "error", event.Duration)
			if value, ok := spans.LoadAndDelete(key(event.ConnectionID, event.RequestID)); ok {
				span := value.(mongoSpan).span
				span.RecordError(event.Failure)
				span.SetStatus(codes.Error, "mongo command failed")
				span.End()
			}
		},
	}
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
