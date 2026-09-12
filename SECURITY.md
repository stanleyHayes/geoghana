# Security policy

## Reporting a vulnerability

**Do not open a public issue, pull request or discussion containing exploit details, credentials, personal data or an active vulnerability.**

Email **the GitHub private advisory channel above** with:

- the product name (GhanaGeo) and the affected surface — a hostname, an endpoint, an SDK and version, or a file path;
- the version, commit or deployment identifier you tested against;
- reproduction steps precise enough for a maintainer to follow;
- the impact you believe it has, and any preconditions an attacker would need.

Do not include live credentials, real API keys, or sensitive personal data in the report. If a proof of concept needs a secret, say so and we will arrange it rather than having it sent in plaintext.

## Response expectations

| Stage | Target |
|---|---|
| Acknowledgement of your report | Within two working days |
| Severity assessment shared with you | Within five working days |
| Coordinated disclosure | After a fix is available |

Critical findings that are reachable in production code are triaged immediately and block release. High findings are remediated within seven days, medium within 30 days, and low findings are reviewed during the monthly dependency update. An exception must name an owner, a compensating control and an expiry date, recorded in the security review — it is never granted silently.

We will keep you informed while a fix is being prepared, and we are glad to credit reporters who want it.

## Scope

GhanaGeo currently has four deployed frontends and **no deployed public API**. That shapes what is testable.

### In scope

- The deployed Next.js surfaces: `geo.digitalghana.dev`, `sandbox-geo.digitalghana.dev`, `console-geo.digitalghana.dev`, `admin-geo.digitalghana.dev`.
- The source in this repository: `services/api`, `services/worker`, `apps/`, `packages/`, `sdks/`, `cli/`.
- The published contracts and anything that lets a caller bypass them — authentication, API-key scope enforcement, fair-use quotas, GraphQL depth and complexity limits, gRPC message limits and deadlines, CORS allow-listing, CSRF and origin checks on cookie-authenticated mutations.
- Session handling, MFA, passkey registration and login, session rotation, replay revocation, and the difference between single and global logout.
- Privileged administrative operations, their RBAC checks and their audit trail — including whether a refused action is still audited.
- Secret handling: anything that puts a credential into a log, a client bundle, an error message, a commit or a telemetry payload.
- Data-integrity issues that let unverified or unlicensed data reach a published dataset — in particular anything that could introduce GhanaPostGPS digital-address data, or promote a `SEED_NEEDS_CANONICAL_RECONCILIATION` row to canonical by an automated path.

### Out of scope

- `api-geo.digitalghana.dev` and `grpc-geo.digitalghana.dev` as deployed services. They are not provisioned; a 404 there is the documented state, not a finding. Report API vulnerabilities against the source or a local run instead.
- Findings that only reproduce against a local development stack's intentionally weak defaults — the Compose Typesense key is literally `ghanageo_local_dev_only`, and `scripts/gen-env.sh` generates development secrets that are never used in production.
- Missing hardening headers or configuration on a third-party provider's own surfaces (Vercel, Render, MongoDB Atlas) that we do not control. Report those to the provider.
- Volumetric denial of service, automated scanner output without a demonstrated impact, and best-practice reports with no exploit path.
- Social engineering of maintainers or contributors, and physical attacks.
- The absence of a feature that is on the roadmap and documented as not yet built.

Testing must stay within the surfaces above. Do not attempt to access another person's account or data, do not degrade service for others, and do not run destructive tests against production.

## Continuous controls

These run on every pull request and every push to `main` ([`.github/workflows/security.yml`](.github/workflows/security.yml), [`.github/workflows/quality.yml`](.github/workflows/quality.yml)):

- Production dependency audits for reachable Go symbols and for JavaScript packages. High or critical findings fail the build.
- CodeQL analysis of Go and JavaScript/TypeScript on pull requests, on `main`, and on a weekly schedule.
- Secret scanning on every commit.
- A licensed-data check that rejects any GhanaPostGPS payload from canonical data surfaces (`make check-licensed-data`).
- Patch-level toolchain pinning in the Go modules and the workspace, so a standard-library fix cannot be silently lost on an older runner.

By construction: configuration comes from the environment, never from source; authorization values and API-key secrets are prohibited from application and incident logs, where only the non-secret key prefix may appear; browser keys are origin-restricted and scope-limited; and the API connects to MongoDB as a least-privilege user scoped to its own database, with all filters built from typed structs rather than raw maps assembled from request data.

The full policy, including the application-security acceptance criteria the local suite must prove rather than infer, is [`docs/security.md`](docs/security.md).

## Production release security gates

A production release is gated on all of the following. Passing unit tests is not one of them.

1. `make test`, `make lint`, `govulncheck ./...` from `services/api`, and `pnpm audit --prod` from the repository root all clean.
2. The hosted CodeQL result retained with the release evidence, and the final critical/high triage decision recorded by the release reviewer. Local success alone does not close that external review gate.
3. `make check-licensed-data` passing, including its positive regression fixtures.
4. The fail-closed Render preflight passing with real production environment files:
   `make production-preflight API_ENV=… WORKER_ENV=…`. It rejects blanks and placeholders without printing any value, and it is the gate — a valid `render.yaml` is not.
5. Least-privilege credentials, an exact CORS allow-list, rate limits in force, security headers, HTTPS required, and passkey relying-party origins pinned to the canonical hosts.
6. Immutable audit events for every privileged change, including refusals.
7. Alerting configured and delivering — rejected browser-key origins, exhausted quota buckets and failed privileged-account authentication are emitted as signed security alerts.
8. A tested restore path and a rehearsed rollback, with the evidence recorded under [`docs/runbooks/evidence/`](docs/runbooks/evidence).

Operating targets for the running service, including the error-budget burn-rate alerting, are in [`docs/slo.md`](docs/slo.md). Incident procedure is in [`docs/runbooks/operations.md`](docs/runbooks/operations.md).

## Supported versions

GhanaGeo has not made a public V1 release. Until it does, security fixes are applied to `main` only. When versioned releases begin, this section will name the supported release line.

## Licensing and data safety

Reports about data we are not licensed to hold are security reports, not documentation issues. GhanaPostGPS digital addresses are proprietary to Ghana Post and are never scraped, stored or redistributed; if you find such a payload anywhere in this repository, its fixtures, its migrations or a published dataset, report it privately through the address above. The licence position for every source is recorded in [`docs/licensing-register.md`](docs/licensing-register.md).

A dedicated `security@digitalghana.dev` mailbox is not configured yet — the domain currently has no mail routing — so GitHub private vulnerability reporting is the only monitored channel. This file will be updated if that changes.
