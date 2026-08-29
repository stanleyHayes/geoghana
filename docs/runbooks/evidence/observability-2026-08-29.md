# Observability verification — 2026-08-29

## Implemented

- OpenTelemetry lifecycle for API and worker with `none`, `stdout`, and OTLP
  exporters and W3C trace-context propagation.
- HTTP and gRPC server spans, Mongo command spans, Redis spans with command
  statements disabled, and Typesense client spans.
- Correlated JSON request logs with bounded operation, request/trace IDs,
  application ID and the public key prefix. Credentials and query content are
  never added.
- Private Prometheus registries for request/error/latency histograms, dependency
  latency, GraphQL query/APQ cache hit rate, limiter decisions, outbox depth,
  publication-to-worker ETL lag and publication-to-index lag.
- Worker `/healthz` and `/metrics` endpoints on the configurable metrics port.
- Read-API and publishing-pipeline SLOs plus burn-rate and incident guidance in
  `docs/slo.md`.

## Verification

`go test ./services/api/... ./services/worker/...` and
`go vet ./services/api/... ./services/worker/...` passed. Compose and Render
YAML validation passed, as did `git diff --check`.

A live local API request to `GET /v1/regions?limit=1` returned 413 bytes with
HTTP 200. Its structured log used the bounded operation `GET /v1/regions` and
trace ID `5b080a4be57e625f3df297aa71b20c98`; the exported HTTP, Redis and Mongo child
spans shared that trace ID. The API `/metrics` scrape exposed populated request,
dependency and limiter series.

The live worker served HTTP 204 at `/healthz`. Its metrics scrape exposed
pending, processing and dead outbox gauges, all zero, plus the index-lag
histogram. The worker shut down cleanly on SIGINT.

## External activation still required

The Render Blueprint accepts an OTLP endpoint and headers, but a production
collector/alert receiver cannot be verified before hosting is provisioned.
Likewise, the status page cannot prove public service health until DNS and the
production API exist. These are tracked as external launch gates rather than
represented as completed production monitoring.
