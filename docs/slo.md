# GhanaGeo service-level objectives

These objectives apply after the public production service has completed its
initial stabilization window. They are operating targets, not a promise that
third-party networks or a caller's own systems will always be available.

## Public read API

- **Availability:** 99.9% per calendar month for REST, GraphQL and gRPC read
  operations. A good event is a request completed without a server-side 5xx or
  gRPC `Internal`, `Unavailable`, or `DeadlineExceeded` result.
- **Latency:** 95% of ordinary reads complete within 350 ms and 99% within
  1 second. Boundary/export downloads are measured separately because payload
  size dominates them.
- Planned maintenance announced at least 48 hours ahead is reported on the
  status page but excluded from the availability calculation.

## Dataset publishing pipeline

- **Availability:** 99.0% of valid publication events complete without reaching
  the dead-letter state.
- **Freshness:** 95% of publication events are searchable within 60 seconds and
  99% within five minutes. The `ghanageo_index_lag_seconds` histogram is the
  source of truth.
- Any dead-letter event or pending event older than five minutes pages the
  operator. Queue state comes from `ghanageo_outbox_depth`.

## Measurement and alerting

Prometheus scrapes `/metrics` on the API and port 9091 on the worker. Request
rate, error ratio, latency percentiles, dependency latency, limiter rejects,
queue depth and index lag are derived from the `ghanageo_*` series. OpenTelemetry
exports correlated traces through OTLP; structured request logs include both
`request_id` and `trace_id` and never include credentials or query contents.

Use multi-window burn-rate alerts for the read API: page when both the one-hour
and five-minute error-budget burn exceed 14.4x, and create a ticket when both
the six-hour and 30-minute burn exceed 6x. A dependency outage is diagnosed by
the bounded Mongo, Redis, and Typesense latency/error labels.

## Incident communication

The public `/status` page checks the live `/health` endpoint and presents the
current pre-launch state honestly. During an incident, post an acknowledgement,
impact and next update time; update at least every 30 minutes; then publish a
short resolution and follow-up. Do not mark the API operational until DNS,
hosting and the production data store have been verified.
