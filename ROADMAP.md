# GhanaGeo roadmap

This roadmap is **directional, not a commitment**. It contains no dates, because the items that remain are mostly gated on external state — provider accounts, registry credentials, a cluster tier, a legal agreement — and a date on any of those would be fiction. The machine-readable state of every story lives in the task board at §1a of [`agent_plan.md`](agent_plan.md), which is the source of truth; this file is the readable summary.

Current lifecycle: **externally blocked**. V1 is implemented and locally verified, four production frontends are live, and the public API is not deployed.

---

## Now — shipped and verified

Everything below is marked done on the task board with recorded evidence, except where a line names a partially closed item — GEO-30.1 is in progress and only its frontend boundary has shipped, and the application-security suite is tracked as Spec §22.3 story GEO-23.3, which carries no board row of its own.

**Contracts and foundation**

- [x] Monorepo, pnpm workspace, Turborepo, Go workspace, dedicated local port block (GEO-1.1–1.3)
- [x] Three published contracts from one source of truth: [`contracts/openapi/v1.yaml`](contracts/openapi/v1.yaml), [`contracts/graphql/schema.graphql`](contracts/graphql/schema.graphql), [`proto/ghanageo/v1/geography.proto`](proto/ghanageo/v1/geography.proto), plus a generated error catalogue (GEO-2.1–2.5)

**Data**

- [x] Seed import of 16 regions, 261 districts and 16 places, proven idempotent (GEO-3.1–3.3, GEO-4.1)
- [x] Deterministic ULIDs, strict reference validators, compatibility tombstones and redirects, published as `2026.08.3-ulid` (GEO-3.1)
- [x] **Complete district boundary coverage — 261 of 261 districts and all 16 regions**, using geoBoundaries first and OpenStreetMap only for what it could not supply. See [`docs/boundary-coverage.md`](docs/boundary-coverage.md) (GEO-4.6, GEO-4.7)
- [x] OpenStreetMap ingestion of roads, points of interest and settlements, with ODbL attribution carried on the records and in every export file (GEO-4.7)
- [x] Dataset release workflow: `draft → validation → review → approved → published`, with rollback, checksums and a changelog, decoupled from code deployment (GEO-8.3)
- [x] Transactional outbox worker with leases, bounded retry, dead-lettering and idempotent search rebuild (GEO-4.WORKER)

**API**

- [x] REST, GraphQL and native gRPC over one shared application layer, behind one authentication and rate-limit layer (GEO-8.x, GEO-10.x, GEO-11.x)
- [x] Identity, organizations, applications, API keys, rotation and revocation, scopes, fair-use quotas, immutable audit logging and signed security alerting (GEO-9.x, GEO-16.x)
- [x] Passkeys, TOTP MFA, session rotation and replay revocation (GEO-9.2, GEO-17.MFA-ENROLMENT)
- [x] Search, autocomplete, geocode, reverse and nearby meeting every functional and performance target — worst measured p95 72.5 ms, every hot query confirmed `IXSCAN` with no collection scan (GEO-12.6)

**Interfaces**

- [x] The tri-morphic design system and its Design-QA gates, passing the full 180-point production-build matrix (GEO-14.x, [`design-qa.md`](design-qa.md))
- [x] Marketing site and documentation with quick starts for curl, CLI, React, GraphQL and gRPC, verified against the running site (GEO-18.x)
- [x] Anonymous sandbox demonstrating all three protocols safely, developer console and admin/steward portal (GEO-15.x, GEO-16.x, GEO-17.x)
- [x] Four Next.js surfaces deployed and live on canonical hosts behind a valid certificate (GEO-30.1, frontend boundary only)

**Distribution**

- [x] Seven TypeScript packages built, tested and packed with generated-artifact drift checks (GEO-13.1–13.7)
- [x] CLI distribution and release automation: tag workflow, seven-platform npm bundle, checksums, Homebrew formula (GEO-13.9)
- [x] The V2.0 SDK family implemented and locally verified — Python, Go, Dart plus Flutter, Java plus Spring Boot starter, .NET, PHP plus Laravel — held to [`docs/sdk-charter.md`](docs/sdk-charter.md) by a language-neutral conformance suite. The retained 13-report matrix proves 109 case/protocol evidence pairs with zero failures or skips (EP-23)

**Security and operations**

- [x] Security review with no unresolved critical or high findings (GEO-21.REVIEW, [`docs/runbooks/evidence/security-review-2026-08-29.md`](docs/runbooks/evidence/security-review-2026-08-29.md))
- [x] CI rejection of any GhanaPostGPS digital-address payload, with positive regression fixtures (GEO-21.7)
- [x] Production restoration drill executed and measured: 87,798 documents restored, 0 failed, RTO approximately 8 minutes (GEO-22.2)
- [x] Application-security acceptance suite proving the abuse boundaries rather than inferring them from configuration (GEO-23.3, Spec §22.3, [`docs/security.md`](docs/security.md))

