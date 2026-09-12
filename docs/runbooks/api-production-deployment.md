# GhanaGeo API production deployment

Status as of 2026-09-12. This runbook records what is provisioned, what is still
blocked, and exactly how to finish. It replaces guesswork about why
`api-geo.digitalghana.dev` does not serve.

## What is provisioned

| Resource | Render ID | State | Notes |
|---|---|---|---|
| `ghanageo-typesense` | `srv-daiiji1594qs738tpot0` | **live** | Private service, Typesense 29.0, frankfurt. Reachable only on Render's internal network at `ghanageo-typesense:10000`. |
| `ghanageo-redis` | `red-daiicmvqj5pc73a8b7lg` | **available** | Key Value 8.1.4, 256mb, frankfurt, `noeviction`, journal+snapshot persistence. |
| `ghanageo-api` | `srv-daiidlnqj5pc73a8esb0` | **build succeeds, start fails** | Docker web service from this repository, frankfurt, health check `/health`. |
| `ghanageo-worker` | `srv-daiif2u7bikc738prt50` | **build succeeds, runs the wrong binary** | See "Worker entrypoint" below. |

The container image builds cleanly on Render. That was not previously true: the
Dockerfile could not build at all until the `go.work`/`.dockerignore` conflict
was fixed, so no credential would have produced a running service before.

## Blocker 1 — transactional mail (blocks the API)

The API starts, validates its production configuration and exits:

    {"level":"ERROR","msg":"fatal","err":"transactional mail configuration missing: RESEND_API_KEY, RESEND_FROM_EMAIL"}

This is deliberate. `services/api/cmd/api/production_security.go` fails closed
when `GHANAGEO_ENV=production` and transactional mail is unconfigured, because
the developer console sends real email. Do not work around it by lowering
`GHANAGEO_ENV`: that same switch also relaxes HTTPS enforcement and passkey
origin validation.

To resolve it:

1. Create a Resend account and verify `digitalghana.dev` as a sending domain.
   Note that the domain currently publishes **no MX records at all**; sending
   needs the SPF and DKIM records Resend issues, which is separate from being
   able to receive mail.
2. Set on `ghanageo-api`:
   - `RESEND_API_KEY` — the Resend API key.
   - `RESEND_FROM_EMAIL` — a sender on the verified domain.
   `RESEND_API_URL` already defaults to `https://api.resend.com/emails`, which is
   the only endpoint the validator accepts.
3. Redeploy. The remaining production identity checks
   (`API_PASSKEY_RPID`, `API_PASSKEY_ORIGINS`) are already satisfied.

## Blocker 2 — worker entrypoint

The image's `ENTRYPOINT` is `/ghanageo-api`, and `render.yaml` overrides it for
the worker with `dockerCommand: /ghanageo-worker`. The Render CLI cannot set
`dockerCommand` for a Docker-runtime service — `--start-command` is rejected as
"only supported for native runtimes" — so the worker created by CLI currently
runs the API binary and fails on an API-only check:

    {"level":"ERROR","msg":"fatal","err":"API_PASSKEY_RPID must be a production registrable domain"}

Fix it either way:

- **Dashboard:** set the worker's Docker Command to `/ghanageo-worker`, or
- **Blueprint:** deploy `render.yaml` as a Blueprint instance, which sets
  `dockerCommand` and wires `REDIS_URL` from the Key Value automatically.

## Known deviations from `render.yaml`

The CLI cannot express everything the blueprint does. These were accepted to get
the services created, and should be reconciled before the API is called stable:

- **No persistent disk.** `render.yaml` mounts a 10GB disk at `/var/lib/ghanageo`
  for dataset exports; the CLI has no disk flag, so `API_EXPORT_DIR` is
  `/tmp/ghanageo/exports` and exports do not survive a restart.
- **Typesense stores its index in `/tmp`.** The image creates neither
  `/data` nor a nested path, and an image-runtime service cannot run `mkdir`, so
  `/tmp` is the only directory guaranteed to exist. The index is therefore
  rebuilt on restart rather than persisted. Give it a disk before relying on it.
- **Typesense is not in `render.yaml`.** It was added as a self-hosted private
  service instead of a managed Typesense Cloud cluster. Add it to the blueprint
  so the topology stays reproducible.
- **Auto-deploy is off** on both services, so a push does not redeploy them yet.
- Observability and alerting (`OTEL_EXPORTER_OTLP_*`, `SENTRY_DSN`,
  `SECURITY_ALERT_WEBHOOK_*`) and `API_TRUSTED_PROXY_CIDRS` are unset.

## DNS

`api-geo.digitalghana.dev` is **not** pointed at Render. Leave it that way until
the API answers `/health` on `https://ghanageo-api.onrender.com`; pointing the
canonical hostname at a failing service is worse than leaving it unresolved.
When ready, add the custom domain on the Render service and set the CNAME in
Vercel DNS, matching how `api-gov` and `api-calendar` are already wired.

## Verifying when unblocked

```sh
curl -s https://ghanageo-api.onrender.com/health
```

Then run the preflight, which enumerates every remaining production value:

```sh
ruby scripts/production-preflight.rb \
  --env ghanageo-api=.env.production \
  --env ghanageo-worker=.env.production
```
