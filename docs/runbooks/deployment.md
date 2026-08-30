# Production deployment

## REST and GraphQL on Render

`render.yaml` defines the public HTTP process. It uses the repository
`Dockerfile`, binds Render's assigned `PORT`, persists downloadable exports and
keeps every credential as a dashboard-supplied secret. Sync the Blueprint only
after the quality/security checks pass and all `sync: false` values are ready.

The Blueprint also provisions `ghanageo-redis` as a private, persistent Render
Key Value service in Frankfurt. API and worker `REDIS_URL` values reference its
internal `connectionString` directly, so operators must not copy a Redis URL
into either service. Public access is disabled, the eviction policy is
`noeviction`, and journal-plus-snapshot persistence protects queues and security
counters from ordinary restarts.

CI may lint the Blueprint without deployment credentials using the explicitly
limited mode below. This is not a production deployment gate:

```sh
make production-blueprint-check
```

Before syncing or deploying, run the fail-closed gate with at least one
production dotenv files. It validates API and worker settings independently,
including telemetry, paging and error-reporting gates, and rejects blanks or
placeholders without printing any value. Calling it without either path fails:

```sh
make production-preflight \
  API_ENV=services/api/.env.production \
  WORKER_ENV=services/worker/.env.production
```

The same Blueprint defines `ghanageo-worker` from the same immutable image.
Its command is `/ghanageo-worker`; it consumes the transactional outbox and
rebuilds search after dataset publication or rollback. Verify its queue-depth
log and `/ghanageo-worker health` before enabling public traffic.

The first release must explicitly run these one-time commands from a Render
shell before public DNS is enabled:

```sh
/ghanageo-admin migrate
/ghanageo-admin data seed --file /app/data/seed-data/manifest.json
```

Then publish the approved dataset/export through the normal steward workflow;
do not silently manufacture a production release during container startup.

## Native gRPC on an HTTP/2-capable host

Render does not currently support native gRPC over public external
connections. Deploy the same image to an HTTP/2-native container host (for
example Cloud Run) with:

```text
GHANAGEO_ENV=production
GHANAGEO_SERVE_MODE=grpc
PORT=<platform assigned port>
```

Supply the same Mongo, Redis, Typesense, dataset-version, auth and security
alert configuration as the HTTP process. The binary binds the gRPC server to
`PORT`, publishes the standard gRPC health service and reflection, and does not
start an unused HTTP listener in this mode.

Do not add `grpc.geo.digitalghana.dev` DNS until `grpcurl` proves health,
reflection, anonymous reads, keyed scope enforcement, 429 behavior and the
resumable change stream through the public TLS endpoint.

## Frontends on Vercel

Create four projects from the same repository with these root directories:

| Project | Root | Domain |
|---|---|---|
| Marketing/docs | `apps/web` | `geo.digitalghana.dev` |
| Sandbox | `apps/sandbox` | `sandbox.geo.digitalghana.dev` |
| Developer console | `apps/portal` | `console.geo.digitalghana.dev` |
| Admin | `apps/admin` | `admin.geo.digitalghana.dev` |

Use pnpm 11 and preserve workspace access from the monorepo root. Populate the
generated production environment values in each project. Preview deployments
must use preview API/origin values; never reuse production browser or server
credentials.

Validate each Vercel handoff before deployment. The named service selects the
correct frontend requirements, including a populated `NEXT_PUBLIC_SENTRY_DSN`:

```sh
ruby scripts/production-preflight.rb --env web=apps/web/.env.production
ruby scripts/production-preflight.rb --env sandbox=apps/sandbox/.env.production
ruby scripts/production-preflight.rb --env portal=apps/portal/.env.production
ruby scripts/production-preflight.rb --env admin=apps/admin/.env.production
```

The marketing project additionally requires:

```text
NEXT_PUBLIC_GHANAGEO_WEB_URL=https://geo.digitalghana.dev
NEXT_PUBLIC_GHANAGEO_SANDBOX_URL=https://sandbox.geo.digitalghana.dev
NEXT_PUBLIC_GHANAGEO_PORTAL_URL=https://console.geo.digitalghana.dev
NEXT_PUBLIC_GHANAGEO_INDEXABLE=true
```

Set `NEXT_PUBLIC_GHANAGEO_INDEXABLE=false` on every preview and staging
environment. It controls both the robots metadata and `robots.txt`; only the
canonical production deployment may emit an indexable sitemap. Before switching
it to `true`, prove the final hostname, TLS certificate and redirects, then run:

```sh
GHANAGEO_WEB_AUDIT_URL=https://geo.digitalghana.dev \
GHANAGEO_EXPECTED_ORIGIN=https://geo.digitalghana.dev \
GHANAGEO_EXPECT_INDEXABLE=true pnpm --filter @ghanageo/web seo:check
```

Submit `https://geo.digitalghana.dev/sitemap.xml` in Google Search Console only
after that audit passes. Search Console ownership, crawl discovery and field
Core Web Vitals require the public DNS target and cannot be proven from a local
build.

## Go-live proof

After TLS and DNS settle, run the link checker, the 180-point design matrix,
the cross-protocol journey, the p95 suite and the security abuse suite against
the public hosts. Record exact deployment IDs, commit SHA, dataset version and
test timestamps in `docs/runbooks/evidence/`.