---

## Next — the open gates

These are the only things between the current state and a public V1. Every one is blocked on something outside this repository.

| Gate | Blocking dependency | Board item |
|---|---|---|
| Deploy the public API at `api-geo.digitalghana.dev` | 14 API/worker provider values still reported missing by the real production preflight; production Redis and Typesense (or Atlas Search) not provisioned | GEO-30.1 |
| Native gRPC at `grpc-geo.digitalghana.dev` | Render does not serve native gRPC externally; needs an HTTP/2-native container host, then `grpcurl` proof of health, reflection, anonymous reads, scope enforcement, `429` behaviour and the resumable change stream before DNS is added | [`docs/runbooks/deployment.md`](docs/runbooks/deployment.md) |
| Close the CORS, passkey, data-path, alert, backup and rollback gates | All depend on the API being deployed first | GEO-30.1 |
| Continuous backup and point-in-time restore | An Atlas M10 cluster tier — M0 does not offer PITR. The restore mechanics themselves are already proven | GEO-22.1 |
| Production alert delivery | `SECURITY_ALERT_WEBHOOK_URL` and `SECURITY_ALERT_WEBHOOK_SECRET` pointing at a real incident receiver | GEO-22.1 |
| Publish the seven V1 npm packages | npm release credentials. The packages build, test, pack and pass drift checks; publication is an external launch action | V1 Definition of Done |
| Publish the V2.0 SDKs to their registries | Registry ownership and credentials, protected-environment approvals, the V1 public-launch gate, and REST/GraphQL/gRPC contracts frozen for one release cycle | EP-23 |
| A working donate action | Merchant accounts for the mobile-money and card rails. The free-forever commitment is already public; the rails are labelled "not yet connected" because they are | V1 Definition of Done (§24) |
| Green hosted GitHub checks | GitHub account billing. This is an account state, not a code finding | GEO-21.REVIEW |

---

## Later — deferred scope

Post-V1 work, shaped in §23 of [`agent_plan.md`](agent_plan.md). Each wave re-enters the lifecycle at discovery and produces its own requirements and approval gate before any code is written.

| Wave | Theme | Gate to start |
|---|---|---|
| V2.0 Distribution | The full multi-language SDK family | V1 launched; contracts frozen for one release cycle |
| V2.1 Enterprise | OAuth2 client credentials, SSO/SAML and OIDC with SCIM, service accounts, private datasets, per-contract SLAs, funding sustainability | At least one signed enterprise or government pilot |
| V2.2 Data depth | Business/POI registry with freshness metadata, full road-network expansion, region-segmented offline datasets, vector tiles, address validation, postal metadata | Canonical geography stable; steward programme staffed |
| V2.3 Ecosystem | Webhooks, a cursor-resumable change feed over HTTP, contributor reputation, a regional data-steward programme, a public contribution portal | Community contribution volume justifies moderation tooling |
| V2.X (gated) | A licensed GhanaPostGPS adapter | **A signed licence or written integration agreement. A legal gate, not an engineering one. No target date, and no story in it may start early — not a spike, not a prototype** |

Also deferred, and named so they do not drift back in quietly: a managed queue or pub-sub in place of the transactional outbox, a separate vector-tile service, and the canonical reconciliation of official GSS/GNHR district codes — seed rows deliberately carry a blank `official_code` and the status `SEED_NEEDS_CANONICAL_RECONCILIATION` until an official, licence-recorded source artefact exists. Vector tiles additionally revisit the V1 Leaflet decision and require their own architecture decision record.

---

## Explicitly out of scope

Naming these prevents scope drift as much as naming what is in scope. From §23.7 and §2 of [`agent_plan.md`](agent_plan.md).

- **A general-purpose worldwide geocoder.** GhanaGeo stays Ghana-specific.
- **Turn-by-turn routing or a navigation engine.**
- **Real-time traffic.**
- **Property ownership or personal household data.** PII is stripped at ingestion, before the raw landing write.
- **Emergency-service dispatch.**
- **Redistribution of any GhanaPostGPS digital address**, licensed or not. A licence would permit lookup, never redistribution, and the CI exclusion test stays enabled permanently — including after a licence is signed.
- **A paid access tier.** GhanaGeo is free and stays free. No payment provider, no plan upgrade, no card on file — rule F1 of §24 in `agent_plan.md` forbids any code path that charges for access. Fair-use limits are identical for everyone and a limit may be raised on documented need, never on payment. Sponsorship buys recognition, never capability.

---

## How this file stays honest

A lifecycle word is one of: proposed, building, beta, stable, externally blocked, retired or deferred. A status transition updates the task board in [`agent_plan.md`](agent_plan.md), this file, and the dated evidence path in [`docs/runbooks/evidence/`](docs/runbooks/evidence) — in the same reviewed change. A passing build is never evidence of a deployment, and a checked-in blueprint is never evidence of a provisioned service.
