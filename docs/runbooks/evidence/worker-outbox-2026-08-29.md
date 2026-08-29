# Transactional outbox and worker evidence — 2026-08-29

## Publication path

The current `2026.08.3-ulid` release was published through the real admin
command after applying the strict `outbox` validator and indexes. The dataset
catalogue transition and event insertion share one MongoDB transaction.

Observed event before the worker started:

- ID: `01M17NCE2PNRTS5DH6J8BSN5ZP`
- Topic: `dataset.published`
- Status: `pending`
- Attempts: `0`
- Payload version: `2026.08.3-ulid`

The worker claimed it once, rebuilt 16,201 Typesense documents in 827 ms and
acknowledged it as `completed`. The final queue contained zero pending, zero
processing and zero dead events.

## Failure behavior

Two isolated unsupported-topic fixtures exercised the real Mongo queue. One
started pending; the other started processing under a deliberately expired
`crashed-worker` lease. Both were reclaimed/retried, stopped at the configured
two-attempt ceiling, moved to `dead`, retained their diagnostic error and
cleared their leases. Both fixtures were removed after verification.

Unit tests separately cover successful acknowledgement, exponential retry and
dead-letter threshold behavior without timing sleeps. Full API and worker
tests/vet pass; Compose and the two-service Render Blueprint validate.
