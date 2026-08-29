# External V1 launch gates — 2026-08-29

Checked from the release workstation against the configured production and
hosted services. No credential values are recorded here.

## GitHub Actions and CodeQL

- Repository: private `stanleyHayes/geoghana`.
- Latest quality run: `33272922624`, commit
  `1a4724b5161198d5dfa7f553fc29ae002d73fcd3`.
- Every job ended before acquiring a runner. GitHub check annotations report:
  “The job was not started because recent account payments have failed or your
  spending limit needs to be increased.”
- Actions are enabled and allow all actions; workflow permissions default to
  read. The failure is account billing, not repository workflow syntax.
- The Code Scanning Alerts API returns HTTP 403: code scanning is not enabled
  for this private repository. Consequently there is no hosted CodeQL result
  to retain or triage yet.

Resolution owner action: repair GitHub billing/spending, enable GitHub Code
Security/CodeQL for the private repository, then rerun `quality.yml` and
`security.yml` on the exact release commit. Retain both CodeQL analyses and
record the critical/high triage decision.

## npm registry

- `npm whoami` returns `ENEEDAUTH` on the release workstation.
- No publish attempt was made.
- The scoped SDK and CLI workflows are credential-ready and support dry-run
  artifacts without reading the token.

Resolution owner action: configure an npm trusted publisher or an `NPM_TOKEN`
with publish access to `@ghanageo`, rehearse the manual workflows, then push the
signed release tag.

## MongoDB Atlas restoration

- `.env.production` contains a non-local Atlas connection and the connection
  ping succeeds.
- The configured `ghanageo` production database currently has zero
  collections. There is therefore no production snapshot or application state
  from which to measure RPO/RTO.
- The local archive-to-isolated-database drill remains successful and is
  recorded separately in `restore-drill-2026-08-29.md`.

Resolution owner action: deploy and seed the production database, enable Atlas
continuous backup/PITR, wait for a recoverable point, restore it to an isolated
Atlas target, compare counts and validators, record elapsed RPO/RTO, and remove
the scratch target.

## Donations

- Neither local nor production configuration defines a mobile-money/card
  merchant integration.
- The public site correctly labels the rails as not connected; donation state
  remains isolated from rate limits.

Resolution owner action: supply the selected merchant/provider account and
approved public checkout target before changing the public action from its
honest pre-launch state.

## Public deployment and DNS

- DNS lookups return no A or CNAME records for `geo.digitalghana.dev`,
  `api.geo.digitalghana.dev`, `sandbox.geo.digitalghana.dev`,
  `console.geo.digitalghana.dev`, or `admin.geo.digitalghana.dev`.
- HTTPS probes consequently return no HTTP response for all five hosts.
- Local production builds and route/link checks pass, but they do not prove a
  public deployment.

Resolution owner action: provision the API and four frontend targets, populate
their production secrets, configure per-host TLS and DNS, then run the live
smoke/accessibility/performance checks against the deployed release.

The repository now includes a schema-validated Render Blueprint for the HTTP
API, a secret-excluding production Docker context and one image that supports
explicit `http` and `grpc` process modes. Both modes were exercised locally on
platform-assigned ports: HTTP health returned dataset `2026.08.3-ulid`, gRPC
health returned `SERVING`, and the gRPC-only process exposed no HTTP listener.
Render's current official documentation says native gRPC is not supported over
external connections, so `grpc.geo.digitalghana.dev` requires an HTTP/2-native
container host; it cannot honestly be declared as another Render web service.
