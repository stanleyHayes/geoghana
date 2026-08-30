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
- Fresh registry lookups return HTTP 404 for all seven release artifacts:
  `@ghanageo/core`, `@ghanageo/client`, `@ghanageo/react`,
  `@ghanageo/node`, `@ghanageo/data`, `@ghanageo/proto` and the umbrella
  `ghanageo` package. No publish attempt was made.
- The scoped SDK and CLI workflows are credential-ready and support dry-run
  artifacts without reading the token.
- The current hosted workflows cannot start because GitHub reports failed
  account payments or an insufficient spending limit. npm authorization and
  hosted-runner billing are separate owner actions.

Resolution owner action: configure an npm trusted publisher or an `NPM_TOKEN`
with publish access to `@ghanageo`, rehearse the manual workflows, then push the
signed release tag.

## MongoDB Atlas restoration

- The earlier empty-database observation in this document has been superseded:
  production was subsequently deployed and seeded.
- A production archive was restored into an isolated scratch database on the
  same Atlas cluster. All 18 collection counts matched: **87,798 documents,
  0 failed**, with a measured full-dataset **RTO of approximately 8 minutes**.
  Indexes were restored, the scratch database was removed, and production was
  re-verified intact. Full evidence is in
  [`restore-drill-production-2026-08-29.md`](restore-drill-production-2026-08-29.md).
- Production remains on Atlas **M0**, which does not provide continuous backup
  or point-in-time restore. Until the cluster is upgraded, RPO equals the age
  of the latest verified manual dump. The completed archive drill therefore
  proves restoration mechanics and measured RTO, but not PITR.

Resolution owner action: upgrade Atlas to a PITR-capable tier, enable continuous
backup, wait for a recoverable point, restore a selected timestamp to an
isolated Atlas target, compare counts and validators, record measured RPO and
RTO, and remove the scratch target.

## Donations

- Neither local nor production configuration defines a mobile-money/card
  merchant integration.
- The public support action currently scrolls to the `#give` section. Both
  mobile-money and card rails are explicitly labelled "not yet connected";
  there is no approved checkout target, merchant callback or donation webhook.
- The reusable support component already accepts a donation URL, while donation
  state remains isolated from rate limits. Merchant provisioning is external;
  binding and verifying the approved checkout target is the final bounded
  application task after that provider decision.

Resolution owner action: supply the selected merchant/provider account and
approved public checkout target before changing the public action from its
honest pre-launch state.

## Fresh blocked-gate audit — 2026-08-29

This read-only follow-up distinguishes code-complete seams from provider-side
launch state. No secret values were printed and no external system was mutated.

- **npm:** all seven package lookups return 404 and `npm whoami` returns
  `ENEEDAUTH`. Packaging, provenance and secret-shape checks exist; registry
  authorization and publication remain external release actions.
- **Donations:** the free-forever and donor-neutral access rules are public, but
  both payment rails remain disconnected. A merchant account and approved
  checkout model must be supplied before the final URL/provider wiring and
  successful/cancelled/failed journey verification.
- **Backups:** the 87,798-document archive restoration succeeded with zero
  failed documents and an approximately 8-minute RTO. Atlas M0 still cannot
  provide continuous backup or PITR, so that acceptance criterion remains a
  cluster-tier gate.
- **Alerting:** the API and worker expose metrics and OTLP configuration seams;
  the API also supports a signed security-alert webhook. SLOs and burn-rate
  thresholds are documented in [`../../slo.md`](../../slo.md). No production
  collector, metrics scraper, paging receiver or provider-specific alert rules
  are configured in the locally available environment. Provisioning those
  services is external; binding the selected provider and retaining evidence
  from a synthetic page are the final wiring and verification steps.

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
