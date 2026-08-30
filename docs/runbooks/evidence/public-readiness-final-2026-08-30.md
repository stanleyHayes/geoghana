# Public-readiness final audit — 2026-08-30

## Code outcome

The scoped V1 application is code-ready. The public launch is not complete
until the external gates below are supplied and exercised against the exact
release commit.

The admin no longer presents readiness briefs as product functionality. Its
navigation exposes 45 real routes plus two implicit authentication entries;
the route/link audit has no dead or orphan destinations. Dashboard metrics,
operations, durable imports, moderation, release lifecycle and notes,
developer support, fair-use policy, health probes, geography, geometry,
aliases, redirects and audit export all read or mutate protected server
contracts. Missing dependency telemetry is rendered as unavailable, never as
synthetic success.

The deliberate V1 scope boundaries are recorded in `agent_plan.md`: roads and
POIs support provenance-rich reads plus create/deprecate but not full PATCH
editing; geometry uses a structured Polygon/MultiPolygon editor with ETag CAS
rather than a visual vertex toolbar; import detail retains a source reference
and SHA-256 digest rather than copying potentially licensed or personal raw
provider payloads into the operator database.

## Security closure

Independent review found no remaining critical or high code finding after:

- database-enforced single-successor audit hash chains across replicas;
- database-enforced single published dataset version;
- bounded WebAuthn payloads;
- Redis-independent, fail-closed Argon2 concurrency and bounded IP-prefix
  throttling;
- private/no-store, cookie-varying authenticated responses;
- symlink-resolved artifact containment;
- MFA/RBAC, secret-free DTOs, reason-required mutations, idempotency, ETag/CAS
  and transactional state-plus-audit commits.

## Reproducible local gates

The final worktree passed:

- `go test ./...` and `go vet ./...` in `services/api`;
- workspace `pnpm typecheck`, `pnpm lint`, `pnpm test` and `pnpm build`;
- `pnpm check:links`;
- `make production-blueprint-check` and `make check-licensed-data`;
- Redocly validation and REST/OpenAPI route parity;
- SDK contract regeneration/lock verification;
- `git diff --check`.

The real production preflight remains intentionally fail-closed: the checked
API and worker production files report 28 missing, blank or placeholder
provider values (Redis, Typesense, Resend, trusted proxies, OTLP, security
webhook, Sentry and fixed passkey/HTTPS/service settings). Blueprint-only and
preflight regression tests pass, but that is not a deployment claim.

## External launch gates

1. Provision the API, web, sandbox, portal and admin services; populate all
   production secret/config values; configure DNS and TLS for the documented
   hosts. Native public gRPC requires an HTTP/2-capable host rather than the
   current Render web-service plan.
2. Repair GitHub billing, enable hosted Actions/CodeQL for the private
   repository, and retain clean results for the exact release commit.
3. Configure npm trusted publishing or an authorized token and publish the
   signed SDK release; the packages are not currently present in the registry.
4. Upgrade Atlas from M0 to a PITR-capable tier, enable continuous backup, and
   retain a timestamp restore drill. The existing 87,798-document archive
   restore proves mechanics/RTO, not PITR.
5. Bind a production metrics/trace collector, alert rules and paging receiver;
   retain a synthetic page and recovery event.
6. Supply an approved merchant checkout before enabling donations. Donation
   state must remain isolated from fair-use limits.
7. After deployment, verify public smoke, accessibility, performance, Search
   Console ownership and field Core Web Vitals on the resolving HTTPS hosts.

No provider mutation or secret fabrication was performed during this audit.
