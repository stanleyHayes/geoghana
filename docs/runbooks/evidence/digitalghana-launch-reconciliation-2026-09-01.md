# Digital Ghana launch reconciliation — 2026-09-01

## Scope

`GEO-30.1` reconciles GhanaGeo's actual provider state with the approved Digital Ghana portfolio architecture. It does not treat wildcard DNS, a checked-in Blueprint or a created provider project as proof of a live application.

## Authenticated inventory

- GitHub repository: private `stanleyHayes/geoghana`, clean `main` parity before this branch.
- Vercel team: `hayfordstanleys-projects`.
- Four independently deployable Vercel projects created: `ghanageo-web`, `ghanageo-sandbox`, `ghanageo-portal`, `ghanageo-admin`.
- Render workspace: `tea-cspvc3ggph6c739fskn0`; no GhanaGeo services existed at inventory time.
- Vercel DNS resolves the apex, planned hostnames and arbitrary wildcard probes, but representative TLS handshakes did not complete before deployment.

## Canonical hostname decision

Active configuration, contracts, SDK defaults and tests now use first-level operational hostnames:

- `geo.digitalghana.dev`
- `api-geo.digitalghana.dev`
- `grpc-geo.digitalghana.dev`
- `sandbox-geo.digitalghana.dev`
- `console-geo.digitalghana.dev`
- `admin-geo.digitalghana.dev`

Dated historical evidence remains unchanged. Compatibility redirects for any previously advertised nested names can be added only after the canonical surfaces are healthy.

## Deployment configuration

Each Next.js surface deploys from the monorepo root with a dedicated configuration in `infra/vercel/`. Root-level Next.js framework detection is pinned to the same `16.3.3` version used by the applications. `.vercelignore` excludes unrelated service, SDK, cache and evidence trees from frontend uploads while retaining workspace packages.

## Verification completed

- Render Blueprint validation: passed; 2 services and 1 Key Value action recognized.
- Production preflight regression tests: passed.
- Blueprint-only production preflight: passed.
- Generated environment regression checks: passed with secrets withheld.
- Workspace typecheck: 17/17 tasks passed.
- Workspace tests: 13/13 tasks passed, including 39 client tests and cross-protocol conformance fixtures.
- Internal route/link audit: web 11/11, sandbox 1 route, portal 4 routes, admin 45 routes; all links resolve.
- Vercel configuration JSON parse checks: passed.

## Provider gate discovered

The first `ghanageo-web` deployment was blocked before build because the inherited Git commit author email was not associated with the authenticated Vercel team. Repository-local author identity was changed to the GitHub/Vercel account identity for the new `GEO-30.1` commit. The blocked deployment is retained as evidence and is not a production release.

## Remaining gates

- Retry and verify all four Vercel deployments from the corrected commit, then attach hostnames and run TLS/application smoke checks.
- Supply 14 missing API/worker provider values identified by the real production preflight.
- Provision Render API, worker and Key Value only when the required database/search/mail/telemetry/security configuration is available.
- Repair hosted GitHub checks, complete alert delivery, backup/PITR and rollback evidence before stable launch.
