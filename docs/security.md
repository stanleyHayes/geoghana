# Security and patching policy

Report vulnerabilities privately through GitHub by opening a draft advisory at <https://github.com/stanleyHayes/geoghana/security/advisories/new>. Do not open a
public issue containing credentials, exploit details or personal data. We aim
to acknowledge reports within two working days, provide a severity assessment
within five, and coordinate disclosure after a fix is available.

## Continuous controls

- Every pull request and `main` build runs production dependency audits for Go
  reachable symbols and JavaScript packages. High or critical findings fail.
- CodeQL scans Go and JavaScript/TypeScript on pull requests, `main`, and a
  weekly schedule.
- Go modules and the workspace pin a patch-level toolchain so standard-library
  fixes cannot be silently lost on an older runner.
- CI rejects unlicensed GhanaPostGPS payloads from canonical data surfaces.
- Authorization values and API-key secrets are prohibited from application and
  incident logs; only the non-secret key prefix may be recorded.
- The API emits structured security alerts for rejected browser-key origins,
  exhausted quota buckets and failed privileged-account authentication. In
  production, `SECURITY_ALERT_WEBHOOK_URL` points to the operator's incident
  receiver and `SECURITY_ALERT_WEBHOOK_SECRET` signs the exact JSON body with
  HMAC-SHA256 in `X-GhanaGeo-Signature`. Alert delivery failures never take down
  authentication or public reads, but are emitted as error-level log events.

## Patching cadence

Critical reachable findings are triaged immediately and block release. High
findings are remediated within seven days; medium findings within 30 days; low
findings are reviewed during the monthly dependency update. An exception must
name an owner, compensating control and expiry date in the security review.

Before release, operators retain the scanner outputs with the release evidence
and confirm there are no unresolved reachable critical or high findings.

## V1 application-security acceptance

The local test suite must prove the §22.3 abuse boundaries, rather than infer
them from configuration:

- expired and revoked keys, missing scopes and exhausted quota fail before the
  protected handler;
- GraphQL depth and weighted complexity attacks fail before resolver work;
- gRPC enforces its 1 MiB receive limit and server-side deadlines;
- CORS uses an exact allow-list, and a cookie-authenticated unsafe request from
  another origin is rejected before routing;
- sign-in cannot promote a planted session, tokens rotate, replay revokes all
  sessions, single and global logout differ, and MFA recovery is single-use;
- unknown roles and permissions deny by default, and refused admin mutations
  enter the audit chain.

Run `make test`, `make lint`, `govulncheck ./...` from `services/api`, and
`pnpm audit --prod` from the repository root. The release reviewer then retains
the hosted CodeQL result and records the final critical/high triage decision;
local success alone does not close that external review gate.
