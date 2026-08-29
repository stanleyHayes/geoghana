# GhanaGeo worker

The worker consumes the MongoDB transactional outbox. Dataset publication and
rollback enqueue `dataset.published` in the same transaction that changes the
live catalogue; the worker then performs an idempotent full search-index
rebuild.

Delivery is at least once. Claims are leased, abandoned work is reclaimed,
failures use bounded exponential retry, and exhausted/unknown work moves to
the `dead` state for operator review. Completed events expire after 30 days;
dead events do not expire automatically.

```sh
go run ./cmd/worker
go run ./cmd/worker health
```

Configuration is environment-only: `MONGO_URI`, `MONGO_DB`, `TYPESENSE_URL`,
`TYPESENSE_API_KEY`, `DATASET_VERSION`, plus optional `WORKER_POLL_INTERVAL`,
`WORKER_LEASE`, `WORKER_RETRY_BASE` and `WORKER_MAX_ATTEMPTS`.
