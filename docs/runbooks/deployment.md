# Production deployment

## REST and GraphQL on Render

`render.yaml` defines the public HTTP process. It uses the repository
`Dockerfile`, binds Render's assigned `PORT`, persists downloadable exports and
keeps every credential as a dashboard-supplied secret. Sync the Blueprint only
after the quality/security checks pass and all `sync: false` values are ready.

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

## Go-live proof

After TLS and DNS settle, run the link checker, the 180-point design matrix,
the cross-protocol journey, the p95 suite and the security abuse suite against
the public hosts. Record exact deployment IDs, commit SHA, dataset version and
test timestamps in `docs/runbooks/evidence/`.
