# GhanaGeo operational runbooks

These procedures cover the V1 read API and dataset publishing pipeline. The
incident commander records every command, timestamp, actor and decision in the
incident log; secrets and raw authorization headers never enter that log.

## Incident response

1. Declare severity and appoint an incident commander and communications owner.
2. Confirm impact using health, request/error/latency, datastore and pipeline
   signals. Preserve request IDs and the deployment and dataset versions.
3. Contain with the smallest reversible action: disable a release, isolate an
   unhealthy instance, tighten an edge rule, or pause publishing.
4. Publish an initial status update, then update at least every 30 minutes.
5. Recover, verify SLOs and representative REST/GraphQL/gRPC requests, and
   monitor for recurrence before resolving the incident.
6. Complete a blameless review within two working days with owners and dates.

## API-key compromise

1. Revoke the prefix immediately; never request or paste the secret.
2. Search the immutable audit chain and redacted request logs by safe prefix,
   organization, application, origin and source IP.
3. Block abusive origins/IPs at the edge when necessary and notify the owner.
4. Create a replacement key with least-privilege scopes and copy-once delivery.
5. Verify the revoked key returns `KEY_REVOKED` on REST, GraphQL and gRPC.

## Bad dataset release

1. Pause the publishing worker and identify the last known-good version.
2. Preserve the rejected version and validation report; never edit a published
   artifact in place.
3. Execute the audited rollback operation to repoint `current` to the prior
   immutable version, then rebuild search indexes from that version.
4. Verify checksums, record counts, stable-ID redirects and golden queries.
5. Resume publishing only after review; publish a changelog correction.

## ETL failure

1. Stop promotion while leaving the current published dataset available.
2. Inspect the staged run, rejection report, source licence and source checksum.
3. Retry only idempotent stages. A changed source starts a new run identifier.
4. Quarantine invalid rows rather than weakening validators or provenance rules.
5. Re-run acceptance, duplicate, spatial and stable-ID checks before promotion.

## Quota or abuse incident

1. Determine whether Redis limiting is healthy and whether traffic is a single
   caller, distributed abuse, or legitimate public demand.
2. Apply temporary edge controls by IP/origin/fingerprint; do not create a paid
   bypass. Elevated limits require documented public-interest need.
3. Confirm REST, GraphQL and gRPC return their stable rate-limit errors and that
   health/readiness probes remain outside consumer quota.
4. Record the event in the audit/security stream and tune limits only from
   measured capacity evidence.

## Security alert delivery

1. Configure an HTTPS receiver with `SECURITY_ALERT_WEBHOOK_URL` and store a
   distinct generated signing key in `SECURITY_ALERT_WEBHOOK_SECRET`.
2. Verify `X-GhanaGeo-Signature` as HMAC-SHA256 over the raw request body before
   parsing or routing an alert. Reject missing or invalid signatures.
3. Route `security.admin.authentication_failed` and
   `security.key.unusual_usage` to the on-call security channel. Aggregate
   `security.quota.spike` by actor and source IP before paging.
4. Alert on the structured `security alert delivery failed` log event so a
   broken incident-provider integration cannot fail silently.
5. During an incident, correlate the safe actor ID or key prefix with the
   immutable audit chain. Never paste a key secret into the alert receiver.

## Backup and restoration drill

Production uses MongoDB Atlas continuous backup with point-in-time recovery.
At least quarterly, restore a selected recovery point into an isolated scratch
cluster, compare collection counts and validators, run dataset acceptance and
API smoke tests, record RPO/RTO, then delete the scratch cluster.

For a reproducible local proof of the restore mechanics:

```sh
./scripts/restore-drill.sh
```

The script creates a compressed Mongo archive, restores it only into a uniquely
named `ghanageo_restore_drill_*` database, compares every collection count and
schema-validator coverage, prints the archive SHA-256, and removes the scratch
database. It never drops or writes to the source database.
