# Production gate audit — 2026-08-30

Read-only checks were run from the release workstation. No credential values
were printed, no deployment was started, and no external service was mutated.

## Release decision

**Not ready for public traffic.** The repository contains the deployment,
observability, recovery and release seams, but the current release still has
local configuration/link defects and provider-side launch gates. Keep public
DNS disabled until every blocking row below is closed against the same release
commit.

## Local blockers

| Gate | Evidence | Required closure |
|---|---|---|
| Production secret separation | `scripts/gen-env.sh:239` writes the local `INTERNAL_SERVICE_TOKEN` to the production API while `:272` writes the production token to the worker. The four production Next.js files at `:366-370` also receive local auth/session/internal secrets. | Use the `PROD_*` secrets consistently, regenerate ignored env files, prove API/worker equality and production/development inequality without printing values. |
| Generated frontend production URLs | The generator only emits API, GraphQL and site values at `scripts/gen-env.sh:354-357`; the marketing SEO contract also requires web, sandbox, portal and indexability values documented at `docs/runbooks/deployment.md:63-80`. | Generate every required public URL and an explicit production-only indexing switch; validate before build. |
| Visible localhost links | Marketing, sandbox, portal and admin source still contains public navigation links hard-coded to `localhost` (`apps/web/src/components/site-chrome.tsx`, `apps/web/src/app/about/page.tsx`, `apps/sandbox/src/app/page.tsx`, `apps/portal/src/app/page.tsx`, `apps/admin/src/app/page.tsx`). | Resolve public origins from validated environment configuration and retain localhost only as a development fallback. Rebuild and crawl all four surfaces. |
| Deterministic deployment preflight | The runbook lists the manual go-live proof, but no single command currently rejects mismatched secrets, placeholders, missing required variables, localhost production links, unresolved hosts or an indexable preview before deploy. | Add a secret-safe preflight and make it a required release check. |

The production env files are ignored by Git and contain values for their
existing keys, but presence is not proof that the values are correct. Because
the generator currently mixes development and production secrets, the files
must be regenerated after that defect is fixed.

## External blockers

| Gate | Current evidence | Owner action |
|---|---|---|
| Public deployment, TLS and DNS | `dig` returned no A or CNAME for `geo`, `api.geo`, `grpc.geo`, `sandbox.geo`, `console.geo` or `admin.geo.digitalghana.dev`; every HTTPS probe returned no response. | Provision Render/API, HTTP/2-native gRPC and four Vercel projects; bind TLS and DNS, then execute the live suite in `docs/runbooks/deployment.md:88-93`. |
| npm publication | `npm whoami` returned `ENEEDAUTH`; all seven V1 package `latest` endpoints returned HTTP 404. | Establish `@ghanageo` ownership and trusted publishing or a scoped token, repair hosted CI, then publish the signed release and verify clean-consumer installs. |
| Donation checkout | `/support` honestly marks mobile-money and card rails “not yet connected”; no merchant checkout/callback configuration exists. | Select and approve a merchant provider/account and checkout target, then wire and verify success, cancellation, failure and webhook handling. |
| Atlas continuous backup/PITR | The production archive restore already proves 87,798 documents with zero failures and about eight-minute RTO. The current M0 tier does not satisfy continuous backup/PITR. | Upgrade to a PITR-capable Atlas tier, enable continuous backup, restore a timestamp to an isolated target and retain RPO/RTO evidence. |
| Production telemetry and paging | Render declares OTLP and signed alert webhook secrets, and the runbooks define signals and burn rates. No collector, scraper, paging receiver or synthetic page is proven active. | Provision the selected providers, scrape API/worker metrics, ingest traces, deliver a signed security alert and a synthetic burn-rate page, then retain acknowledgements. |
| Hosted security gates | The latest Security and Quality runs (`33279741644`, `33279741667`) failed before any job steps. The Code Scanning API still returns 403 because scanning is not enabled. | Repair GitHub billing/spending, enable CodeQL for the private repository, rerun both workflows on the release commit and triage critical/high results. |
| Search launch services | DNS is absent, so Search Console ownership, sitemap submission and field Core Web Vitals cannot be established. | After the canonical host passes the production SEO audit, verify Search Console, submit the sitemap and monitor field data. |

## Locally verified, not blocking by itself

- `render.yaml` keeps provider credentials dashboard-supplied and defines
  health checking, persistent exports, OTLP and signed security-alert seams.
- The deployment runbook separates Render HTTP from native gRPC hosting and
  prevents publishing the gRPC record before public TLS protocol checks pass.
- The production archive restoration drill is complete; only the stronger PITR
  acceptance criterion remains.
- `pnpm audit --prod --audit-level high` reported no known vulnerabilities on
  2026-08-30. This does not replace the blocked hosted CodeQL/container checks.
- All seven package artifacts are locally buildable according to the V1 ledger;
  only actual registry ownership/publication remains for that checklist row.

## Final public-traffic gate

After local defects and provider activation are complete, use one immutable
release commit and record: deployment IDs, image digest, dataset version, DNS
answers, TLS certificate, frontend crawl results, admin anonymous/role/session
denials, REST/GraphQL/gRPC health and rate-limit behavior, p95 results, backup
recovery point, alert acknowledgements, package versions and donation checkout
receipts. A partially passing collection from different commits is not launch
evidence.
