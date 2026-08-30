# Admin authentication and production takeover — 2026-08-30

## Release decision

**Approved locally; not approved for public launch.** The source and local
release gates have no known reproducible blocker. Real infrastructure values,
provider activation and public-host verification are still absent.

## Admin security boundary

- Every protected admin route is intercepted before render and validated
  against the API session endpoint. Missing, invalid and expired sessions are
  redirected safely; an unavailable auth service fails closed with HTTP 503.
- Ordinary developer roles are rejected. Admin admission requires
  `geography:view`; mutations independently require `geography:edit` in the
  transport and use case. Privileged roles require completed MFA.
- Session expiry, logout, recovery-code login, TOTP enrolment and role-aware
  navigation use the real session. The former fake role preview is removed.
- Redirect targets reject absolute, protocol-relative, backslash, encoded and
  double-encoded external forms.
- A per-request nonce CSP, HSTS, COOP, noindex and private/no-store headers ship
  on admin documents. Hashed Next assets retain immutable public caching.
- Authenticated unknown routes render the branded 404; known routes unavailable
  to the operator render access denial.

## Production hardening completed

- Production env generation no longer mixes development and production
  secrets. API and worker internal tokens agree; local-only rotation cannot
  overwrite pasted production values.
- Production mail uses the exact canonical Resend HTTPS endpoint and never logs
  verification/reset tokens. Unsafe portal/provider URLs fail startup.
- Password recovery is enumeration-safe, rate-limited, single-use and expiring.
  Reset, password replacement, session-epoch bump and session revocation are one
  transaction. Concurrent issuance, rotation and 16-way token replay each have
  real replica-set race evidence.
- Trusted forwarding data is accepted only from configured immediate-peer
  CIDRs, spoofable chains are stripped, HTTPS survives normalization, and
  production cookies remain Secure.
- Production passkey RP/origins fail closed and cannot use localhost.
- Deployment preflight is service-aware and fails without real environment
  inputs. It checks datastores, mail, proxy trust, passkeys, OTLP, signed alerts,
  Sentry and canonical frontend origins without printing values.
- CI now runs MongoDB as `rs0`, makes missing transaction support fatal, pins
  security tooling, and executes the security-critical concurrency tests.
- Public cross-app links and shared shell defaults are validated HTTPS origins;
  intentional authentication entry routes are explicitly modeled by the link
  gate. SDK contract locks were regenerated and verified.

## Verification

- Full API and worker tests and vet: pass.
- Focused race/replica-set authentication and reset suites: pass.
- Workspace lint, typecheck, tests and all production builds: pass.
- Admin runtime matrix: anonymous/invalid/expired denied; ordinary role denied;
  privileged role accepted; auth outage returns 503.
- Admin CSP nonce, Secure/HttpOnly cookies, private no-store documents and
  immutable hashed assets: pass.
- Production blueprint, service-aware preflight fixtures, env generator, route
  links, dependency audit, SDK contract locks and 13-report/109-pair retained
  conformance matrix: pass.
- Independent release verification, security audit and code-quality review:
  **approved locally**.

## What is not implemented

Authentication now protects the entire admin surface, but authentication does
not turn an operator brief into a live capability. The API currently exposes a
small implemented admin subset (permissions plus geography edit/deprecate
operations). Planned pages that describe future adapters, jobs, release stages,
organizations, limits, health and audit capabilities remain honest readiness
briefs until their own backend stories satisfy acceptance criteria.

## External launch blockers

- The real production env fails closed until Redis, Typesense, Resend, trusted
  proxy CIDRs, OTLP, signed security alerts, Sentry, passkey and canonical origin
  values are populated.
- Public GhanaGeo DNS/TLS hosts are unresolved and undeployed.
- npm registry ownership/credentials and V1 package publication are absent.
- Donation merchant checkout is not provisioned.
- Atlas M0 cannot provide required continuous backup/PITR; an M10+ decision is
  required.
- Production telemetry collection, paging and synthetic alerts are unproven.
- Hosted GitHub security/quality execution remains account/billing gated and
  CodeQL is unavailable on the current private-repository plan.
- Search Console and field Core Web Vitals require the live public host.
