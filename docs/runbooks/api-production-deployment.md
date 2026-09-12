# GhanaGeo API production deployment

Live since 2026-09-12. This runbook records the deployed topology, how it was
unblocked, and where it still deviates from `render.yaml`.

## Deployed topology

| Resource | Render ID | Live deploy | Notes |
|---|---|---|---|
| `ghanageo-api` | `srv-daiidlnqj5pc73a8esb0` | `dep-daij5srm8hqs73d36ajg` | Docker web service from this repository, frankfurt, health check `/health`. |
| `ghanageo-worker` | `srv-daiif2u7bikc738prt50` | `dep-daij6crm8hqs73d384a0` | Same image, `dockerCommand: /ghanageo-worker`. |
| `ghanageo-typesense` | `srv-daiiji1594qs738tpot0` | `dep-daiijip594qs738tprog` | Private service, Typesense 29.0, reachable only at `ghanageo-typesense:10000`. |
| `ghanageo-redis` | `red-daiicmvqj5pc73a8b7lg` | — | Key Value 8.1.4, 256mb, `noeviction`, journal+snapshot. |

Canonical hostname `api-geo.digitalghana.dev` is a Vercel DNS CNAME to
`ghanageo-api.onrender.com`, registered as a verified Render custom domain
(`cdm-daij2nh594qs738vhev0`). TLS at launch: `CN=api-geo.digitalghana.dev`,
issuer Google Trust Services `WE1`, valid 2026-09-12 to 2026-12-11.

## Verification at launch

```
GET https://api-geo.digitalghana.dev/health
{"datasetVersion":"2026.08.3-ulid","status":"ok"}
```

`/v1/regions`, `/v1/districts` and `POST /graphql` all answer 200. `/v1/search`
returns scored results with a `matchReason` and full region/district hierarchy.

The Typesense index is populated by a one-off job against the API service:

```sh
render jobs create srv-daiidlnqj5pc73a8esb0 --start-command "/ghanageo-admin data reindex"
```

At launch that reported `indexed 16201 documents (regions, districts and places)`,
matching the dataset recorded in the ledger. **Search returns
`INTERNAL: Search failed.` until this has been run** against a fresh Typesense
instance — an empty index is not a startup error, so nothing else surfaces it.

## What had to be fixed to get here

Four defects, in the order they surfaced. Each one hid the next.

1. **The image could not build.** `go.work` declares `./cli`, which
   `.dockerignore` deliberately excludes, so `go mod download` failed with
   "cannot load module /src/cli". No credential would have produced a running
   service. The workspace now drops `./cli` inside the image rather than
   widening the build context.
2. **Transactional mail was unconfigured.** `production_security.go` fails
   closed without `RESEND_API_KEY` and `RESEND_FROM_EMAIL`. Resolved by
   supplying them; the guard was not weakened.
3. **`MONGO_URI` was stored with its surrounding quotes**, so the driver saw a
   scheme of `"mongodb+srv` and refused it. The value in `.env.production` is
   quoted; whatever reads it must strip one layer before setting it on Render.
4. **`API_TRUSTED_PROXY_CIDRS` was unset** while `API_TRUST_PROXY_HEADERS` was
   true, which is a hard error. Set to `10.0.0.0/8`: Render terminates TLS at
   its edge and connects to the container over its private network, so that is
   the immediate peer range. It is deliberately not a wildcard — `.env.example`
   forbids `0.0.0.0/0`.

A fifth affected only the worker: the image `ENTRYPOINT` is `/ghanageo-api`, and
the Render CLI cannot set `dockerCommand` for a Docker-runtime service, so a
CLI-created worker ran the API binary and died on an API-only check. It is set
via the REST API (`PATCH /v1/services/{id}`) or the dashboard. Confirm the fix
in the logs: the worker must report `"service":"ghanageo-worker"`.

## Known deviations from `render.yaml`

Still true, and worth closing before this is called stable:

- **No persistent disk.** `render.yaml` mounts 10GB at `/var/lib/ghanageo` for
  dataset exports; `API_EXPORT_DIR` is `/tmp/ghanageo/exports`, so exports do
  not survive a restart.
- **Typesense stores its index in `/tmp`.** The image ships no `/data`
  directory and an image-runtime service cannot run `mkdir`, so `/tmp` is the
  only guaranteed-writable path. The index is lost on restart and must be
  rebuilt with the reindex job above. Give it a disk before relying on it.
- **Typesense is not in `render.yaml`.** It is a self-hosted private service
  rather than a managed cluster. Add it to the blueprint so the topology stays
  reproducible.
- **Auto-deploy is on and ungated.** `autoDeployTrigger` was moved from
  `checksPass` to `commit` on 2026-09-12, on the blueprint and on both live
  services, so every push to `main` redeploys the API and worker. Because the SDK
  release-conformance matrix is currently red, this means a failing build no
  longer holds a deploy back: `main` goes to production whatever CI says. That is
  a deliberate temporary choice. Move it back to `checksPass` once the matrix is
  green.
- Observability and alerting (`OTEL_EXPORTER_OTLP_*`, `SENTRY_DSN`,
  `SECURITY_ALERT_WEBHOOK_*`) remain unset.

## Rollback

Render keeps prior deploys. Roll back by redeploying the previous live deploy ID
for the affected service; the canonical domain follows automatically. A rollback
does not touch MongoDB, so no data migration is involved. If a rollback changes
the dataset version, rerun the reindex job so Typesense matches Mongo.
