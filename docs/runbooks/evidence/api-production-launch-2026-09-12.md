# GhanaGeo API production launch evidence

**Launched:** 2026-09-12
**Application commit:** `841cd6c` (`main`)
**Dataset:** `2026.08.3-ulid`
**Lifecycle:** beta — stable is not claimed
**Operational owner:** repository owner `stanleyHayes`

## Canonical surfaces and providers

| Surface | Canonical URL | Provider | Immutable evidence |
|---|---|---|---|
| Public REST/GraphQL API | `https://api-geo.digitalghana.dev` | Render web service `srv-daiidlnqj5pc73a8esb0` | deploy `dep-daij5srm8hqs73d36ajg` |
| Background worker | — | Render background worker `srv-daiif2u7bikc738prt50` | deploy `dep-daij6crm8hqs73d384a0` |
| Search node | `ghanageo-typesense:10000` (private) | Render private service `srv-daiiji1594qs738tpot0` | deploy `dep-daiijip594qs738tprog` |
| Cache / queue | — | Render Key Value `red-daiicmvqj5pc73a8b7lg` | 256mb, `noeviction`, journal+snapshot |
| Primary datastore | — | MongoDB Atlas (`mongodb+srv`) | existing cluster |

The Render provider hostname is `https://ghanageo-api.onrender.com`. The canonical
CNAME is owned in Vercel DNS (`rec_3fe5b2d0538a7ffcef7cffc2`) and the Render custom
domain `cdm-daij2nh594qs738vhev0` is verified.

TLS at launch: subject `CN=api-geo.digitalghana.dev`, issuer Google Trust Services
`WE1`, valid 2026-09-12 through 2026-12-11.

## Smoke evidence

```
GET https://api-geo.digitalghana.dev/health          200
{"datasetVersion":"2026.08.3-ulid","status":"ok"}

GET  /v1/regions                                     200  (source-linked, provenance per record)
GET  /v1/districts?limit=1                           200
POST /graphql  {"query":"{ __typename }"}            200
GET  /v1/search?q=Accra&limit=3                      200  (scored, with matchReason and hierarchy)
GET  /v1/search?q=Kumasi&limit=1                     200  (Kumasi, REGIONAL_CAPITAL, Ashanti)
```

Search index built by a one-off job against the API service
(`/ghanageo-admin data reindex`), which reported
`indexed 16201 documents (regions, districts and places)` — matching the
16,201 active records stamped `2026.08.3-ulid` recorded under GEO-3.1.

Worker identity confirmed from its own logs: `"service":"ghanageo-worker"`.

## Defects fixed to reach launch

1. Container image could not build: `go.work` declares `./cli`, which
   `.dockerignore` excludes. Fixed by dropping `./cli` from the workspace inside
   the image rather than widening the build context.
2. API fail-closed on absent `RESEND_API_KEY` / `RESEND_FROM_EMAIL`. Resolved by
   supplying real values; the guard in `production_security.go` was not weakened,
   and `GHANAGEO_ENV` remains `production`.
3. `MONGO_URI` was stored with its surrounding quotes, so the driver rejected the
   scheme. Corrected to the unquoted `mongodb+srv://` value.
4. `API_TRUSTED_PROXY_CIDRS` was unset while `API_TRUST_PROXY_HEADERS` was true.
   Set to `10.0.0.0/8`, Render's private edge range — explicitly not a wildcard.
5. Worker inherited the image `ENTRYPOINT` (`/ghanageo-api`). `dockerCommand` set
   to `/ghanageo-worker`.

## Known limitations at launch

- Typesense keeps its index in `/tmp` and dataset exports write to `/tmp`; both
  are lost on restart, and search returns `INTERNAL` until a reindex is run.
- Typesense is not represented in `render.yaml`.
- Auto-deploy was off on the API and worker at launch (`autoDeployTrigger:
  checksPass` with a red SDK release-conformance matrix). Later the same day the
  owner moved it to `commit` on the blueprint and both services, so pushes to
  `main` now deploy regardless of CI state.
- Observability and alerting (`OTEL_EXPORTER_OTLP_*`, `SENTRY_DSN`,
  `SECURITY_ALERT_WEBHOOK_*`) are unset.
- Native gRPC on `grpc-geo.digitalghana.dev` is still not deployed.
- Continuous backup and point-in-time restore still require an Atlas M10 tier.

## Rollback

Redeploy the previous live deploy ID for the affected Render service; the
canonical domain follows. No data migration is involved. If the dataset version
changes, rerun the reindex job so Typesense matches MongoDB.
