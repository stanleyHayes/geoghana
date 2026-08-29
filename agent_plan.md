# GhanaGeo — Agent Execution Plan

**Project key:** `GEO`
**Version:** 1.0
**Last updated:** 2026-08-29
**Governed by:** `GhanaGeo_End_to_End_Build_Specification_v2.docx` (product/technical source of truth), `AI_Development_Workflow_Training_Manual.docx`, `AI_Native_Software_Engineering_Operations_Manual.docx`
**Audience:** AI coding agents and engineers building GhanaGeo in parallel
**Status:** Sprint 0 complete. Data core, REST v1 and the design system run and are verified. Contracts (EP-02) are the next critical path.

> This is the single source of truth for **how the work is decomposed and sequenced**.
> The Build Specification says **what and why** to build. This plan says **who builds it, in what order, against which contracts, and how to avoid stepping on each other.**
>
> **Companion doc:** [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md) — the mandatory UI/UX + motion specification (tri-morphic theming, admin shell, sidebar, navbar, dropdowns, transitions). Every frontend story in lanes **L7–L11** builds to it. A frontend PR that does not satisfy `DESIGN_SYSTEM.md` is not Done.
>
> **If this plan conflicts with the Build Specification, stop and reconcile the conflict before writing code.**

---

## Table of Contents

1. [How to Use This Plan](#1-how-to-use-this-plan)
2. [Agent Task Board (live)](#1a-agent-task-board-live)
3. [Non-Negotiable Engineering Rules](#2-non-negotiable-engineering-rules)
4. [Technology Decisions](#3-technology-decisions)
5. [Monorepo Folder Structure](#4-monorepo-folder-structure)
6. [Service Internal Structure (Hexagonal)](#5-service-internal-structure-hexagonal)
7. [Contracts-First: The Key to Parallelism](#6-contracts-first-the-key-to-parallelism)
8. [Cross-Cutting Platform Concerns](#7-cross-cutting-platform-concerns)
9. [Parallelization Model — Lanes, Ownership & Collisions](#8-parallelization-model--lanes-ownership--collisions)
10. [Ticket Conventions](#9-ticket-conventions)
11. [Global Definition of Done](#10-global-definition-of-done)
12. [Dependency Graph](#11-dependency-graph)
13. [Sprint Plan Overview](#12-sprint-plan-overview)
14. [Epic Catalog](#13-epic-catalog)
15. [Sprint 0 — Foundation & Contracts](#14-sprint-0--foundation--contracts)
16. [Sprint 1 — Data Core](#15-sprint-1--data-core)
17. [Sprint 2 — API Core](#16-sprint-2--api-core)
18. [Sprint 3 — Transports & Identity](#17-sprint-3--transports--identity)
19. [Sprint 4 — Search, SDKs & Design System](#18-sprint-4--search-sdks--design-system)
20. [Sprint 5 — Portals (Developer, Sandbox, Admin)](#19-sprint-5--portals-developer-sandbox-admin)
21. [Sprint 6 — Marketing, Docs & Launch Hardening](#20-sprint-6--marketing-docs--launch-hardening)
22. [Quality, Security & Go-Live](#21-quality-security--go-live)
23. [Risk Register](#22-risk-register)
24. [Post-V1 — Specification v2 Roadmap](#23-post-v1--specification-v2-roadmap)
25. [Funding Model — Free Forever](#24-funding-model--free-forever)
26. [Appendix A — API Scope Catalog](#appendix-a--api-scope-catalog)
27. [Appendix B — Error Code Catalog](#appendix-b--error-code-catalog)
28. [Appendix C — RBAC Permission Catalog](#appendix-c--rbac-permission-catalog)
29. [Appendix D — Quota Cost Class Registry](#appendix-d--quota-cost-class-registry)
30. [Appendix E — Seed Data Contract](#appendix-e--seed-data-contract)
31. [Appendix F — Service / Port / Owner Registry](#appendix-f--service--port--owner-registry)

---

## 1. How to Use This Plan

**If you are an AI agent assigned a story:**

1. Find your story ID (e.g. `GEO-12.3`) in the sprint sections below.
2. Confirm your **lane** and the **directory you exclusively own** for that story (§8). Do not edit files outside your owned paths without leaving a coordination note on the task board (§1a).
3. Read the story's **Depends-On** field. If a dependency is not `Done`, build against its **published contract** (`contracts/openapi/*`, `contracts/graphql/*`, `proto/*`) using a stub or mock — never against its live database or internal code.
4. Create your branch: `feature/GEO-12.3-district-crud` (§9).
5. Implement to the story's **Acceptance Criteria** and the **Global Definition of Done** (§10).
6. Open a PR titled `GEO-12.3 <summary>`. PRs are the only merge path.
7. If you must change a shared contract, that is a **separate PR** to `contracts/` or `proto/` reviewed by the Contracts owner (§6). Never fork a contract locally.
8. Update the task board (§1a) when your story changes state.

**Reading order for onboarding:** §2 → §3 → §5 → §6 → §8 → §9 → §10, then your epic in §13+.
**Frontend agents additionally read `DESIGN_SYSTEM.md` in full before their first component.**

---

## 1a. Agent Task Board (live)

This section tracks work currently in-flight and recently completed. The active agent updates it after every batch of work.

**2026-08-29 — Planning lane.** Authored `agent_plan.md` and `DESIGN_SYSTEM.md` from Build Specification v2, the two governing manuals, and a grounded audit of the RentOS, Xtiitch and AuraEDU admin shells.

**2026-08-29 — Build lane.** Sprint 0 complete; the data core, REST v1, all three contracts, search and the design system run and are verified against live services, not mocks. MongoDB 8.0 replica set, Redis 7 and Typesense 29 healthy in Compose. Seed imports 16 regions / 261 districts / 16 places, proven idempotent. REST v1 serves 13 endpoints with the Spec §19 error envelope. Search indexes 293 documents and is typo-tolerant with domain-computed confidence. The tri-morphic design system renders in all six material × mode combinations with zero console errors.

**2026-08-29 — Product decision: free forever.** GhanaGeo has no paid tier and will not get one. Funded by donations and institutional sponsorship. Six rules in §24 exist so drifting back toward paid access requires a visible decision. This supersedes the pricing and billing items in Spec §14, §16 and §28.

**Port block.** This machine already runs other projects on 8080, 3003, 6379 and 27017, so GhanaGeo claims: Mongo `27117`, Redis `6679`, Typesense `8108`, API `8180`, gRPC `9190`, admin `3103`.

**Decision taken without escalating.** Typesense is the `SearchPort` implementation for local and CI; Atlas Search remains a swappable second implementation. This removes the managed-service dependency from the critical path (RK-6).

| Story | Epic | Status | Agent | Notes |
|---|---|---|---|---|
| — | EP-00 Planning | ✅ **Done** | Claude | `agent_plan.md` + `DESIGN_SYSTEM.md`. |
| GEO-1.1 | EP-01 Foundation | ✅ **Done** | Claude | Monorepo, pnpm workspace, Turborepo, `go.work`, source material relocated. |
| GEO-1.2 | EP-01 Foundation | ✅ **Done** | Claude | `CLAUDE.md` + `AGENTS.md`. |
| GEO-1.3 | EP-01 Foundation | ✅ **Done (code)** | Claude / Codex (L0/L1/L12) | Mongo replica set, Redis, Typesense, API and worker are defined in Compose with health/dependency wiring. `services/worker` is now a real Go module in `go.work`, consuming a strict indexed transactional outbox with leases, retry and dead-letter behavior. API/worker binaries and split HTTP/gRPC modes run locally; Compose/Render definitions validate. The production container build is CI-gated; a local Docker build reached registry metadata resolution but the workstation registry connection stalled before base images downloaded. |
| GEO-4.WORKER | EP-04 Ingestion / Async | ✅ **Done** | Codex (L1/L12) | Dataset publish/rollback now atomically updates the live catalogue and enqueues `dataset.published`. The worker performs an idempotent full Typesense rebuild, reclaims expired leases, retries exponentially and dead-letters exhausted/unknown work while reporting queue depth. Live evidence: event `01M17NCE2PNRTS5DH6J8BSN5ZP` rebuilt 16,201 documents in 827 ms and completed with zero pending/processing/dead; isolated pending and crashed-lease fixtures both reached dead at the two-attempt ceiling and were cleaned up. Evidence: `docs/runbooks/evidence/worker-outbox-2026-08-29.md`. |
| GEO-3.1 | EP-03 Schema | ✅ **Done** | Claude / Codex (L0/L1/L2/L3 coordinated) | Canonical region, district and place IDs are deterministic ULIDs derived from normalized source identity. The transactional migration preserves all relationships and merge lineage, emits tombstones for every published legacy ID, and is idempotent on a scratch and live replica set. Strict validators reject non-ULID IDs and references. Two migration runs plus an old-manifest seed replay preserve 16 regions, 261 districts, 15,941 places, 248 district geometries and zero dangling references; all 16,201 active records remain stamped `2026.08.3-ulid`. REST returns 410 + `mergedInto` for legacy slugs, REST/GraphQL expose the published release and changelog, Typesense was rebuilt with 16,201 ULID documents, and the OpenAPI example now matches the contract. |
| GEO-3.2 | EP-03 Schema | ✅ **Done** | Claude | Place-type, status and verification enumerations. |
| GEO-3.3 | EP-03 Schema | ✅ **Done** | Claude | Redirects; a merged id resolves to 410 with `mergedInto`. |
| GEO-4.1 | EP-04 Ingestion | ✅ **Done** | Claude | Seed import with checksum verification; idempotency proven. |
| GEO-4.2 | EP-04 Ingestion | ✅ **Done** | Claude | `data validate` asserts counts and zero orphan districts. |
| GEO-5.1 | EP-05 Matching | ✅ **Done** | Claude | Normalization: Twi/Ga/Ewe folding, Ghanaian abbreviations, determinism test. |
| GEO-5.3 | EP-05 Matching | ✅ **Done** | Claude | Geometry validation replacing PostGIS, with a known-bad-polygon suite. |
| GEO-7.1–7.4 | EP-07 Domain | ✅ **Done** | Claude | Entities, ports, Mongo repositories, geography use cases. |
| GEO-8.1–8.5 | EP-08 REST | ✅ **Done** | Claude | Nine endpoints, error envelope, cursor pagination, CORS allow-list. |
| GEO-14.1 | EP-14 Design | ✅ **Done** | Claude | Three-axis token system; 14 material tokens; verified in six combinations. |
| GEO-14.2 | EP-14 Design | ✅ **Done** | Claude | ThemeProvider, FOUC script, contrast clamp (360 hues × 2 modes pass). |
| GEO-14.3 | EP-14 Design | ✅ **Done** | Codex (L7) | Full required primitive inventory now exported, including Radix-backed overlays/menus, tabs, toast, table, pagination, breadcrumb, avatar, progress, alert and combobox. UI + all four consumer typechecks and web build pass. |
| GEO-14.7 | EP-14 Design | ✅ **Done** | Codex (L7; coordinated L0 workflow edit) | The production-build Playwright + axe gate passes all 180 combinations: 4 surfaces × 3 materials × 3 themes × 5 viewports, with zero WCAG A/AA violations, console/request/5xx errors, theme failures or body overflow. `quality.yml` now runs the same matrix as `Design QA (180-point matrix)`. Evidence in `design-qa.md`. |
| GEO-14.5 | EP-14 Design | ✅ **Done** | Claude | AppShell: sidebar, navbar, command palette. |
| GEO-14.6 | EP-14 Design | ✅ **Done** | Claude | Theme picker with live preview; needs mounting in portal and marketing. |
| GEO-2.1–2.5 | EP-02 Contracts | ✅ **Done** | Claude | OpenAPI 3.1 (redocly clean), protobuf (buf lint clean), GraphQL SDL, error catalog with a generator and drift tests. |
| GEO-12.1–12.4 | EP-12 Search | ✅ **Done** | Claude | Typesense SearchPort; search, autocomplete, geocode, reverse. Domain relevance scoring replaced the engine's unusable score. |
| GEO-29.1–29.4 | EP-16b Support | ✅ **Done** | Codex (L8) | Reconciled against source: public support route, shell support control, sponsor recognition and transparent reporting ledger all exist; donor-neutral quota language remains explicit. Web typecheck/build pass. |
| GEO-12.5 | EP-12 Search | ✅ **Done** | Claude | `/nearby` serves nationwide via the 2dsphere index; radius bounded and validated. p95 54.5 ms against a 500 ms target. |
| GEO-12.6 | EP-12 Search | ✅ **Done** | Claude | p95 acceptance gate (`make perf`) and index-plan gate (`make perf-indexes`). All nine Spec §22.4 targets pass; six hot queries confirmed IXSCAN with `totalDocsExamined` ≈ `nReturned`, no COLLSCAN. The gate caught a real regression: reading boundary geometry took the districts list to a 1046 ms p95, fixed by projecting geometry out of list and get paths (a 20× improvement). |
| GEO-18.NAV | EP-18 Marketing & Docs | ✅ **Done** | Codex (L8) | App-local floating navbar, mobile navigation and editorial footer deployed across all five public routes. Web typecheck + production build passed; rendered desktop check found no horizontal overflow. |
| GEO-18.REDESIGN | EP-18 Marketing & Docs | ✅ **Done** | Codex (L8) | Map-led public-site redesign across all five routes with live search, editorial content rhythm and developer pathways. Typecheck/build/link checks pass; browser sweep found zero horizontal overflow. |
| GEO-18.ABOUT | EP-18 Marketing & Docs | ✅ **Done** | Codex (L8) | Editorial, evidence-led `/about` redesign with manifesto hero, numbered commitments, source ledger, audience pathways and disclosure section. All source facts/caveats preserved; web typecheck + production build + link check + diff check pass; browser QA confirms zero overflow at 1480px and 390px. |
| GEO-18.TRANSPARENCY | EP-18 Marketing & Docs | ✅ **Done** | Codex (L8) | Public-ledger redesign with reporting status, cadence, honest pre-launch state, named income/cost tables, funding firewall and correction policy. Recorded values and null semantics preserved; web typecheck/build/link/diff checks pass; browser QA confirms zero page overflow at 1480px and 390px with contained mobile table scrolling. |
| GEO-18.MVP-PAGES | EP-18 Marketing & Docs | ✅ **Done (code)** | Codex (L8) | Products, Developers, Coverage, Changelog, Status and Contact are implemented and linked; sitemap, robots, Organization/WebSite/SoftwareApplication JSON-LD and generated Open Graph artwork ship with the static build. Typecheck/build/link checks pass; Playwright + axe finds zero violations or overflow across all six routes at 390px in light and dark modes. No documented public host currently resolves in DNS, so deployment and live Core Web Vitals remain external launch gates. Evidence: `docs/runbooks/evidence/external-launch-gates-2026-08-29.md`. |
| GEO-15.UX | EP-15 Sandbox | ✅ **Done** | Codex (L9) | REST workbench now has protocol chrome, sample library, run history, response metrics/states, and generated cURL/JavaScript/Go snippets. Sandbox typecheck + production build passed; live request returned HTTP 200 in browser. |
| GEO-16.REDESIGN | EP-16 Developer Portal | ✅ **Done** | Codex (L10) | Developer workspace redesign with API quick start, dataset/service status, tools, example activity and honest account states. Typecheck/build/link checks pass; rendered browser verification found zero horizontal overflow. |
| GEO-17.3 | EP-17 Admin | ✅ **Done** | Claude | Permission-checked edit and deprecate for regions, districts and places, plus the console screens: detail/edit pages, sign-in with MFA, and controls gated on the RBAC matrix (presentation only — the API enforces independently). Every mutation is audited, refusals included; deprecation writes a redirect so an old id returns 410 with `mergedInto`, never 404. Verified end to end in a browser. Roads, POIs and aliases are not modelled yet, so they are not covered. |
| GEO-17.ROUTES | EP-17 Admin | ✅ **Done** | Codex (L11) | All 66 configured admin destinations now build as real routes: implemented screens stay live and unfinished areas explain their scope without fake data. Added branded in-shell 404 and map-pin loading splash. Admin typecheck/build passed; browser verified `/ops/health` and an unknown URL with no horizontal overflow. |
| GEO-17.READINESS | EP-17 Admin | ✅ **Done** | Codex (L11) | Replaced the shared “not built yet” treatment across 59 planned routes with honest operator briefs: approved scope, service readiness, dependency/tracking metadata and live workspace shortcuts. Admin typecheck/build pass; rendered desktop/mobile QA has zero overflow. |
| GEO-14.OUTFIT | EP-14 Design | ✅ **Done** | Codex (L7) | Outfit is now the shared sans and display family across web, sandbox, portal, admin and UI tokens; redundant Fraunces loading/references removed while JetBrains Mono remains for code/data. Five typechecks, four production builds and computed-font browser checks pass. |
| GEO-14.BORDERLESS | EP-14 Design | ✅ **Done** | Codex (L7) | Cards, buttons, inputs, selects, icon controls, overlays and key app surfaces now use borderless neu/glass/clay depth treatments. Computed browser checks confirm 0px borders with distinct material shadows; five typechecks, four production builds and all route-link checks pass. |
| GEO-14.NATIVE-CONTROLS | EP-14 Design | ✅ **Done** | Codex (L7/L9/L10) | Removed every raw application select, checkbox, radio and browser date-time picker from JSX. Sandbox gRPC methods and portal organization/application/key/role/scope/expiry controls now use branded Radix or token-native primitives with keyboard/typeahead behavior and form serialization preserved. UI/sandbox/portal typechecks and both production builds pass; a clean production browser check confirms the open gRPC menu follows GhanaGeo dark-neu tokens instead of OS chrome. |
| GEO-14.DARK-DEPTH | EP-14 Design | ✅ **Done** | Codex (L7) | Dark neumorphism now uses tighter shadows and a low-opacity brand-tinted reflection instead of a broad white halo. Shared UI and portal typechecks, portal production build and diff check pass. |
| GEO-14.LOADING | EP-14 Design | ✅ **Done** | Codex (L7/L8/L9/L11) | Layout-matched skeletons now cover admin route/data loading, shared palette search, marketing search and sandbox requests; visible loading text and spinner implementations were removed. Five typechecks and four production builds pass. |
| GEO-15.3 | EP-15 Sandbox | ✅ **Done** | Codex (L9) | Live GraphQL workspace has introspection-backed schema docs, query completions, variables, restorable history, generated snippets and a weighted 300-point complexity warning. Typecheck/build, live query/introspection and 390/1440px no-overflow checks pass. |
| GEO-15.4 | EP-15 Sandbox | ✅ **Done** | Codex (L3/L9) | Allow-listed, 16 KiB-capped and 30 rpm gRPC bridge executes anonymous reads without privileged credentials; the method picker generates typed per-method forms. `StreamDatasetChanges` is exposed as a cancellable NDJSON stream with live output and resume-cursor input. Live `Search`/`Nearby`, enforced HTTP 429 and a three-event resumed stream pass. |
| GEO-15.5 | EP-15 Sandbox | ✅ **Done** | Codex (L9) | Leaflet/OpenStreetMap response panel renders coordinates and GeoJSON with permanent ODbL attribution and material-aware overlays. Live `Nearby` produced 10 markers with zero overflow at 390/1440px; typecheck/build pass. |
| GEO-15.6 | EP-15 Sandbox | ✅ **Done** | Codex (L9/L10) | One-click Osu, Adenta, Tema Community 25 and Kumasi samples exist. The account CTA serializes protocol, request and payload; the portal detects the handoff, opens registration, preserves it through authentication and exposes an in-portal replay for REST, GraphQL or the allow-listed gRPC bridge. Portal typecheck/build and a headless URL-handoff assertion pass. |
| GEO-17.PAGINATION | EP-17 Admin | ✅ **Done** | Codex (L11) | Places use 25-row cursor pages, districts use 25-row pages and Explorer uses 8-result pages; compact 16-region and curated lists remain intentionally unpaginated. Admin typecheck/build and rendered skeleton QA pass. |
| GEO-16.SPACING | EP-16 Developer Portal | ✅ **Done** | Codex (L10) | Portal dashboard gaps and panel padding now share a responsive rhythm after the material-depth pass. Portal typecheck/build and isolated production render pass. |
| GEO-14.TABLE-GUTTERS | EP-14 Design | ✅ **Done** | Codex (L7/L11) | Shared data tables now keep 20px desktop and 16px mobile edge gutters; the zero-padding override and ineffective table-element padding were removed from Regions, Districts and Places. UI/admin typechecks, 69-route admin build and diff check pass. |
| GEO-29.SUPPORT-PANEL | EP-16b Support | ✅ **Done** | Codex (L7) | Shared funding panel redesigned as a responsive monthly ledger with paired totals, explicit progress, scannable cost modules, stronger actions and a quieter equal-access assurance. UI/web/admin typechecks, public/admin builds, rendered production QA and diff check pass. |
| GEO-13.1–13.7 | EP-13 SDK Family | ✅ **Done (code)** | Codex (L6; coordinated L9/L12 integration) | Publishable `core`, `client`, `react`, `node`, offline `data`, umbrella `ghanageo` and generated `proto` packages now implement the V1 surface. OpenAPI/protobuf/data generation is drift-gated; React includes all 12 hooks, MSW/RTL acceptance coverage and a tested Next.js 16 hydration example; sandbox dogfoods the client; offline data carries 16 regions, 261 districts and 15,924 places at `2026.08.3-ulid`; umbrella ESM/CJS runtime imports pass. All seven tarballs build with rewritten public dependency ranges, expected files and no embedded API key. Registry publication remains the explicit external launch action in the V1 checklist. |
| GEO-13.8 | EP-13b Public CLI | ✅ **Done** | Claude | 9 commands, table/JSON/CSV, 7 platform binaries, npm wrapper. No API key required. |
| GEO-13.9 | EP-13b Public CLI | ✅ **Done** | Codex (L6/L12; coordinated L0 workflow edit) | Tag-driven workflow builds seven static targets, embeds the API target, verifies checksums/npm contents, publishes npm, creates GitHub release assets and updates Homebrew. `go test ./...`, launcher tests, `verify-cli-release.sh 0.1.0-test.1 v1`, checksum verification, Ruby syntax and `actionlint` pass. |
| GEO-9.1–9.6 | EP-09 Identity | ✅ **Done** | Claude | Keys (argon2id, secret shown once), browser-key safety rules, anonymous-as-identity, Redis token bucket, cost classes, limit headers, immediate revocation. Verified live: 429 under concurrent load, 401 on revoke, no prefix enumeration. |
| GEO-9.7 | EP-09 Identity | ✅ **Done** | Claude | Append-only audit log with a tamper-evident hash chain. Actor/action/target/before/after/request-id/outcome; failed attempts recorded, not just successes; secrets redacted on the way in. `audit verify` detects edits, deletions and insertions — proven live against direct `mongosh` tampering. Wired into key create/revoke, dataset publish, reindex and district assignment. |
| GEO-9.8 | EP-09 Identity | ✅ **Done** | Codex (L4) | Signed webhook plus structured-log alerts cover rejected browser origins/IP allow-lists, shared-transport quota exhaustion and failed privileged authentication. Webhook delivery is time-bounded and fails open while its failures remain observable. Exact IPv4/IPv6 and CIDR key restrictions are now enforced. Focused delivery/signal tests, full API tests, vet, env-generator smoke, diff and GhanaPost exclusion checks pass. |
| GEO-9.3 | EP-09 Identity | ✅ **Done** | Codex (L4) | Owner-isolated Developer → Organization → Application → Keys hierarchy with test/live formats, one-time Argon2id-backed secrets, classes/scopes, expiry, origin and exact-IP/CIDR restrictions, last-used metadata, rotation and immediate revocation. Mutations are audit-chained without secrets. Full API tests/vet plus a live register → verify → login → org → app → create → rotate → revoke flow pass; the 20-entry audit chain remained intact. |
| GEO-16.2–16.4 | EP-16 Portal | ✅ **Done** | Codex (L4/L10; coordinated L0 contract) | Members/roles, invitations, revocation, acceptance and owner-only transfer work end to end. Applications persist test/live environments, domains, callbacks and the free plan. Keys expose class/scope, expiry, IP/origin restrictions, copy-confirm, rotation and revocation. Full API tests, a live two-account lifecycle, the 45-point portal matrix and the published method-aware OpenAPI contract/docs pass; replayed sessions and former-owner transfer are denied. |
| GEO-16.5–16.6 | EP-16 Portal | ✅ **Done** | Codex (L4/L10; coordinated L0 contract) | Durable, secret-free REST/GraphQL/gRPC telemetry records request ID, normalized operation, status/error, latency, quota charge/remaining and geography. Mongo enforces strict schema/indexes and the V1 30-day TTL. Member-authorized summaries and cursor pages drive themed charts, skeleton/empty/error states and keyboard-scrollable logs. Full API tests/vet, live three-protocol aggregation, pagination/isolation/redaction checks, the portal matrix and published REST docs pass. Contract-specific enterprise retention remains the explicit post-MVP GEO-25.6 story. |
| GEO-9.2 | EP-09 Identity | ✅ **Done** | Claude | Email/password with verified email, **passkeys/WebAuthn** with TOTP and single-use recovery codes, short-lived sessions with per-request rotation and global revocation, MFA mandatory for every non-developer role. Session fixation impossible by construction; a replayed session token revokes every session; a passkey whose sign count does not advance is treated as cloned. Verified end to end with Chrome's virtual authenticator (`scripts/passkey-e2e.mjs`) — a steward signs in with a passkey alone and gets an authenticated DATA_ADMIN session. |
| GEO-21.7 | EP-20 Security | ✅ **Done** | Codex (L12; coordinated L0 workflow edit) | CI scans canonical data, seeds, exports, fixtures and migrations for GhanaPostGPS provider payloads and digital-address patterns. Clean corpus, positive regression fixtures, workflow lint and diff check pass. |
| GEO-21.5 | EP-20 Security | ✅ **Done** | Codex (L12/L0) | Scheduled and PR CI runs CodeQL for Go/JS, `govulncheck` on API/CLI and production `pnpm audit`; patch SLAs are documented. Go was raised from vulnerable 1.26.5 to 1.26.6. Local scans report zero reachable Go and zero npm vulnerabilities; vet/actionlint pass. |
| GEO-21.REVIEW | EP-20 Security | ✅ **Done** | Claude + Codex | **No unresolved critical or high findings.** `govulncheck` reports 0 reachable vulnerabilities; `gosec` raised 27 across 84 files, all resolved or triaged with reasons; `pnpm audit --prod` is clean. Two genuine issues fixed: unbounded argon2 parameters read from a stored digest (a tampered row requesting a 4TB allocation would have taken the process down on one sign-in, and on API keys that is every request), and the transitive mongo-driver GSSAPI advisory, bumped to the patched v1.17.9. Local §22.3 suite passes. Evidence: `docs/runbooks/evidence/security-review-2026-08-29.md`. **Hosted scanning remains blocked on account state, not code:** GitHub Actions rejected run `33272922624` for billing, and CodeQL is not enabled for this private repository — both need action on the GitHub account. |
| GEO-22.1 | EP-21 Backups & DR | 🟡 **Blocked on cluster tier** | Claude + Codex | Production is now deployed and seeded, and the archive/restore path is proven end to end against it (GEO-22.2). **Cannot close:** the criterion is Atlas *continuous backup with point-in-time restore*, and this cluster is **M0 (shared/free)**, where those features do not exist — they begin at M10. Verified directly: admin commands return AtlasError. Until the cluster is upgraded, RPO equals the age of the last manual `mongodump`. This is a billing decision, not a code or tooling gap. |
| GEO-22.2 | EP-21 Backups & DR | ✅ **Done** | Claude + Codex | **Production restoration drill executed, measured and documented.** The dataset was deployed to Atlas, then backed up (78MB, SHA-256 recorded, 136.5s) and restored into an isolated scratch database on the same cluster (346.3s, **87,798 documents, 0 failed**). All 18 collections matched source counts exactly, indexes restored 6/6 including both 2dsphere, scratch dropped and production re-verified intact. **Measured RTO ≈ 8 minutes.** Evidence: `docs/runbooks/evidence/restore-drill-production-2026-08-29.md`. |
| GEO-22.4 | EP-21 Backups & DR | ✅ **Done** | Codex (L12) | Operator runbooks cover incident response, key compromise, bad dataset rollback, ETL failure, quota/abuse response and quarterly backup restoration. |
| GEO-22.3 | EP-21 Deployment | ✅ **Done (code)** | Codex (L0/L12) | Production Docker boundary and schema-valid Render Blueprint now deploy REST/GraphQL with assigned-port binding, health checks, persistent export storage and dashboard-supplied secrets. The API supports explicit `http`, `grpc` and local `all` modes; focused tests plus live split-mode probes returned HTTP dataset `2026.08.3-ulid`, gRPC `SERVING`, and no stray HTTP listener in gRPC mode. Render officially cannot expose native gRPC, so the runbook routes the identical gRPC-mode image to an HTTP/2-native host rather than claiming an invalid Render deployment. Public provisioning/DNS remains external. |
| GEO-20.1–20.5 | EP-19 Observability & SLOs | ✅ **Done (code)** | Codex (L12/L3) | API and worker now own OTel lifecycle; HTTP/gRPC, Mongo, Redis and Typesense spans share trace context; Redis statement capture is explicitly disabled. Secret-free JSON request logs carry request/trace IDs, app ID and public key prefix. Prometheus exposes bounded request/error/latency, dependency, GraphQL cache hit/miss, limiter, queue-depth, ETL-lag and index-lag signals; worker has dedicated health/metrics endpoints. Separate read/publishing SLOs, burn-rate alerts and incident communication are documented. Full API/worker tests and vet pass; live request, correlated trace, metrics and worker probes are recorded in `docs/runbooks/evidence/observability-2026-08-29.md`. Production OTLP/alerts and public status activation remain external deployment gates. |
| GEO-10.1–10.5 | EP-10 GraphQL | ✅ **Done** | Claude | gqlgen generated FROM the published contract. Resolvers over shared use cases, per-request loaders, depth and complexity budgets, error codes matching REST. |
| GEO-5.2 | EP-05 Dedupe | ✅ **Done** | Claude | Cross-source duplicate detection. All 16 seed capitals merged with their GeoNames twins; merged ids resolve 410 with `mergedInto`. |
| GEO-11.x | EP-11 gRPC | ✅ **Done** | Claude / Codex | 12 unary RPCs plus resumable `StreamDatasetChanges` run on :9190 over shared use cases/audit events, with health and reflection. Error codes map from the shared catalog. The stream tails successful geography mutations, maps update/merge/deprecation semantics, resumes after a durable cursor and charges both connection and delivered messages. Focused tests/vet and a live three-event `grpcurl` resume pass. |
| GEO-11.SECURITY | EP-11 gRPC / EP-20 Security | ✅ **Done** | Codex (L3/L4; coordinated L0 wiring) | gRPC now uses the shared key resolver and Redis fair-use limiter, weighted RPC costs, `grpc:access` enforcement for keyed callers, 15s unary/5m stream deadlines, 1 MiB send/receive limits and structured request logs. Health/reflection bypass consumer quota. Full API tests plus live headers, 401, 429 and health checks pass. REST/GraphQL route scopes are enforced while anonymous public reads remain valid. |
| GEO-8.3 | EP-08 REST | ✅ **Done** | Claude | `/datasets` and `/datasets/{version}/downloads` plus artifact streaming. `data export` generates real GeoJSON/CSV; every checksum and size is computed from bytes actually written. Fixed a silent persistence bug: geometry was written but never read back, so 16 region and 248 district boundaries were invisible to the domain. |
| GEO-4.4, 4.8 | EP-04 Ingestion | ✅ **Done** | Claude | Adapter port + GeoNames (CC BY). 15,925 places with coordinates, zero rejections. `/reverse` and `/nearby` now work nationwide. |
| GEO-4.6 | EP-04 Ingestion | ✅ **Done** | Claude | **261/261 districts and 16/16 regions have boundary geometry.** geoBoundaries (CC BY, 2019) supplies 248; OpenStreetMap `admin_level=6` (ODbL, current) fills the remaining 13, including Guan District which was inaugurated in October 2021 and could not exist in the 2019 release. OSM only fills gaps — an existing boundary is never overwritten (R8). Matching is region-gated on an area-weighted interior point, not the district's own polygon: a boundary shares edges with its neighbours, so polygon intersection returned an arbitrary region and mis-gated two districts. All 13 matched at 1.00. Documented in `docs/boundary-coverage.md`. |
| GEO-4.7 | EP-04 Ingestion | ✅ **Done** | Claude | Geofabrik Ghana extract: 19,728 named roads, 19,686 POIs, plus `admin_level=6` boundaries. 99.5% of roads and 99.6% of POIs resolve to a district by containment; the remainder sit offshore or across a border and are left unassigned rather than snapped. ODbL attribution enforced by the domain validator, the Mongo schema, every API response (`/v1/roads`, `/v1/pois`) and every export file at file and feature level. Settlements are scanned and reported but deliberately not imported as places — `data import` owns place identity, and a second writer would fork it. |
| GEO-23.1-METADATA | EP-22 Acceptance | ✅ **Done** | Codex (L1/L2) | Domain writes and strict Mongo validators now require a stable id, dataset version and complete provenance (`sourceId`, `externalId`, `retrievedAt`, 64-character payload hash). Importers generate deterministic source hashes; migration backfills legacy rows before tightening validators. Live migration passes twice, Mongo rejects an invalid metadata write with code 121, all 16,218 stored records pass the corpus query, and the §22.1 validator reports 0/16,201 active records incomplete. Two seed replays preserve the identical ID fingerprint and, after fixing merge-upsert semantics, all 16 region and 248 district geometries. Full Go/workspace tests, vet/lint, admin production build and diff check pass. |

**Legend:** ✅ Done & verified · 🟡 In progress · ⬜ Not started · 🔴 Blocked

### 2026-08-29 unresolved-work reconciliation

- **Completed and hardened:** GEO-11.1–11.5 gRPC transport is live; shared authentication/rate limiting, scopes, deadlines, message limits, logs, health and reflection are verified. `StreamDatasetChanges` now uses successful append-only geography audit events as a real cursor-resumable feed and charges each delivered message.
- **Completed:** GEO-13.9 CLI distribution/release automation now has a locally verified tag workflow, seven-platform npm bundle, checksums, Homebrew formula generation and explicit API-target reporting.
- **Completed in code, release pending:** GEO-13.1–13.7 now ships the complete V1 TypeScript SDK family and sandbox integration. All seven consumer tarballs, generated-artifact drift checks, React acceptance/SSR behavior, offline dataset versioning and umbrella ESM/CJS entry points pass locally; npm publication remains credential-gated external work.
- **Completed, security-critical:** GEO-9.7 immutable audit logging and GEO-9.8 signed security-event alerting now cover privileged actions, unusual key use, quota exhaustion and failed administrator authentication.
- **Completed developer journey:** GEO-9.2 and GEO-16.2–16.6 now pass, including passkeys, invitations, ownership transfer, application metadata, expiry controls, attributed usage across REST/GraphQL/gRPC, the full UI matrix and method-aware published REST contract/docs. Contract-specific enterprise retention remains post-MVP GEO-25.6.
- **Data/external dependency:** GEO-4.6 GSS boundaries still requires an official, licence-recorded source artefact. GEO-4.7 OSM ingestion is separately implementable, but district assignment and remaining polygon golden cases stay incomplete until authoritative boundary coverage is reconciled.
- **Completed data-contract correction:** GEO-3.1 now satisfies R7 with deterministic ULIDs, strict reference validators, relationship-safe migration, compatibility tombstones, seed-replay stability, a published `2026.08.3-ulid` release and matching OpenAPI/runtime examples.
- **Stale combined item:** `/nearby` already works nationwide; GEO-12.5–12.6 should be split so the implemented nearby path can close independently while the p95 load gate remains measurable work.
- **Completed quality gate:** GEO-14.7 now passes the full 180-point production-build matrix and is wired into `quality.yml`; `design-qa.md` records the exact fixes and evidence.

---

## 2. Non-Negotiable Engineering Rules

These are hard rules. A PR that violates one is rejected regardless of test status.

### 2.1 Legal & data rules

| # | Rule |
|---|---|
| R1 | **GhanaPostGPS digital-address data must never be scraped, stored, or redistributed.** GhanaPost integration exists only as an adapter interface behind a feature flag, activated solely under a signed licence. CI runs a check that fails on any GhanaPost address payload in fixtures, seed data, or migrations. |
| R2 | Every canonical record carries **source provenance** (`source_id`, `external_id`, `retrieved_at`, `source_payload_hash`) and a **dataset version**. A record with no provenance cannot be published. |
| R3 | **PII is stripped at ingestion**, not at serving. The GNHR communities endpoint exposes focal-person/contact fields; the ingestion adapter drops them before the raw landing write. A test asserts no PII-shaped column reaches staging. |
| R4 | Source licences are recorded in `docs/licensing-register.md` before that source's adapter is written. OSM requires ODbL attribution; GeoNames requires CC BY attribution. Attribution text ships in API responses and downloads. |
| R5 | Seed rows marked `SEED_NEEDS_CANONICAL_RECONCILIATION` must never be promoted to canonical `PUBLISHED` status by an automated path. |

### 2.2 Architecture rules

| # | Rule |
|---|---|
| R6 | **Transports contain no business logic.** REST handlers, GraphQL resolvers and gRPC services validate their wire format, map to an application command/query, invoke the shared use case, and map the result back. Any `if` statement expressing a domain rule inside a transport is a bug. |
| R7 | **Stable IDs are ULIDs.** Human-readable codes are alternate identifiers, never primary keys. IDs never change across reimports; merges create redirects, not deletions. |
| R8 | **Every import is idempotent.** Re-running an import with the same input produces no change. No source overwrites canonical data merely by arriving later — reconciliation rules decide precedence. |
| R9 | **No user input is ever interpolated into a query document.** All MongoDB filters are built from typed structs, never from raw maps assembled from request data. `$where`, `$function` and `mapReduce` are forbidden. The API connects with a least-privilege user scoped to its own database, without `dbAdmin`. |
| R10 | **Dataset publication is decoupled from code deployment** and is independently rollback-able. |
| R11 | The API is a **modular monolith** in Go with ports/adapters boundaries. Do not split into services without an ADR justifying it. |
| R12 | **No secrets in source or client bundles.** CI secret-scans every commit. Browser keys are origin-restricted and scope-limited by construction. |

### 2.3 Process rules

| # | Rule |
|---|---|
| R13 | Every repository contains `CLAUDE.md` and `AGENTS.md` defining project rules, coding standards, the Jira workflow, the GitHub workflow and AI operating procedures. (Workflow Manual, "CLAUDE.md and AGENTS.md".) |
| R14 | Agents must not modify unrelated code. Preserve in-flight work in a shared worktree; leave a coordination note. |
| R15 | Documentation is updated in the same PR when a change affects a documented workflow or contract. |
| R16 | Forward-only reviewed migrations. Schema is enforced by **MongoDB JSON Schema validators** committed as migrations — a collection without a validator cannot ship. Every migration has a tested restore path. |
| R17 | Frontend PRs satisfy `DESIGN_SYSTEM.md`, including the accessibility gates in its Design-QA checklist. |

---

## 3. Technology Decisions

Locked for V1. Changing a locked choice requires an ADR in `docs/adr/`.

| Layer | Decision | Rationale |
|---|---|---|
| **Backend language** | **Go 1.27.0**, modular monolith, ports/adapters | Spec §5.1. Typed, fast, natural gRPC fit, single deployable. |
| **Database** | **MongoDB 8.3** with `2dsphere` indexes; driver `go.mongodb.org/mongo-driver/v2 v2.8.2` | Native GeoJSON storage, `$geoIntersects` boundary containment, `$nearSphere` proximity, `$geoWithin` bounding queries. Deviation from Spec §5.1 (PostgreSQL/PostGIS) — see ADR-0002. |
| **Migrations** | `migrate-mongo`-style numbered Go migrations creating collections, JSON Schema validators and indexes | R16. Mongo is schemaless by default; validators make it not so. |
| **Cache / rate limit** | Redis 7 | Spec §5.1. Token-bucket / GCRA counters, hierarchical keys. |
| **Search** | **MongoDB Atlas Search** (Lucene) via a `SearchPort` interface; **Typesense** as the self-hosted fallback implementation | Self-hosted MongoDB `$text` has **no fuzzy matching or typo tolerance**, which Spec §10 requires. Atlas Search provides `autocomplete` and `fuzzy{maxEdits}` operators with scoring. The port keeps both swappable. See RK-6. |
| **Async** | Transactional outbox + Go worker | Spec §5.1. No external queue in V1. |
| **REST** | Go stdlib `net/http` + `chi`, OpenAPI 3.1 generated from code and diffed against `contracts/openapi/v1.yaml` | Contract is the artifact, not the code. |
| **GraphQL** | `gqlgen` (schema-first), DataLoader, complexity + depth limits, persisted queries | Spec §8.4. |
| **gRPC** | `protoc` + `buf` (lint + breaking-change checks), `ghanageo.v1` package | Spec §9.2. |
| **gRPC-Web** | ConnectRPC (`connectrpc.com/connect`) for the sandbox playground | Spec §9. Browsers cannot speak native gRPC. |
| **Frontend framework** | **Next.js 16.3.3** (App Router) + **React 19.2.8** + **TypeScript 5.9.3** strict | Spec §5.1. The locked workspace compiler is authoritative. A later compiler upgrade requires its own compatibility pass across `packages/config` and every application; roadmap prose must not claim an unreleased/uninstalled toolchain. |
| **Styling** | **Tailwind CSS 4.3.3** (`@theme`, CSS-first config) over a CSS custom-property token layer | The tri-morphic token architecture in `DESIGN_SYSTEM.md` requires runtime-swappable CSS variables; Tailwind v4 reads them natively. |
| **Component primitives** | **Radix UI** primitives (`@radix-ui/react-* 1.1.x`), wrapped in `packages/ui`; `lucide-react 1.37.0` icons; `class-variance-authority 0.7.1` + `tailwind-merge 3.6.0` | Unstyled + accessible: mandatory, because the three visual styles must swap without touching component logic. |
| **Motion** | **Pure CSS + the native View Transitions API** in `apps/admin`, `apps/portal` and `apps/sandbox` — **zero animation-library JS**. **`motion` 13.1.1** + **GSAP 3.15.0**/ScrollTrigger in `apps/web` (marketing) **only**. | House rule, independently converged on by all three reference codebases: RentOS ships zero animation dependencies, Xtiitch admin ships zero, and AuraEDU's spec permits Framer Motion on marketing only. Enforced by a per-workspace ESLint `no-restricted-imports` rule. See `DESIGN_SYSTEM.md` §2, §9. |
| **Maps** | **Leaflet 1.9.4** + **react-leaflet 5.0.0** + `@types/leaflet 1.9.22` + `leaflet.markercluster 1.5.3` | **Requires no API key** — OpenStreetMap raster tiles are free with attribution. Lighter than a GL renderer and sufficient for the admin explorer and sandbox map. No Google Maps dependency anywhere in the product. |
| **Data fetching** | **TanStack Query 5.102.8** | Already the contract of `@ghanageo/react` (starter zip). |
| **Charts** | **Recharts 3.10.1** for portal analytics | Composable, SSR-safe, themeable from CSS vars. |
| **Monorepo** | pnpm workspaces + **Turborepo 2.10.12**; Go workspace (`go.work`) for services | Spec §23 recommends a monorepo to keep contracts synchronized. |
| **Testing** | Go: `testing` + `testcontainers-go` (**MongoDB replica set** + Redis). Frontend: **Vitest 4.1.11** + React Testing Library + **MSW 2.15.0**. E2E: **Playwright 1.62.1**. | Spec §22. A replica set is required because multi-document transactions need one. |
| **Observability** | OpenTelemetry traces/metrics/logs, Prometheus-compatible export | Spec §20. |
| **Hosting** | Render (Go API, Redis, worker) · **MongoDB Atlas** (database + Atlas Search) · Vercel (Next.js apps) · Cloudflare (WAF/CDN in front) | Spec §5.1, amended for MongoDB. Atlas is chosen over self-hosted specifically to get Atlas Search. |
| **Email** | Resend | Spec §5.1. |
| **Package manager** | **pnpm 11.24.0** | Workspace protocol, strict peer resolution. |

**Version policy:** every version above is the latest published release as of **2026-08-29**, verified against the npm registry, the Go module proxy and Docker Hub. Versions are **pinned exactly** in `package.json` and `go.mod` — no `^` or `~` ranges — and bumped deliberately by a scheduled dependency PR, never drifted into.

**Explicitly deferred:** self-hosted Typesense (unless Atlas is dropped), managed queue/pub-sub, vector tiles service, Python/Dart/Java SDKs, OAuth2 client credentials, SSO/SAML, webhooks. All are post-V1 (Spec §28).

---

## 4. Monorepo Folder Structure

```
ghanageo/
├── CLAUDE.md                      # AI operating procedures (R13)
├── AGENTS.md                      # agent rules + lane map (R13)
├── agent_plan.md                  # this file
├── DESIGN_SYSTEM.md               # mandatory UI/UX + motion spec
├── apps/
│   ├── web/                       # marketing site + documentation hub   (L8)
│   ├── sandbox/                   # public interactive playground        (L9)
│   ├── portal/                    # authenticated developer portal       (L10)
│   └── admin/                     # admin + data-steward portal          (L11)
├── services/
│   ├── api/                       # Go modular monolith (REST+GraphQL+gRPC)
│   └── worker/                    # ETL / outbox / export / index worker
├── packages/
│   ├── ui/                        # shared design system (tokens, primitives, shell)
│   ├── core/                      # @ghanageo/core   — types + utilities
│   ├── client/                    # @ghanageo/client — REST/GraphQL client
│   ├── react/                     # @ghanageo/react  — TanStack Query hooks
│   ├── node/                      # @ghanageo/node   — server helpers
│   ├── data/                      # @ghanageo/data   — offline dataset assets
│   ├── proto/                     # @ghanageo/proto  — generated TS clients
│   └── config/                    # shared tsconfig / eslint / tailwind preset
├── proto/
│   └── ghanageo/v1/*.proto        # protobuf source of truth
├── contracts/
│   ├── openapi/v1.yaml            # REST contract
│   ├── graphql/schema.graphql     # GraphQL SDL contract
│   └── errors/catalog.yaml        # error code catalog (Appendix B)
├── data/
│   ├── schemas/                   # JSON Schema for source payloads
│   ├── transforms/                # normalization + matching rules
│   └── seed-data/                 # regions.csv districts.csv places.csv sources.csv manifest.json
├── infra/
│   ├── docker/                    # local Compose stack
│   ├── render/                    # render.yaml
│   └── cloudflare/
└── docs/
    ├── adr/                       # architecture decision records
    ├── licensing-register.md      # R4
    ├── runbooks/
    └── errors/                    # public error documentation pages
```

**Seed data note:** the three CSVs currently at repository root (`ghanageo_regions_seed.csv`, `ghanageo_districts_seed.csv`, `ghanageo_places_seed.csv`) and `GhanaGeo_Initial_Seed_Data.xlsx` move to `data/seed-data/` in `GEO-1.1` and are renamed per Spec §31. The `.docx` specifications move to `docs/spec/`.

---

## 5. Service Internal Structure (Hexagonal)

`services/api` is one Go module with enforced internal boundaries. `internal/` prevents external import; an import-lint rule (`go-arch-lint` or `depguard`) enforces direction.

```
services/api/
├── cmd/
│   ├── api/main.go                # HTTP + gRPC server bootstrap
│   └── ghanageo/main.go           # CLI: data seed | validate | reconcile | release
├── internal/
│   ├── domain/                    # entities, value objects, invariants. ZERO dependencies.
│   │   ├── geography/             # Region, District, Place, Alias, Road, POI, Boundary
│   │   ├── dataset/               # DatasetVersion, Release, ChangeRequest
│   │   └── identity/              # Org, Application, APIKey, Scope, Role
│   ├── app/                       # use cases (commands + queries). Depends on domain + ports only.
│   │   ├── geography/
│   │   ├── search/
│   │   ├── dataset/
│   │   └── identity/
│   ├── ports/                     # interfaces the app layer needs
│   │   ├── repository.go
│   │   ├── search.go              # SearchPort — Atlas Search today, Typesense fallback
│   │   ├── cache.go
│   │   ├── clock.go
│   │   └── ghanapost.go           # adapter interface only; no implementation in V1 (R1)
│   ├── adapters/                  # driven side
│   │   ├── mongo/                 # repositories + 2dsphere geo queries
│   │   ├── redis/                 # cache + rate-limit counters
│   │   ├── search/atlas/          # Atlas Search SearchPort implementation
│   │   └── outbox/
│   ├── transport/                 # driving side — NO business logic (R6)
│   │   ├── rest/
│   │   ├── graphql/
│   │   └── grpc/
│   └── platform/                  # cross-cutting: auth, quota, ratelimit, otel, audit, requestid
└── migrations/                    # numbered forward-only Go migrations (collections, validators, indexes)
```

**Dependency direction:** `transport → app → domain`, and `app → ports ← adapters`. Nothing in `domain` imports `app`, `adapters`, or any library beyond the standard library and a ULID package — **in particular, no `bson` tags in `domain`**. BSON mapping lives in `adapters/mongo`, so the database representation can change without touching the domain.

---

## 6. Contracts-First: The Key to Parallelism

Contracts are published in Sprint 0 **before** implementations exist. Every downstream lane builds against the contract, not the implementation. This is what allows twelve lanes to run concurrently.

| Contract | Path | Owner | Consumers |
|---|---|---|---|
| REST | `contracts/openapi/v1.yaml` | L0 | L3, L6, L8, L9, L10 |
| GraphQL | `contracts/graphql/schema.graphql` | L0 | L3, L6, L9 |
| gRPC | `proto/ghanageo/v1/*.proto` | L0 | L3, L6, L9 |
| Error catalog | `contracts/errors/catalog.yaml` | L0 | all |
| Design tokens | `packages/ui/src/tokens/*` | L7 | L8, L9, L10, L11 |
| Seed manifest | `data/seed-data/manifest.json` | L1 | L1, L2, L11 |
| Domain events | `contracts/events/*.json` | L0 | L1, L12 |

**Rules:**

- A contract change is **always a separate PR** touching only `contracts/` or `proto/`, reviewed by the L0 owner.
- CI runs `buf breaking`, an OpenAPI diff and a GraphQL schema-snapshot check against the previous release on every PR. Breaking changes fail the build unless the PR carries the `contract-break-approved` label.
- Mock servers are generated from the contracts (`prism` for OpenAPI, `gqlgen` stubs for GraphQL, `buf curl` for gRPC) so frontends develop before the API exists.
- **Semantic parity is a contract too.** A single fixture query must return semantically identical results across REST, GraphQL and gRPC (Spec §22.2). `GEO-14.4` owns that test.

---

## 7. Cross-Cutting Platform Concerns

Built once in `internal/platform`, applied by middleware to all three transports. No lane reimplements these.

| Concern | Mechanism | Story |
|---|---|---|
| Request ID | Generated at edge, propagated to logs/traces/errors, returned in every response | GEO-9.1 |
| Identity | API key (REST/gRPC metadata/GraphQL header) or session cookie; anonymous is an explicit identity, not an absence | GEO-9.2 |
| Scope check | Declarative per operation, from Appendix A | GEO-9.4 |
| Rate limit | Redis GCRA, hierarchical IP → account → org → app → key → endpoint class | GEO-9.5 |
| Quota | Cost-class accounting (Appendix D); GraphQL cost computed from requested fields | GEO-9.6 |
| Audit | Every privileged mutation writes an immutable audit row with actor, before/after, request ID | GEO-9.7 |
| Tracing | OpenTelemetry spans: gateway → transport → use case → repository/cache/search | GEO-20.1 |
| Logging | Structured JSON with `request_id`, `trace_id`, `app_id`, `key_prefix`, `protocol`, `operation`, `latency_ms`. **Never** the key secret. | GEO-20.2 |
| Errors | Stable machine codes mapped per protocol (Spec §19, Appendix B) | GEO-9.8 |
| Dataset version | Injected into every response body or metadata frame | GEO-6.2 |

---

## 8. Parallelization Model — Lanes, Ownership & Collisions

Each lane **exclusively owns** its directories. Two agents never edit the same file concurrently. Shared registry files (route tables, sidebar config, DI wiring) are edited **centrally by the owning lane only**; other lanes request an entry via a coordination note on the task board.

| Lane | Name | Exclusively owns | Depends on |
|---|---|---|---|
| **L0** | Contracts & Platform Foundation | `contracts/`, `proto/`, `packages/config/`, root CI, `CLAUDE.md`, `AGENTS.md` | — |
| **L1** | Data & ETL | `data/`, `services/api/migrations/`, `services/worker/internal/etl/` | L0 |
| **L2** | Domain & Use Cases | `services/api/internal/domain/`, `internal/app/`, `internal/ports/` | L0 |
| **L3** | Transports | `services/api/internal/transport/` | L0, L2 |
| **L4** | Identity & Gateway | `services/api/internal/platform/`, `internal/domain/identity/`, `internal/app/identity/` | L0, L2 |
| **L5** | Search & Spatial | `services/api/internal/adapters/search/`, `internal/app/search/`, geo-index migrations | L1, L2 |
| **L6** | SDKs | `packages/core/`, `client/`, `react/`, `node/`, `data/`, `proto/` | L0 |
| **L7** | Design System | `packages/ui/` | L0 |
| **L8** | Marketing & Docs | `apps/web/` | L6, L7 |
| **L9** | Sandbox | `apps/sandbox/` | L6, L7 |
| **L10** | Developer Portal | `apps/portal/` | L4, L6, L7 |
| **L11** | Admin & Data Steward | `apps/admin/` | L1, L4, L6, L7 |
| **L12** | Platform Ops | `infra/`, `docs/runbooks/`, observability wiring | L0 |

### Known collision points and their resolution

| Collision | Resolution |
|---|---|
| `services/api/internal/adapters/mongo/` — L1 writes validators/indexes, L2 writes repositories | L1 owns `migrations/`; **L2 owns repository Go files**. L1 never edits `.go` in `adapters/mongo/`. |
| `cmd/api/main.go` DI wiring | **L0 owns it.** Other lanes add a constructor and request a wiring line via the task board. |
| `packages/ui` token files vs consumer apps | L7 owns tokens. Apps consume only; an app never redefines a token locally. A needed token is a request to L7. |
| Admin sidebar route registry (`apps/admin/src/config/navigation.ts`) | **L11 owns it centrally.** Matches the pattern proven in RentOS (`client/App.tsx` + `Sidebar`) where central editing avoided concurrent-edit conflicts. |
| `contracts/` | L0 only, always a separate PR (§6). |

---

## 9. Ticket Conventions

Per the Workflow Manual ("Jira Standards", "GitHub Standards").

**Hierarchy:** `Client → Project (GEO) → Epic → Story → Subtask`

**Every story must include** (Workflow Manual, "Jira Standards"):
User Story · Business Value · Acceptance Criteria · Technical Notes · Definition of Done · Estimates · Dependencies

**Naming:**

```
Branch:   feature/GEO-12.3-district-crud
Commit:   GEO-12.3 add district repository with region FK validation
PR title: GEO-12.3 District CRUD
```

**Jira workflow states** (Ops Manual, "Jira Workflow"):

```
Lead → Discovery → Requirements → Solution Design → Backlog Grooming → Sprint Planning
 → Development → Code Review → QA → Staging → UAT → Beta → Production → Sign Off → Support → Closed
```

**Approval gates** (Ops Manual, Phases 2–11): Client approves requirements · Engineering Lead approves solution design · PM approves the groomed backlog · QA approves staging · Client approves UAT · Client signs project acceptance.

Merged PRs automatically update Jira status and the internal Project Dashboard, which must display: Client · Project · Epic · Story · Jira Key · GitHub Branch · Pull Request URL · Status · Assigned Developer · Estimated Effort · Actual Effort · Progress % · Last Updated.

---

## 10. Global Definition of Done

A story is Done only when **every** line is true. This extends the Workflow Manual's "Definition of Done" with GhanaGeo-specific gates.

**Baseline (Workflow Manual):**

- [ ] Code implemented
- [ ] Tests pass
- [ ] Pull Request approved
- [ ] Pull Request merged
- [ ] Jira updated
- [ ] Dashboard updated
- [ ] Documentation updated if required

**GhanaGeo additions:**

- [ ] `go vet`, `golangci-lint`, `gofumpt` clean (Go) / `tsc --noEmit`, ESLint, Prettier clean (TS)
- [ ] Unit tests for domain logic; integration tests against a real MongoDB replica set + Redis via testcontainers
- [ ] Contract checks pass: `buf lint`, `buf breaking`, OpenAPI diff, GraphQL schema snapshot
- [ ] New API surface is reflected in `contracts/` **and** its documentation page
- [ ] No secret, key, or credential in the diff (CI secret-scan)
- [ ] New dependencies scanned (SCA) with no unresolved critical/high
- [ ] Structured log lines added for new operations; no secret logged
- [ ] Error paths return catalog codes (Appendix B), never bare strings
- [ ] Migrations are forward-only and were applied to a scratch DB and rolled forward twice (idempotency)

**Frontend additions (L7–L11):**

- [ ] Renders correctly in **all three visual styles** (neumorphic, glass, clay) × **light, dark, and one custom theme**
- [ ] Passes the `DESIGN_SYSTEM.md` Design-QA checklist, including the WCAG 2.2 AA contrast gate and visible focus ring in every style
- [ ] Keyboard-operable; focus order correct; `prefers-reduced-motion` respected
- [ ] No horizontal overflow at 320 px, 390 px, 768 px, 1280 px, 1920 px
- [ ] Loading, empty, error and permission-denied states implemented — not just the happy path
- [ ] Production build succeeds; no console errors

---

## 11. Dependency Graph

```
                            L0 Contracts & Foundation
                                       │
        ┌──────────────┬───────────────┼───────────────┬──────────────┐
        │              │               │               │              │
   L1 Data/ETL    L2 Domain       L6 SDKs         L7 Design      L12 Platform Ops
        │              │               │           System              │
        │              ├───────────────┤               │               │
        │              │               │               │               │
        │         L4 Identity     L3 Transports        │               │
        │         & Gateway            │               │               │
        │              │               │               │               │
        └──────┬───────┴───────┬───────┘               │               │
               │               │                       │               │
          L5 Search       (API live)                    │               │
               │               │                       │               │
               └───────┬───────┴───────────┬───────────┴───────────────┘
                       │                   │
              ┌────────┼────────┬──────────┴──────┐
              │        │        │                 │
          L8 Web   L9 Sandbox  L10 Portal    L11 Admin
```

**Critical path:** `L0 → L2 → L3 → L4 → L10/L11 → Launch`.
**Longest independent lane:** `L1 Data/ETL` — start it in Sprint 0 in parallel with L0, because canonical data quality gates the whole product and cannot be compressed late.
**Build-order amendment (Spec §32.7):** `@ghanageo/react` (L6) ships **before** the sandbox (L9), so the sandbox dogfoods the same hooks customers get.

---

## 12. Sprint Plan Overview

| Sprint | Theme | Lanes active | Exit criteria |
|---|---|---|---|
| **0** | Foundation & Contracts | L0, L1, L7, L12 | Monorepo builds; Docker stack up; all three contracts published; design tokens exist; CI green |
| **1** | Data Core | L1, L2, L5, L7 | Collections + validators + `2dsphere` indexes live; seed imports idempotently (16 regions / 261 districts); provenance + dataset versioning working |
| **2** | API Core | L2, L3, L5, L6 | REST v1 live; OpenAPI generated; use cases tested; `@ghanageo/core` + `client` published internally |
| **3** | Transports & Identity | L3, L4, L6 | GraphQL + gRPC live with semantic parity; keys/scopes/quotas/rate limits enforced; audit log writing |
| **4** | Search, SDKs & Design System | L5, L6, L7 | Search/autocomplete/geocode/reverse/nearby meet p95 targets; `@ghanageo/react` published; design system complete in 3 styles × 3 themes |
| **5** | Portals | L9, L10, L11 | Sandbox, developer portal and admin portal functional end-to-end |
| **6** | Marketing, Docs & Launch | L8, L12 + all | Marketing + docs public; observability, backups, DR drill, security review complete |

---

## 13. Epic Catalog

| Epic | Title | Lane | Sprint | Spec ref |
|---|---|---|---|---|
| EP-01 | Monorepo, Local Stack & CI/CD | L0/L12 | 0 | §23, §24 |
| EP-02 | API Contracts (OpenAPI, GraphQL SDL, protobuf) | L0 | 0 | §6–§9 |
| EP-03 | Canonical Schema, Provenance & Dataset Versioning | L1 | 0–1 | §3, §4, §18 |
| EP-04 | Ingestion Adapters & ETL Pipeline | L1 | 1 | §4.1 |
| EP-05 | Matching, Dedupe & Geometry Validation | L1 | 1 | §4.2 |
| EP-06 | Dataset Release Workflow | L1 | 1 | §18, §17 |
| EP-07 | Domain Model & Application Use Cases | L2 | 1–2 | §3, §6 |
| EP-08 | REST API v1 | L3 | 2 | §7 |
| EP-09 | Identity, API Keys, Scopes, Quotas & Audit | L4 | 3 | §12, §13 |
| EP-10 | GraphQL API v1 | L3 | 3 | §8 |
| EP-11 | gRPC API v1 | L3 | 3 | §9 |
| EP-12 | Search, Geocoding & Spatial Behaviour | L5 | 4 | §10 |
| EP-13 | SDK Family (`core`/`client`/`react`/`node`/`data`/`proto`) | L6 | 2–4 | §11, §32 |
| EP-14 | Design System & UI Platform | L7 | 0–4 | `DESIGN_SYSTEM.md` |
| EP-15 | Interactive Sandbox | L9 | 5 | §15 |
| EP-16 | Developer Portal | L10 | 5 | §14 |
| EP-16b | Support & Funding (free-forever model) | L8/L10 | 5–6 | product decision, 2026-08-29 |
| EP-13b | **Public CLI** (`ghanageo`) | L6 | 4 | product decision, 2026-08-29 |
| EP-17 | Admin & Data Steward Portal | L11 | 5 | §17 |
| EP-18 | Marketing Website & Documentation Hub | L8 | 6 | §16 |
| EP-19 | Observability, SLOs & Status | L12 | 6 | §20 |
| EP-20 | Security Hardening & Privacy | L12 | 6 | §12.4, §21 |
| EP-21 | Backups, DR & Runbooks | L12 | 6 | §27 |
| EP-22 | Acceptance, Load & Compatibility Testing | all | 6 | §22 |
| **EP-23** | **SDK Family Expansion** (Python/Go/Dart/Java/C#/PHP) | L6 | **V2.0** | §28, §32.6 |
| **EP-24** | **Enterprise Access** (OAuth2 CC, SSO/SAML, service accounts, private datasets) | L4/L10 | **V2.1** | §28 |
| **EP-25** | **Data Depth** (POI registry, roads, offline datasets, vector tiles, address validation) | L1/L5 | **V2.2** | §28 |
| **EP-26** | **Ecosystem** (webhooks, change feed, contributor reputation, steward programme) | L1/L11 | **V2.3** | §28 |
| **EP-27** | **GhanaPostGPS Adapter** — *gated on a signed licence, no date* | L1 | **gated** | §28, R1 |

Epics **EP-23 – EP-27** are Specification v2 and are detailed in §23. They are listed here so the catalog is complete, **not** so they can be pulled into a V1 sprint.

---

## 14. Sprint 0 — Foundation & Contracts

**Goal:** every downstream lane can start on day one of Sprint 1 without waiting for an implementation.

### EP-01 — Monorepo, Local Stack & CI/CD (L0/L12)

#### GEO-1.1 — Monorepo skeleton
**Depends-On:** — · **Lane:** L0
**User story:** As an agent, I need a repository whose structure matches §4 so I know where my code goes.
**Acceptance criteria:**
- pnpm workspace + Turborepo at root; `go.work` covering `services/api` and `services/worker`.
- Directory tree of §4 exists with placeholder `README.md` in each.
- Spec `.docx` files moved to `docs/spec/`; seed CSVs moved to `data/seed-data/` and renamed `regions.csv`, `districts.csv`, `places.csv`; `GhanaGeo_Initial_Seed_Data.xlsx` to `data/seed-data/source/`.
- `GhanaGeo_React_Hooks_Starter.zip` extracted into `packages/react/` as the starting point for EP-13.
- `packages/config` exports shared `tsconfig`, ESLint flat config, Prettier config, Tailwind v4 preset.
- `pnpm install && pnpm build && pnpm typecheck` succeed on a clean clone.

#### GEO-1.2 — `CLAUDE.md` and `AGENTS.md`
**Depends-On:** GEO-1.1 · **Lane:** L0 · **Rule:** R13
**Acceptance criteria:** Both files exist and cover coding standards, the Jira workflow, the GitHub workflow, AI operating procedures, the lane map (§8), and the non-negotiable rules (§2). `AGENTS.md` links this plan and `DESIGN_SYSTEM.md`.

#### GEO-1.3 — Local Docker stack
**Depends-On:** GEO-1.1 · **Lane:** L12
**Acceptance criteria:** `docker compose up` starts **MongoDB 8.3 as a single-node replica set** (`--replSet rs0`, auto-initiated by an init script — required for multi-document transactions), **Redis 7**, the API and the worker. Healthchecks on all four. `make dev` documented in `README.md`. A `mongo-init.js` creates the application user with a least-privilege role (R9). **Local search runs the Typesense `SearchPort` implementation**, since Atlas Search is not available offline — a documented, tested divergence between local and hosted search.

#### GEO-1.4 — CI pipeline
**Depends-On:** GEO-1.1 · **Lane:** L12
**Acceptance criteria:** GitHub Actions runs, on every PR: Go lint/vet/test, locked TypeScript compiler typecheck/lint/test, `buf lint` + `buf breaking`, OpenAPI diff, GraphQL schema snapshot, migration validation, secret scan, SCA dependency scan, container scan, and an integration job with **MongoDB replica-set** + Redis service containers. Vercel preview deployments configured for the four Next.js apps (Spec §23).

#### GEO-1.5 — Environment matrix & config
**Depends-On:** GEO-1.3 · **Lane:** L12 · **Spec:** §24
**Acceptance criteria:** Local / Test-CI / Sandbox / Staging / Production defined with separate credentials and separate databases. Config loaded from environment only. `.env.example` complete; no real secret committed. Sandbox config asserts a dedicated, capped key class (Spec §15, "Sandbox isolation").

#### GEO-1.6 — ADR register and licensing register
**Depends-On:** GEO-1.1 · **Lane:** L0 · **Rule:** R4
**Acceptance criteria:** `docs/adr/0001-modular-monolith.md`, `0002-postgis-canonical-store.md`, `0003-contracts-first.md`, `0004-tri-morphic-design-system.md` written. `docs/licensing-register.md` seeded with GNHR, GSS, OSM (ODbL), GeoNames (CC BY) and GhanaPostGPS (**blocked — no licence**) rows.

### EP-02 — API Contracts (L0)

#### GEO-2.1 — Protobuf definitions
**Depends-On:** GEO-1.1 · **Spec:** §9.1
**Acceptance criteria:** `proto/ghanageo/v1/` defines `GeographyService` with all eleven RPCs from Spec §9.1 including `StreamDatasetChanges`. `buf lint` clean. Generated Go and TypeScript clients build. Deadlines and max message sizes documented.

#### GEO-2.2 — GraphQL schema
**Depends-On:** GEO-1.1 · **Spec:** §8.2
**Acceptance criteria:** `contracts/graphql/schema.graphql` implements the Spec §8.2 sketch: `GeoJSON`/`DateTime` scalars, `PlaceType` enum, `Region`/`District`/`Place`/`Coordinate` types, all Relay-style connections, and every `Query` field. Pagination mandatory on list fields with a documented max page size.

#### GEO-2.3 — OpenAPI 3.1 contract
**Depends-On:** GEO-1.1 · **Spec:** §7.2
**Acceptance criteria:** `contracts/openapi/v1.yaml` defines all fifteen endpoints of Spec §7.2 with parameters, responses, the error envelope, rate-limit headers and examples. A Prism mock server runs from it so L8/L9/L10 can develop immediately.

#### GEO-2.4 — Error catalog
**Depends-On:** GEO-1.1 · **Spec:** §19
**Acceptance criteria:** `contracts/errors/catalog.yaml` lists every code in Appendix B with its HTTP status, gRPC status, message template and docs URL. A generator emits Go constants, TS types and the public `/docs/errors/{CODE}` pages from it.

#### GEO-2.5 — Contract compatibility CI
**Depends-On:** GEO-2.1–2.4, GEO-1.4 · **Spec:** §18
**Acceptance criteria:** CI fails on an unapproved breaking change in any of the three contracts. The `contract-break-approved` label is the only override and requires the L0 owner's review.

### EP-03 (start) — Canonical Schema (L1)

#### GEO-3.1 — Core schema migration
**Depends-On:** GEO-1.3 · **Spec:** §3.1
**Acceptance criteria:**
- Collections `regions`, `districts`, `places`, `place_aliases`, `roads`, `pois`, `postal_areas`, `sources`, `source_records`, `dataset_versions`, `change_requests` created with the fields of Spec §3.1.
- **`_id` is a ULID string** (R7) — not an auto-generated `ObjectId`, because IDs must be stable and reproducible across reimports.
- **Every collection has a JSON Schema validator** with `validationLevel: "strict"` and `validationAction: "error"` (R16). Mongo will not silently accept a malformed document.
- Geometry stored as **GeoJSON objects** (`{type, coordinates}`, WGS84 / EPSG:4326 — the only CRS MongoDB accepts) with **`2dsphere` indexes** on `centroid` and `geometry`.
- Every document carries embedded provenance (`sourceId`, `externalId`, `retrievedAt`, `sourcePayloadHash`) and `datasetVersion` (R2).
- **Denormalization is deliberate and documented:** `places` embeds `regionName` and `districtName` for read performance, with a reconciliation job that repairs them after a rename. Aliases are embedded in `places` (bounded, always read together); roads and POIs are separate collections (unbounded).
- Compound indexes for the query patterns in Spec §7.2, including `{districtId: 1, type: 1, normalizedName: 1}` and `{normalizedName: "text"}`.

#### GEO-3.2 — Place-type and status enumerations
**Depends-On:** GEO-3.1 · **Spec:** §3**
**Acceptance criteria:** `place_type` covers CITY, TOWN, VILLAGE, COMMUNITY, SUBURB, NEIGHBOURHOOD, HAMLET, SETTLEMENT, LOCALITY, REGIONAL_CAPITAL. `status` covers ACTIVE, DEPRECATED, MERGED. `verification_status` covers REFERENCE, SEED_NEEDS_CANONICAL_RECONCILIATION, REVIEWED, CANONICAL — matching the values present in the seed CSVs.

#### GEO-3.3 — Merge lineage & redirects
**Depends-On:** GEO-3.1 · **Spec:** §4.2, §18
**Acceptance criteria:** `place_redirects(old_id, new_id, reason, merged_at)`. Resolving a deprecated ID returns the successor with `mergedInto` metadata rather than 404. Old IDs continue resolving for a documented period.

### EP-14 (start) — Design System foundations (L7)

#### GEO-14.1 — Token architecture
**Depends-On:** GEO-1.1 · **Spec:** `DESIGN_SYSTEM.md` §Tokens
**Acceptance criteria:** `packages/ui/src/tokens/` implements the three-layer token system (primitive → semantic → component) with the **style axis** (`neu` | `glass` | `clay`), the **mode axis** (`light` | `dark`) and the **custom brand-hue axis** as described in `DESIGN_SYSTEM.md`. Only the documented surface-token set differs between styles. Tailwind v4 `@theme` reads the CSS variables.

#### GEO-14.2 — Theme provider & SSR flash prevention
**Depends-On:** GEO-14.1
**Acceptance criteria:** `ThemeProvider` resolves style + mode + brand hue, persists to `localStorage` and (when signed in) to the user's server-side profile. A blocking inline script sets `data-style`, `data-mode` and the brand custom properties on `<html>` before first paint — zero flash of wrong theme. `prefers-color-scheme` is the default when the user has expressed no preference.

---

## 15. Sprint 1 — Data Core

**Goal:** canonical Ghanaian geography exists in MongoDB with provenance, reproducibly, from the seed.

### EP-03 (complete) / EP-04 — Ingestion & ETL (L1)

#### GEO-4.1 — Seed import command
**Depends-On:** GEO-3.1 · **Spec:** §31.1 · **Appendix E**
**Acceptance criteria:**
- `ghanageo data seed --file data/seed-data/manifest.json --environment local` imports 16 regions, 261 districts and 16 places.
- **Idempotent (R8):** running it twice produces zero net changes; a CI test asserts this.
- `manifest.json` carries dataset version, per-file checksums, `generated_at` and source revisions.
- CI asserts exactly **16 seed regions** and **261 unique district/MMDA seed rows** (Spec §31.1).
- Records land with `verification_status = SEED_NEEDS_CANONICAL_RECONCILIATION` where the CSV says so, and are **not** promoted automatically (R5).

#### GEO-4.2 — `data validate` and `data reconcile` commands
**Depends-On:** GEO-4.1 · **Spec:** §31.1
**Acceptance criteria:** `ghanageo data validate --dataset seed` runs every §22.1 domain check and exits non-zero on failure. `ghanageo data reconcile --against canonical-staging` reports proposed changes without applying them; applying requires a reviewed change request.

#### GEO-4.3 — Source registry & raw landing
**Depends-On:** GEO-3.1 · **Spec:** §4, §4.1 · **Rule:** R2
**Acceptance criteria:** `sources` seeded from the licensing register. Every fetch writes a content-addressed raw record (`source_payload_hash`) retained so a release is reproducible. Raw landing is append-only.

#### GEO-4.4 — Ingestion port + fixture adapter
**Depends-On:** GEO-4.3 · **Spec:** §4.1
**Acceptance criteria:** `IngestionAdapter` interface with `Fetch → Validate → Normalize` stages. A fixture adapter exercises the whole pipeline in CI with no network. The pipeline stages of Spec §4.1 are each independently testable.

#### GEO-4.5 — GNHR adapter (staging only)
**Depends-On:** GEO-4.4 · **Spec:** §4, §31 · **Rule:** R3
**Acceptance criteria:** Ingests the region → district → community hierarchy into **staging only**. **Strips focal-person, contact and any household/personal fields before the raw landing write**; a test feeds a payload containing PII and asserts none persists. Data is not published without licence confirmation in the register.

#### GEO-4.6 — GSS boundary adapter
**Depends-On:** GEO-4.4 · **Spec:** §4
**Acceptance criteria:** Ingests district boundaries/codes from official Ghana Statistical Service files. Source and version metadata preserved. Geometry passes validation (GEO-5.3).

#### GEO-4.7 — OpenStreetMap Ghana adapter
**Depends-On:** GEO-4.4 · **Spec:** §4 · **Rule:** R4
**Acceptance criteria:** Ingests settlements, suburbs, neighbourhoods, roads and POIs from the Geofabrik Ghana extract. **ODbL attribution** attached to every derived record and surfaced in API responses and downloads.

#### GEO-4.8 — GeoNames cross-check adapter
**Depends-On:** GEO-4.4 · **Spec:** §4
**Acceptance criteria:** Used for names/aliases cross-checking only. **CC BY attribution** recorded. Conflicts raise review-queue items rather than overwriting (R8).

### EP-05 — Matching, Dedupe & Geometry (L1)

#### GEO-5.1 — Normalization rules
**Depends-On:** GEO-4.4 · **Spec:** §4.2, §10
**Acceptance criteria:** Deterministic normalization of case, punctuation, whitespace, diacritics and common Ghanaian abbreviations (`comm` → `community`, `Rd` → `road`, `Adenta`/`Adentan`). Same input always yields the same `normalized_name`. Golden-file tests.

#### GEO-5.2 — Matching & dedupe engine
**Depends-On:** GEO-5.1 · **Spec:** §4.2
**Acceptance criteria:** Combines geographic distance, parent district/region, name similarity and source confidence. **Never auto-merges two same-named places in different districts** — that case always goes to the review queue. Merge lineage and redirects recorded (GEO-3.3). Fixtures cover the same-name-different-district case explicitly (Spec §22.1).

#### GEO-5.3 — Geometry validation
**Depends-On:** GEO-3.1 · **Spec:** §4.2, §22.1
**Acceptance criteria:** Rejects or flags invalid polygons, self-intersections, impossible coordinates, coordinates outside Ghana's bounding box, and parent-child containment violations. Repairs happen only through reviewed ETL, never silently.
**Technical note — this story carries extra weight under MongoDB.** PostGIS would have supplied `ST_IsValid` and `ST_MakeValid`; MongoDB has no equivalent. It rejects some malformed GeoJSON on insert but offers **no validity checking or repair tooling**, and a self-intersecting polygon that Mongo accepts will silently return wrong `$geoIntersects` results. Therefore validation is implemented **in the Go ETL layer** using a geometry library (`github.com/twpayne/go-geom` + `orb`), and is a **blocking gate before any write to a canonical collection** — not an advisory check. A test suite of known-bad polygons (self-intersecting, unclosed ring, wrong winding order, antimeridian-crossing) asserts each is rejected with a specific error code. Winding order is normalized to right-hand-rule on ingest, because MongoDB interprets large polygons by winding order.

#### GEO-5.4 — Quality scoring & review queue
**Depends-On:** GEO-5.2, GEO-5.3 · **Spec:** §4.1
**Acceptance criteria:** Each record carries a quality score with contributing factors. Anything below threshold or flagged by matching enters the review queue consumed by the admin portal (EP-17).

### EP-06 — Dataset Release Workflow (L1)

#### GEO-6.1 — Dataset version model
**Depends-On:** GEO-3.1 · **Spec:** §18
**Acceptance criteria:** Version format `YYYY.MM.patch` (e.g. `2026.08.1`), independent of API major versions. States: `draft → validation → review → approved → published → rolled_back`. Checksums and changelog per version.

#### GEO-6.2 — Dataset version in every response
**Depends-On:** GEO-6.1 · **Spec:** §18
**Acceptance criteria:** Every REST body, GraphQL result and gRPC response carries the dataset version. A cross-protocol test asserts all three report the same version for the same fixture.

#### GEO-6.3 — Release, rollback & changelog
**Depends-On:** GEO-6.1 · **Spec:** §18, §23
**Acceptance criteria:** Publishing is a separate operation from code deploy and is independently rollback-able (R10). The changelog records additions, merges, renames, boundary changes and source changes. Rollback is tested.

#### GEO-6.4 — Bulk export pipeline
**Depends-On:** GEO-6.3 · **Spec:** §1.1, §7.2
**Acceptance criteria:** Worker produces versioned JSON, CSV, GeoJSON and Parquet exports with checksums, attribution files and a licence notice. `/datasets` and `/datasets/{version}/downloads` serve their metadata.

### EP-07 (start) — Domain & Use Cases (L2)

#### GEO-7.1 — Domain entities
**Depends-On:** GEO-2.1–2.3 · **Spec:** §3
**Acceptance criteria:** `Region`, `District`, `Place`, `PlaceAlias`, `Road`, `POI`, `Boundary` as pure Go with invariants enforced in constructors. Zero external dependencies beyond a ULID package. Administrative and human geography modelled **separately** (Spec §3) — a place's district is not its only parent.

#### GEO-7.2 — Repository ports
**Depends-On:** GEO-7.1
**Acceptance criteria:** Interfaces in `internal/ports` for every aggregate. Cursor pagination in the port signature, not bolted on at the transport.

#### GEO-7.3 — MongoDB repository adapters
**Depends-On:** GEO-7.2, GEO-3.1
**Acceptance criteria:** Filters built from typed structs, never raw maps from request data (R9). BSON tags live here, never in `domain` (§5). Integration-tested with a testcontainers MongoDB replica set. Spatial queries use **`$geoIntersects`** for boundary containment and **`$nearSphere` with `$maxDistance`** for proximity (Spec §10). Multi-document writes that must be atomic (merge + redirect, release publish) use **transactions**, which is why the replica set is mandatory even locally.

---

## 16. Sprint 2 — API Core

### EP-07 (complete) — Use Cases (L2)

#### GEO-7.4 — Geography query use cases
**Depends-On:** GEO-7.3 · **Spec:** §6
**Acceptance criteria:** `ListRegions`, `GetRegion`, `ListDistricts`, `GetDistrict`, `ListPlacesInDistrict`, `GetPlace`, `GetBoundary` implemented once and shared by all three transports (R6). Exhaustive unit tests with fake repositories plus integration tests.

#### GEO-7.5 — Pagination, filtering and determinism
**Depends-On:** GEO-7.4 · **Spec:** §22.2
**Acceptance criteria:** Cursor pagination with a stable sort key; identical results for identical inputs; documented max page size; empty results are a normal `200`/empty connection, never an error.

### EP-08 — REST API v1 (L3)

#### GEO-8.1 — REST transport skeleton
**Depends-On:** GEO-2.3, GEO-7.4 · **Spec:** §7.1
**Acceptance criteria:** `chi` router at `/v1`. Handlers only marshal, validate wire format, call a use case, and map the result (R6). Request ID, panic recovery and CORS middleware wired.

#### GEO-8.2 — Core geography endpoints
**Depends-On:** GEO-8.1 · **Spec:** §7.2
**Acceptance criteria:** `GET /regions`, `/regions/{id}`, `/regions/{id}/districts`, `/districts`, `/districts/{id}/places`, `/places/{id}`, `/boundaries/{id}` implemented and matching `contracts/openapi/v1.yaml` exactly.

#### GEO-8.3 — Dataset endpoints
**Depends-On:** GEO-8.1, GEO-6.4 · **Spec:** §7.2
**Acceptance criteria:** `GET /datasets` and `GET /datasets/{version}/downloads` return published versions with checksums, formats, sizes and attribution.

#### GEO-8.4 — OpenAPI generation & drift check
**Depends-On:** GEO-8.2 · **Spec:** §22.2
**Acceptance criteria:** OpenAPI generated from the running server and diffed against `contracts/openapi/v1.yaml` in CI. Drift fails the build. A generated client smoke test runs against the live test server.

#### GEO-8.5 — Error envelope
**Depends-On:** GEO-8.1, GEO-2.4 · **Spec:** §19
**Acceptance criteria:** Every error returns `{"error":{"code","message","requestId","details","docs"}}` with the HTTP status mapped from the catalog. Invalid coordinates, radius, IDs and oversized inputs return stable, documented codes.

### EP-13 (start) — SDK Family (L6)

#### GEO-13.1 — `@ghanageo/core`
**Depends-On:** GEO-2.3 · **Spec:** §32.6
**Acceptance criteria:** Canonical TypeScript types and utilities shared by every other package. ESM, tree-shakeable, no runtime dependencies. Types generated from the OpenAPI contract so they cannot drift.

#### GEO-13.2 — `@ghanageo/client`
**Depends-On:** GEO-13.1 · **Spec:** §32.6
**Acceptance criteria:** Framework-agnostic REST + GraphQL client. `AbortSignal` propagation. Errors map to a typed `GhanaGeoError` carrying `code`, `status` and `requestId`. Builds on the class already present in the starter zip (`packages/react/src/client.ts`), extracted into this package.

---

## 17. Sprint 3 — Transports & Identity

### EP-09 — Identity, Keys, Quotas & Audit (L4)

#### GEO-9.1 — Request ID & context propagation
**Depends-On:** GEO-8.1 · **Spec:** §19, §20
**Acceptance criteria:** Request ID generated at the edge, carried through logs, traces and every error, returned on every response across all three protocols.

#### GEO-9.2 — Account authentication
**Depends-On:** GEO-1.3 · **Spec:** §12.1
**Acceptance criteria:** Email/password with verified email; **passkeys/WebAuthn preferred** with TOTP recovery; short-lived sessions with rotation and global revocation; **MFA mandatory for administrators and privileged data stewards**. Session fixation, logout and MFA-recovery flows tested (Spec §22.3).

#### GEO-9.3 — Organization / Application / Key model
**Depends-On:** GEO-9.2 · **Spec:** §12.2
**Acceptance criteria:**
- Hierarchy `Developer → Organization → Application → Keys`.
- Key format `gh_test_<prefix>_<secret>` / `gh_live_<prefix>_<secret>`.
- **Secret shown exactly once.** Storage keeps a non-secret prefix plus a strong one-way digest.
- Rotation, revocation, expiry, last-used metadata, allowed origins and IP allow-lists.
- Keys are labelled **Browser**, **Server** or **Test**; unsafe scope combinations for Browser keys are rejected (Spec §32.5).

#### GEO-9.4 — Scope enforcement
**Depends-On:** GEO-9.3 · **Spec:** §12.3 · **Appendix A**
**Acceptance criteria:** Every operation declares its required scope. Denial tests exist per endpoint and per gRPC method. Expired and revoked keys are rejected immediately (Spec §22.3).

#### GEO-9.5 — Rate limiting
**Depends-On:** GEO-9.3 · **Spec:** §13
**Acceptance criteria:** Redis GCRA with hierarchical enforcement IP → account → org → application → key → endpoint cost class. Standard limit headers on REST and equivalent metadata on GraphQL/gRPC. `429` includes retry guidance. Burst limits separate from monthly quotas. **Sandbox limits are strictly tighter than authenticated production.**

#### GEO-9.6 — Quota & cost accounting
**Depends-On:** GEO-9.5 · **Spec:** §13 · **Appendix D**
**Acceptance criteria:** Cost classes charged per Appendix D. GraphQL cost is computed from the requested field set, gRPC streams account for connection plus messages. Admin can suspend an abusive key instantly.

#### GEO-9.7 — Audit log
**Depends-On:** GEO-9.2 · **Spec:** §12.4, §17
**Acceptance criteria:** Immutable append-only audit rows for every privileged action: actor, action, target, before/after, request ID, timestamp, IP. No delete path exists in the API.

#### GEO-9.8 — Security event alerting
**Depends-On:** GEO-9.5, GEO-9.7 · **Spec:** §12.4
**Acceptance criteria:** Alerts on unusual key usage, quota spikes and failed admin authentication.

### EP-10 — GraphQL API v1 (L3)

#### GEO-10.1 — GraphQL server from shared use cases
**Depends-On:** GEO-2.2, GEO-7.4 · **Spec:** §8
**Acceptance criteria:** `gqlgen` server at `POST /graphql`. Resolvers call the same use cases as REST (R6). Schema matches `contracts/graphql/schema.graphql` exactly.

#### GEO-10.2 — DataLoader batching
**Depends-On:** GEO-10.1 · **Spec:** §8.4
**Acceptance criteria:** Per-request DataLoaders for region, district and place. A nested-query test asserts an N+1 pattern collapses to the expected small query count.

#### GEO-10.3 — Depth, complexity & cost limits
**Depends-On:** GEO-10.1 · **Spec:** §8.4, §13
**Acceptance criteria:** Max depth and a weighted complexity budget per request. Per-field cost multipliers for geometry and spatial fields. Query timeout plus a database statement timeout. Depth/complexity attack tests reject as expected (Spec §22.3).

#### GEO-10.4 — Persisted queries & introspection policy
**Depends-On:** GEO-10.1 · **Spec:** §8.4
**Acceptance criteria:** Persisted-query support for production clients with optional enterprise allow-listing. Introspection available in sandbox/docs; restricted for anonymous production traffic by configuration.

#### GEO-10.5 — GraphQL error mapping
**Depends-On:** GEO-10.1, GEO-2.4 · **Spec:** §19
**Acceptance criteria:** `errors[].extensions.code` plus `requestId`. Partial-data semantics preserved.

### EP-11 — gRPC API v1 (L3)

#### GEO-11.1 — gRPC service implementation
**Depends-On:** GEO-2.1, GEO-7.4 · **Spec:** §9
**Acceptance criteria:** All eleven RPCs implemented over the shared use cases. Deadlines and max message sizes enforced; unbounded requests rejected. Canonical gRPC status codes with `google.rpc`-style details.

#### GEO-11.2 — gRPC auth & quota parity
**Depends-On:** GEO-11.1, GEO-9.4 · **Spec:** §9.2
**Acceptance criteria:** API keys / bearer tokens read from request metadata. The same scope, quota and rate-limit accounting as REST and GraphQL.

#### GEO-11.3 — `StreamDatasetChanges`
**Depends-On:** GEO-11.1, GEO-6.3 · **Spec:** §9.2
**Acceptance criteria:** Server streaming of dataset change events with connection plus per-message quota accounting and a documented reconnect/resume story.

#### GEO-11.4 — ConnectRPC / gRPC-Web gateway
**Depends-On:** GEO-11.1 · **Spec:** §9, §15
**Acceptance criteria:** Browser-reachable ConnectRPC endpoint restricted to allow-listed safe operations, used by the sandbox playground. Never routes anonymous traffic through privileged credentials (Spec §15).

#### GEO-11.5 — Generated client smoke tests
**Depends-On:** GEO-11.1 · **Spec:** §22.2
**Acceptance criteria:** Generated Go and TypeScript clients round-trip against the test server in CI. `buf breaking` gates every proto change.

---

## 18. Sprint 4 — Search, SDKs & Design System

### EP-12 — Search, Geocoding & Spatial (L5)

#### GEO-12.1 — Search index & ranking
**Depends-On:** GEO-5.1, GEO-7.3 · **Spec:** §10
**Acceptance criteria:** Exact, prefix and fuzzy matching over names **and aliases** via an **Atlas Search index** using the `autocomplete` operator (edgeGram) and `text` with `fuzzy: {maxEdits: 2}`, combined in a `compound` query with `should` clauses for scoring. Contextual ranking by supplied district/region; coordinate proximity boosting; typo tolerance; abbreviation normalization. Each result carries a **confidence score and a match explanation**. Ambiguity returns multiple candidates rather than guessing.
**Golden cases (Spec §10):** `"tema comm 25"` → *Tema Community 25*; `"kwabena"` → candidates including *Kwabenya*; `"circle"` → locality/POI candidates ranked with Accra context.

#### GEO-12.2 — Autocomplete
**Depends-On:** GEO-12.1 · **Spec:** §7.2, §22.4
**Acceptance criteria:** `GET /autocomplete?q=` meets **p95 < 200 ms**. Minimum query length enforced server-side. Results ordered by prefix match then popularity then proximity.

#### GEO-12.3 — Geocode
**Depends-On:** GEO-12.1 · **Spec:** §7.2
**Acceptance criteria:** `GET /geocode?q=` returns ranked candidates with confidence and match reason. Never returns a single guess for a genuinely ambiguous query.

#### GEO-12.4 — Reverse geocode
**Depends-On:** GEO-7.3, GEO-5.3 · **Spec:** §10, §22.4
**Acceptance criteria:** `GET /reverse?lat=&lng=` uses a **`$geoIntersects`** query against district and region `geometry` to return the containing area, plus `$nearSphere` localities. **p95 < 400 ms.** Coordinates outside Ghana return a documented, non-error empty result.

#### GEO-12.5 — Nearby
**Depends-On:** GEO-7.3 · **Spec:** §10, §22.4
**Acceptance criteria:** `GET /nearby?lat=&lng=&radius=` via `geography` distance. **p95 < 500 ms.** Radius bounded and validated; oversized radius returns a stable error.

#### GEO-12.6 — Performance indexes & p95 gate
**Depends-On:** GEO-12.1–12.5 · **Spec:** §22.4
**Acceptance criteria:** `2dsphere`, compound and Atlas Search indexes tuned; every hot query verified with `explain("executionStats")` to confirm an `IXSCAN` (never a `COLLSCAN`) and that `totalDocsExamined` is close to `nReturned`. A CI load test asserts every target in Spec §22.4: ID lookup < 150 ms, autocomplete < 200 ms, search < 350 ms, reverse < 400 ms, nearby < 500 ms, ordinary GraphQL < 500 ms — all p95 under normal load.

### EP-13 (complete) — SDKs (L6)

#### GEO-13.3 — `@ghanageo/react`
**Depends-On:** GEO-13.2, GEO-12.5 · **Spec:** §32
**Acceptance criteria:**
- All twelve hooks of Spec §32.1: `useRegions`, `useRegion`, `useDistricts`, `useDistrict`, `usePlace`, `useSearch`, `useAutocomplete`, `useGeocode`, `useReverseGeocode`, `useNearby`, `useGhanaGeoGraphQL`, `useGhanaGeoClient`.
- REST by default (SSR-safe); a generic typed GraphQL hook alongside.
- TanStack Query v5 peer dependency; the exported `ghanaGeoKeys` factory (already in the starter) is stable and public.
- **Acceptance tests of Spec §32.7:** provider reuses clients across renders; `useRegions` dedupes concurrent requests; `useSearch` does not fire below the minimum query length; stale input cancels in-flight queries; 401/403/429 map to typed `GhanaGeoError`; GraphQL errors surface `extensions.code`/`requestId`; query keys never cross-contaminate; the Next.js 16 SSR hydration example works; **the bundle contains no embedded default secret key.**
- Hook tests use MSW + React Testing Library.

#### GEO-13.4 — `@ghanageo/node`
**Depends-On:** GEO-13.2 · **Spec:** §32.6
**Acceptance criteria:** Server-focused helpers; server keys never leak to a client bundle; documented Next.js server/client key split.

#### GEO-13.5 — `@ghanageo/data` (offline)
**Depends-On:** GEO-6.4 · **Spec:** §11
**Acceptance criteria:** Offline `getRegions`, `getDistricts`, `search`, `getPlace` with **no hidden network calls**. Large geometry/POI assets are optional subpackages or lazy downloads. Dataset version exposed programmatically and versioned separately from the code (SemVer for code, `YYYY.MM.patch` for data).

#### GEO-13.6 — `ghanageo` umbrella npm package
**Depends-On:** GEO-13.5 · **Spec:** §11
**Acceptance criteria:** The public `ghanageo` package exposing the Spec §11 API surface, ESM-first with documented CJS compatibility, tree-shakeable, TypeScript-native.

#### GEO-13.8 — Public CLI (`ghanageo`)
**Depends-On:** GEO-8.2, GEO-12.1 · **Lane:** L6 · **Module:** `cli/`
**User story:** As a Ghanaian developer, journalist, researcher or student, I want to query national geography from my terminal without signing up for anything.
**Business value:** A CLI is the cheapest possible on-ramp for a free public dataset. It needs no account, no browser and no SDK, and it makes the data usable by people who are not building an application at all.
**Acceptance criteria:**
- Commands: `regions`, `districts`, `places`, `search`, `suggest`, `nearby`, `reverse`, `version`, `help`.
- **No API key required.** Anonymous use is a first-class path, not a degraded one (§24 F1). `GHANAGEO_API_KEY` is accepted for fair-use identification and unlocks nothing.
- Three output modes: an aligned table for humans, `--json` for `jq`, `--csv` for spreadsheets.
- Table alignment is measured in **runes, not bytes**, so Twi, Ga and Ewe names (`Ɔsu`, `Kwabɛnya`) do not break columns.
- Colour is suppressed when output is not a terminal, and `NO_COLOR` is honoured.
- Distinct exit codes: `0` success, `1` local error, `2` API error. An API error prints the stable code and a docs link.
- Static binaries for darwin/linux/windows × amd64/arm64 plus linux/arm, with `checksums.txt`. No runtime dependency.
- Distributed three ways: `go install`, `npx ghanageo`, and Homebrew.
- The npm wrapper forwards signals and reproduces the child's exit code, and fails with actionable advice on an unsupported platform.

#### GEO-13.9 — CLI distribution and release automation
**Depends-On:** GEO-13.8 · **Lane:** L6/L12
**Acceptance criteria:** One tagged release cross-compiles every platform, publishes the npm package with the binaries, updates the Homebrew tap, and attaches checksums. The CLI reports the API version it targets.

#### GEO-13.7 — `@ghanageo/proto`
**Depends-On:** GEO-11.5 · **Spec:** §32.6
**Acceptance criteria:** Published protobuf definitions and generated TS clients, versioned with the proto package.

### EP-14 (complete) — Design System (L7)

#### GEO-14.3 — Tri-morphic primitive library
**Depends-On:** GEO-14.2 · **Spec:** `DESIGN_SYSTEM.md`
**Acceptance criteria:** Button, Input, Select, Checkbox, Radio, Switch, Card, Badge, Tabs, Table, Dialog, Sheet, Drawer, Popover, Dropdown, Tooltip, Toast, Command palette, Skeleton, Pagination, Breadcrumb, Avatar, Progress, Alert, Combobox — built on Radix, reading **only** design tokens. Each renders correctly in all three styles × light/dark/custom without conditional style logic in the component.

#### GEO-14.4 — Motion system
**Depends-On:** GEO-14.3 · **Spec:** `DESIGN_SYSTEM.md` §Motion
**Acceptance criteria:** Duration/easing token scale, text transitions, in-page transitions and route/layout transitions implemented as documented. `prefers-reduced-motion` honoured throughout. No animation of layout-affecting properties on hover.

#### GEO-14.5 — App shell
**Depends-On:** GEO-14.3 · **Spec:** `DESIGN_SYSTEM.md` §Shell
**Acceptance criteria:** `AppShell` with sidebar, navbar, command palette and content region, consumed by `apps/portal` and `apps/admin`. Navigation is data-driven from a config array, role-filtered, with the collapse/pin/persist behaviour specified.

#### GEO-14.6 — Theme picker
**Depends-On:** GEO-14.2 · **Spec:** `DESIGN_SYSTEM.md` §Theme Picker
**Acceptance criteria:** One component, mounted in **admin settings, portal settings and the marketing site footer**. Selects style, mode and a custom brand hue with a live preview. Persists for anonymous visitors locally and for signed-in users server-side. Contrast is auto-corrected when a user picks an inaccessible hue.

#### GEO-14.7 — Design-QA gate
**Depends-On:** GEO-14.3–14.6
**Acceptance criteria:** `design-qa.md` checklist plus automated axe-core and Playwright visual checks across the 3 × 3 style/theme matrix, wired into CI as a required check for frontend PRs.

---

## 19. Sprint 5 — Portals (Developer, Sandbox, Admin)

All three apps consume `packages/ui` and build to `DESIGN_SYSTEM.md`. None re-implements shell chrome.

### EP-15 — Interactive Sandbox (L9)

The sandbox is the primary acquisition tool (Spec §15). It must work **without** production credentials.

#### GEO-15.1 — Sandbox isolation & ephemeral keys
**Depends-On:** GEO-9.5 · **Spec:** §15
**Acceptance criteria:** A dedicated sandbox key class with a capped dataset and aggressive limits. Anonymous users are **never** routed through privileged production credentials. Session keys are short-lived and abuse-controlled.

#### GEO-15.2 — REST request builder
**Depends-On:** GEO-15.1, GEO-13.3 · **Spec:** §15
**Acceptance criteria:** Endpoint/param/header builder with live response, timing, cost class and generated **curl / JavaScript / Go** snippets. Built on `@ghanageo/react` — the sandbox dogfoods the customer SDK (Spec §32.7 build-order amendment).

#### GEO-15.3 — GraphQL explorer
**Depends-On:** GEO-15.1, GEO-10.4 · **Spec:** §15
**Acceptance criteria:** Schema docs, autocomplete, variables editor, query history and a live **complexity indicator** that warns before the budget is exceeded.

#### GEO-15.4 — gRPC playground
**Depends-On:** GEO-11.4 · **Spec:** §15
**Acceptance criteria:** ConnectRPC-backed playground executing only allow-listed operations, with a method picker, typed request form and streaming output view for `StreamDatasetChanges`.

#### GEO-15.5 — Map visualisation
**Depends-On:** GEO-15.2, GEO-14.3 · **Spec:** §15
**Acceptance criteria:** **Leaflet** view (OpenStreetMap raster tiles, **no API key**, ODbL attribution control always visible) rendering geocode, reverse, nearby and boundary responses as GeoJSON layers. Map panels follow the map-overlay rules in `DESIGN_SYSTEM.md` (legibility over imagery in all three styles).

#### GEO-15.6 — Sample queries & conversion
**Depends-On:** GEO-15.2 · **Spec:** §15
**Acceptance criteria:** One-click samples: **Osu**, **Adenta**, **Tema Community 25**, **Kumasi**. A "Create developer account" CTA that **preserves the current sandbox request** through signup and replays it in the portal.

### EP-16 — Developer Portal (L10)

Spec §14 defines nine areas; each is a story.

#### GEO-16.1 — Portal shell & home
**Depends-On:** GEO-14.5, GEO-9.2 · **Spec:** §14
**Acceptance criteria:** Quick start, usage summary, current incidents and recent changelog. Uses the shared `AppShell`.

#### GEO-16.2 — Organizations
**Depends-On:** GEO-9.3 · **Spec:** §14
**Acceptance criteria:** Members, roles, invitations, ownership transfer.

#### GEO-16.3 — Applications
**Depends-On:** GEO-9.3 · **Spec:** §14
**Acceptance criteria:** Test/live environments, domains, callback metadata, plan.

#### GEO-16.4 — API keys
**Depends-On:** GEO-9.3 · **Spec:** §14, §12.2, §32.5
**Acceptance criteria:** Create, rotate, revoke, scope, expiry, IP/origin restrictions. **Secret revealed once**, with an explicit copy-and-confirm step. Keys labelled **Browser / Server / Test**; the UI blocks unsafe scope combinations on Browser keys and explains why.

#### GEO-16.5 — Usage analytics
**Depends-On:** GEO-9.6 · **Spec:** §14
**Acceptance criteria:** Requests, quota consumption, latency, errors, split by protocol, endpoint and geography. Recharts, themed from tokens.

#### GEO-16.6 — Request logs
**Depends-On:** GEO-20.2 · **Spec:** §14, §21
**Acceptance criteria:** Time, request ID, protocol, operation, status, latency. **Secrets and authorization headers redacted.** Short retention by default; enterprise retention configurable.

#### GEO-16.7 — Documentation surfaces
**Depends-On:** GEO-8.4, GEO-2.2, GEO-2.1 · **Spec:** §14
**Acceptance criteria:** Rendered OpenAPI reference, GraphQL schema explorer, protobuf/gRPC docs and SDK examples for REST, GraphQL, gRPC and npm.

#### GEO-16.8 — Security settings
**Depends-On:** GEO-9.2, GEO-9.7 · **Spec:** §14
**Acceptance criteria:** Passkeys/MFA management, active sessions with revocation, audit history, security contacts.

#### GEO-16.9 — Support & sponsorship surface
**Depends-On:** GEO-16.3 · **Supersedes** the billing scaffold in Spec §14 · **See §24**
**Acceptance criteria:**
- **No billing.** There is no plan upgrade, no invoice, no payment method, no card on file, and no code path that could ever charge a developer. The API is free for every consumer.
- The portal shows a **Support GhanaGeo** panel: what it costs to run, what donations fund next, and a donate action.
- Organizations may optionally record a **sponsor** profile (name, logo, link) for public recognition on the marketing site. Sponsorship buys recognition and nothing else — **never** a higher rate limit, priority support, or any access an unsponsored developer lacks. A test asserts sponsorship has zero effect on quota resolution.
- Fair-use limits remain and are identical for everyone (Appendix D). They exist to stop one consumer degrading the service for others, not to sell a way around themselves.

### EP-17 — Admin & Data Steward Portal (L11)

Spec §17. The IA, shell, sidebar, navbar, dropdowns and per-screen chrome are specified in `DESIGN_SYSTEM.md` §Admin Shell — build to it exactly.

#### GEO-17.1 — Admin shell & RBAC gating
**Depends-On:** GEO-14.5, GEO-9.2 · **Spec:** §17.1
**Acceptance criteria:** Shared `AppShell` with the admin navigation registry (`apps/admin/src/config/navigation.ts`, owned centrally by L11 per §8). Six roles gate visibility **and** the API enforces them independently — hiding a nav item is never the only control. **MFA mandatory** for every admin role (Spec §12.1). RBAC denial tests exist for every privileged mutation (Spec §22.3).

#### GEO-17.2 — Map-first location explorer
**Depends-On:** GEO-12.4, GEO-14.5 · **Spec:** §17
**Acceptance criteria:** Three-pane layout — hierarchy tree, **Leaflet** map (GeoJSON boundary layers, `leaflet.markercluster` for dense place markers), inspector panel — with filters, synchronized selection, and deep-linkable state. Follows the map-overlay legibility rules in `DESIGN_SYSTEM.md`.

#### GEO-17.3 — Geography CRUD
**Depends-On:** GEO-7.4, GEO-17.1 · **Spec:** §17
**Acceptance criteria:** Create/edit/deprecate regions, districts, places, aliases, roads and POIs, permission-scoped. Every mutation writes an audit row (GEO-9.7). Deprecation creates a redirect, never a hard delete (GEO-3.3).

#### GEO-17.4 — Geometry editor
**Depends-On:** GEO-5.3, GEO-17.2 · **Spec:** §17
**Acceptance criteria:** Draw/edit polygons with **validation before publish** — self-intersection, containment and coordinate-range checks block the save with an explanatory error.

#### GEO-17.5 — Import jobs & source runs
**Depends-On:** GEO-4.4 · **Spec:** §17
**Acceptance criteria:** Job list with status, duration, records processed, errors, reconciliation conflicts and duplicate candidates. Drill-down to the raw source record and its payload hash.

#### GEO-17.6 — Change-request moderation
**Depends-On:** GEO-5.4 · **Spec:** §17, §3.1
**Acceptance criteria:** Queue with proposed patch, **evidence**, submitter, reviewer comments and a side-by-side **diff view**. Approve / reject / request-changes transitions, all audited.

#### GEO-17.7 — Dataset release workflow UI
**Depends-On:** GEO-6.3 · **Spec:** §17
**Acceptance criteria:** `draft → validation → review → approved → published → rollback` with per-stage gates, validation report, changelog editor and a rollback action carrying a confirmation that names the version being reverted.

#### GEO-17.8 — Developer & organization management
**Depends-On:** GEO-9.3 · **Spec:** §17
**Acceptance criteria:** Search organizations/applications/keys; suspend a key instantly; view usage. Developer Support role reaches this without any canonical-edit permission.

#### GEO-17.9 — Fair-use limit management
**Depends-On:** GEO-9.6 · **Spec:** §17 · **See §24**
**Acceptance criteria:** Edit the default fair-use limits and per-cost-class ceilings. Changes take effect without a deploy and are audited. A steward can raise a limit for a specific application **on documented need** — a research project, a government integration — recording the reason in the audit log. There is no paid tier and no way to buy an exemption.

#### GEO-17.10 — System health
**Depends-On:** GEO-20.1 · **Spec:** §17
**Acceptance criteria:** Queue depth, index status, ETL freshness, cache hit rate, error rate and database latency, with threshold colouring.

#### GEO-17.11 — Audit log viewer
**Depends-On:** GEO-9.7 · **Spec:** §17
**Acceptance criteria:** Filterable, exportable, **read-only** — no UI path mutates or deletes an audit row. Security/Auditor role sees this and nothing mutable.

---

## 20. Sprint 6 — Marketing, Docs & Launch Hardening

### EP-18 — Marketing Website & Docs (L8)

Spec §16 defines ten pages; each is a story. All build to the **expressive** intensity tier in `DESIGN_SYSTEM.md`.

| Story | Page | Key requirement |
|---|---|---|
| GEO-18.1 | Home | Value proposition, **live location search box**, **protocol selector rendering the same query as REST / GraphQL / gRPC**, coverage proof, CTAs (Spec §16) |
| GEO-18.2 | Products | API, npm, datasets, geocoding, boundaries, enterprise |
| GEO-18.3 | Developers | Quick-start paths by REST / GraphQL / gRPC / npm |
| GEO-18.4 | Data coverage | Contents, source & provenance policy, **stated limitations** |
| GEO-18.5 | **Support** | Free-forever commitment, running costs, donate action, sponsor wall. **Replaces the pricing page** — there is nothing to price. |
| GEO-18.6 | Docs hub | Technical documentation centre |
| GEO-18.7 | Changelog | API **and** dataset changes, generated from GEO-6.3 |
| GEO-18.8 | Status | Operational status and incident history |
| GEO-18.9 | About / Open Data | Mission, governance, licensing, contribution model |
| GEO-18.10 | Contact / Enterprise | Sales and government/institution integration enquiries |

#### GEO-18.11 — Marketing theme picker
**Depends-On:** GEO-14.6 · **User requirement**
**Acceptance criteria:** The theme picker is reachable from the marketing site (footer + a control in the navbar), so a visitor sets style/mode/brand before ever signing up, and the preference carries into the portal on signup.

#### GEO-18.12 — SEO, JSON-LD & performance
**Depends-On:** GEO-18.1 · **Spec:** §16
**Acceptance criteria:** `Organization`, `WebSite` and `SoftwareApplication` JSON-LD; sitemap; OG images; Core Web Vitals green on the home page; no layout shift from text animations.

### EP-19 — Observability & SLOs (L12)

| Story | Scope | Spec |
|---|---|---|
| GEO-20.1 | OpenTelemetry spans across gateway → transport → use case → DB/search/cache | §20 |
| GEO-20.2 | Structured JSON logs with the required fields; **never log key secrets**; redact authorization headers and sensitive query params | §20, §21 |
| GEO-20.3 | Metrics: request rate, p50/p95/p99, error rate, rate-limit rejects, DB latency, cache hit rate, search latency, ETL lag, index lag, queue depth | §20 |
| GEO-20.4 | SLOs: **99.9%** availability for public APIs after stabilization; separate SLOs for read API and the dataset publishing pipeline | §20 |
| GEO-20.5 | Public status page with incident communication | §20 |

### EP-20 — Security Hardening & Privacy (L12)

| Story | Scope | Spec |
|---|---|---|
| GEO-21.1 | TLS everywhere, HSTS, secure cookies, CSRF protection on cookie-authenticated writes | §12.4 |
| GEO-21.2 | Cloudflare WAF/CDN, DDoS protection, bot controls, request-body limits | §12.4 |
| GEO-21.3 | CORS allow-listing; browser-key origin enforcement | §12.4, §32.5 |
| GEO-21.4 | Least-privilege DB roles; secrets manager; environment separation | §12.4 |
| GEO-21.5 | Dependency/SAST/container scanning in CI with a patching cadence | §12.4 |
| GEO-21.6 | Privacy: data minimization at ingestion; **no precise user-location history stored by default**; short raw-log retention; published privacy policy and data-processing roles | §21 |
| GEO-21.7 | **GhanaPost exclusion test** — CI fails if any GhanaPostGPS digital-address data appears in fixtures, seeds, migrations or the canonical dataset | §2.2, R1 |

### EP-21 — Backups, DR & Runbooks (L12)

| Story | Scope |
|---|---|
| GEO-22.1 | Automated **MongoDB Atlas continuous backups with point-in-time restore**; tested restore into a scratch cluster |
| GEO-22.2 | **Restoration drill executed and documented** (Spec §27 Definition of Done) |
| GEO-22.3 | Blue/green or rolling API deploy with readiness/liveness probes |
| GEO-22.4 | Runbooks: incident response, key compromise, bad dataset release rollback, ETL failure, quota incident |

### EP-22 — Acceptance, Load & Compatibility Testing (all lanes)

| Story | Scope | Spec |
|---|---|---|
| GEO-23.1 | Domain/data acceptance suite: one country root, expected region count, active district → active region, spatial containment, valid coordinate ranges, geometry validity, **stable IDs across reimports**, deterministic alias normalization, same-name-different-district fixtures | §22.1 |
| GEO-23.2 | Cross-protocol semantic parity suite | §22.2 |
| GEO-23.3 | Security test suite: expired/revoked keys, scope enforcement, rate limits per tier, GraphQL depth/complexity attacks, gRPC oversized messages and deadlines, admin RBAC denials, CORS/CSRF/session fixation/logout/MFA recovery | §22.3 |
| GEO-23.4 | Load test asserting every p95 target | §22.4 |
| GEO-23.5 | Abuse and DR tests before public launch | §26 |

---

## 21. Quality, Security & Go-Live

### V1 Definition of Done (Spec §27) — launch checklist

- [x] Canonical dataset published with documented sources, licences, version and checksums — GEO-8.3. Six artifacts; every checksum computed from the bytes written and verified equal on download; CC BY attribution inside each file and on the API response.
- [x] REST, GraphQL and gRPC live behind the same authentication and rate-limit layer — shared resolver/Redis limiter and live 401/429/header evidence recorded in GEO-11.SECURITY
- [x] Anonymous sandbox demonstrates all three protocols safely — REST/GraphQL live calls plus an allow-listed, capped and independently throttled gRPC bridge
- [x] A developer can register, verify email, create an organization and application, create test/live keys, rotate/revoke them and inspect attributed usage — live lifecycle and three-protocol telemetry evidence recorded under GEO-9.3 and GEO-16.2–16.6
- [ ] npm package published with TypeScript types and an explicit dataset version — all seven V1 packages (`core`, `client`, `react`, `node`, `data`, `proto`, umbrella `ghanageo`) now build, test and pack; generated drift, ESM/CJS runtime entry points, `2026.08.3-ulid` exposure and no-default-key hygiene are verified. **Registry publication still requires npm release credentials and is an external launch action.**
- [x] Search, autocomplete, geocode, reverse and nearby meet functional **and** performance acceptance tests — GEO-12.6. All nine §22.4 targets pass against a live API (worst p95 72.5ms), and every hot query is confirmed IXSCAN with no COLLSCAN.
- [x] Admin can review, import, edit, publish and roll back a dataset version with complete audit history — GEO-17.3 adds permission-checked edit and deprecate for regions, districts and places; every mutation is audited, refusals included. Deprecation writes a redirect so an old id returns 410 with `mergedInto`, never 404. The admin CONSOLE screens for these remain (the API is complete and verified).
- [x] Marketing website and documentation are public with quick starts for REST, GraphQL, gRPC and npm — all five live on `/docs` (curl, CLI, React, GraphQL, gRPC), verified against the running site.
- [ ] The free-forever commitment is stated publicly, and a working donate action exists (§24) — the commitment is public on `/`, `/support` and `/transparency`, and the rate limiter structurally cannot read donation state. The mobile-money and card rails are labelled "not yet connected" because they are: connecting them needs merchant accounts, which is external-state work.
- [ ] Monitoring, alerting, backups and a completed restoration drill — the **production** drill is done and measured (87,798 documents restored, 0 failed, RTO ≈ 8 minutes; GEO-22.2). Outstanding: Atlas continuous backup/PITR, which needs an M10 cluster — M0 does not offer it — and production alerting.
- [x] Security review with no unresolved critical or high findings — GEO-21.REVIEW. govulncheck 0 reachable, pnpm audit clean, gosec's 27 findings fixed or triaged with reasons. Two real issues fixed. Hosted CodeQL/Actions remain blocked on GitHub account billing, which is not a finding. Evidence: `docs/runbooks/evidence/security-review-2026-08-29.md`.
- [x] **No unlicensed GhanaPostGPS digital-address data exists in the canonical dataset** — current corpus scan and positive CI regression fixtures pass (GEO-21.7)

### Success definition (Spec §1.2)

- [x] A developer discovers GhanaGeo, creates an account, generates a test key and makes a successful request **in under five minutes** — a fresh register → verify → login → organization → application → test key → authenticated search journey completed in 18 seconds against the live local stack (HTTP 200, 10 results).
- [x] All three protocols return semantically consistent results because they share use cases and repositories — transport constructors use the same geography/search services; contract mappings and live equivalent-query checks pass
- [x] Every record has a stable identifier, source provenance and dataset-version metadata — strict domain/storage invariants, complete-corpus audit and seed-replay identity evidence recorded under GEO-23.1-METADATA
- [x] Every public request is authenticated or deliberately anonymous, rate-limited, observable and abuse-resistant — shared REST/GraphQL/gRPC identity and quota enforcement, sandbox-specific caps, attributed secret-free telemetry and live 401/429 evidence are recorded above.
- [x] Data improves continuously without silently breaking applications — deterministic ULIDs, compatibility redirects, versioned checksummed releases, rollback, contract drift gates and the cursor-resumable dataset change stream are all verified.
- [x] The platform operates fully without GhanaPostGPS and can later enable digital addresses via a licensed adapter — exclusion CI passes and the licensing register keeps the adapter blocked

---

## 22. Risk Register

| # | Risk | Likelihood | Impact | Mitigation | Owner |
|---|---|---|---|---|---|
| RK-1 | **GhanaPostGPS data contamination** through a well-meaning contribution or a third-party dataset | Medium | **Critical** — legal | R1; adapter-only interface; CI exclusion test (GEO-21.7); licensing register review before any adapter | L0 |
| RK-2 | Seed districts marked `SEED_NEEDS_CANONICAL_RECONCILIATION` leak to production canonical status | Medium | High | R5; publication gate requires reviewed status; GEO-4.1 test | L1 |
| RK-3 | GNHR payload PII copied into the platform | Medium | **Critical** — privacy | R3; strip at ingestion before raw landing; GEO-4.5 adversarial test | L1 |
| RK-4 | Official GSS/GNHR district codes unavailable or inconsistent, blocking canonical reconciliation | High | Medium | Seed CSVs intentionally leave `official_code` blank; reconciliation is a reviewed workflow, not a blocker for V1 launch of reference data | L1 |
| RK-5 | Business logic drifts between the three transports | Medium | High | R6; shared use cases; cross-protocol parity suite (GEO-23.2) as a required CI check | L3 |
| RK-6 | **Search quality regression from the MongoDB move.** Self-hosted MongoDB `$text` has no fuzzy matching, so Spec §10's typo tolerance depends entirely on Atlas Search — a **managed-service lock-in** the PostGIS design did not have | **High** | High | `SearchPort` abstraction from day one with **two implementations built in Sprint 4**: Atlas Search (hosted) and Typesense (self-hosted, also used locally). Neither the use cases nor the transports know which is active. GEO-12.6 runs the golden search cases against **both**. | L5 |
| RK-7 | **Neumorphic and claymorphic styles fail WCAG contrast in data-dense admin screens** | **High** | High | Documented in `DESIGN_SYSTEM.md`: contrast-corrected variants, mandatory non-shadow boundary affordances, a hard AA gate in CI (GEO-14.7), and admin defaulting to the restrained intensity tier | L7 |
| RK-8 | `backdrop-filter` performance collapse in glass style on long tables and map overlays | Medium | Medium | Blur is capped and applied only to fixed-size chrome, never to scrolling containers; documented in `DESIGN_SYSTEM.md`; measured in the Design-QA gate | L7 |
| RK-9 | Browser API keys leak or are used from unauthorized origins | Medium | High | Key classes (Browser/Server/Test), origin allow-lists, scope restriction, lower quotas, GEO-9.3 unsafe-combination rejection | L4 |
| RK-10 | Sandbox abused as a free unauthenticated API | High | Medium | Dedicated key class, capped dataset, aggressive limits, allow-listed operations, never production credentials (GEO-15.1) | L9 |
| RK-11 | GraphQL complexity attacks exhaust the database | Medium | High | Depth + weighted complexity budgets, per-field geometry multipliers, query and statement timeouts (GEO-10.3) | L3 |
| RK-12 | A bad dataset release breaks consumers | Medium | High | Release workflow gates, checksums, independent rollback (R10), changelog, redirects for merged IDs | L1 |
| RK-13 | Contract drift between `contracts/` and implementations | Medium | High | Generation + diff in CI; drift fails the build (GEO-8.4, GEO-2.5) | L0 |
| RK-14 | Twelve parallel lanes collide on shared registry files | High | Medium | §8 exclusive ownership; central editing of registries; coordination notes on the task board (§1a) — the pattern that worked in RentOS | all |
| RK-16 | **Denormalized `regionName`/`districtName` in `places` drift after a rename**, so search and API responses disagree with canonical geography | Medium | Medium | Renames are transactional and enqueue a reconciliation job; a nightly consistency check reports drift; GEO-23.1 asserts zero drift | L1 |
| RK-17 | **No `ST_MakeValid` equivalent** — an invalid polygon reaches a canonical collection and silently corrupts `$geoIntersects` results | Medium | High | Geometry validity is a blocking Go-layer gate before every canonical write, with a known-bad-polygon test suite and winding-order normalization (GEO-5.3) | L1 |
| RK-18 | **Colour collision:** the default brand hue is green-teal, and green also means `--success`, `vs-canonical` and diff-added. A user-chosen green brand makes "published", "approved" and "added" indistinguishable | Medium | High | `DESIGN_SYSTEM.md` §6.6 reserves two hue arcs (130–165 success, 10–45 danger) from the brand axis; the picker snaps out of them; status and chart-series tokens never derive from `--brand-h`; a QA gate asserts it | L7 |
| RK-19 | **Ghanaian orthography renders as tofu.** Twi/Ga/Ewe aliases need `ɔ ɛ ŋ ɖ ƒ ʋ ɣ` plus combining tone marks, which live outside the default `latin` webfont subset — a place-names product for Ghana showing `Ɔsu` as a box is broken | **High** | High | `DESIGN_SYSTEM.md` §4.4.1: verify glyph coverage in the font binaries before locking any face, require the `latin-ext` subset, declare a Noto Sans fallback, and ship a CI fixture that renders the full character set at every weight and fails on a missing glyph | L7 |
| RK-15 | Monorepo CI time grows until it stops gating anything | Medium | Medium | Turborepo remote caching; `--affected` task filtering; split required vs advisory checks | L12 |

---

## 23. Post-V1 — Specification v2 Roadmap

V1 (Sprints 0–6 above) ships the platform. **Specification v2 is a separate delivery cycle**, and it re-enters the Ops Manual lifecycle at **Phase 2 — Discovery & Requirements Engineering**: each wave below produces its own BRD, PRD and client approval gate before any code. This section is the *shape* of v2, not its groomed backlog.

Sources: Build Specification §25 (Phase 6 — Expansion), §28 (Beyond V1), §32.6 (SDK family).

### 23.1 Wave overview

| Wave | Release | Theme | Gate to start |
|---|---|---|---|
| **V2.0** | Distribution | The full multi-language SDK family | V1 launched; REST/GraphQL/gRPC contracts frozen for 1 release cycle |
| **V2.1** | Enterprise | OAuth2 client credentials, SSO/SAML, service accounts, private datasets | ≥1 signed enterprise/government pilot |
| **V2.2** | Data depth | POI/business registry, road network expansion, address validation, vector tiles | Canonical geography stable; steward programme staffed |
| **V2.3** | Ecosystem | Webhooks/change feeds, contributor reputation, regional data-steward programme | Community contribution volume justifies moderation tooling |
| **V2.X** | *Gated* | **Licensed GhanaPostGPS adapter** | **Signed licence or written integration agreement. This is a legal gate, not an engineering one — it has no target date and must never be "started early".** |

---

### 23.2 EP-23 — SDK Family Expansion (Wave V2.0)

Spec §28 names six additional languages. The engineering problem is **not** writing six clients — it is preventing six clients from diverging. Everything below serves that.

**Discovery status (2026-08-29):** 🟡 **Plan ready; approval required.** TypeScript is now an explicit first-class reference SDK alongside the six additional languages. Draft approval artifacts: `docs/v2/v2.0-sdk-brd.md`, `v2.0-sdk-prd.md`, `v2.0-sdk-architecture.md`, and `v2.0-sdk-implementation-plan.md`. Implementation remains gated on V1 public launch, one frozen-contract release cycle, registry/repository ownership and approval of those documents.

#### GEO-24.1 — SDK conformance suite (build this first)
**Depends-On:** GEO-8.4, GEO-11.5 · **Blocks:** every other GEO-24.x story
**Acceptance criteria:**
- A **language-neutral conformance suite** defined as data (YAML cases + expected results) in `contracts/conformance/`, generated from the OpenAPI and protobuf contracts.
- Each case declares: operation, inputs, expected result shape, expected error code, expected quota cost.
- Every SDK ships a thin runner that executes the suite against a live test server and reports pass/fail per case.
- **CI runs the suite for every SDK on every contract change.** A contract PR that breaks any SDK fails before merge.
- The suite covers the Spec §22.2 cross-protocol parity cases, the Spec §32.7 React acceptance cases generalised to all languages, and every error code in Appendix B.

#### GEO-24.2 — SDK design charter
**Depends-On:** GEO-24.1
**Acceptance criteria:** `docs/sdk-charter.md` fixes, for every language: naming (idiomatic per language, semantically identical), the typed error surface (`code`, `status`, `requestId`), cancellation, retry-and-backoff policy, pagination iterator ergonomics, dataset-version exposure, telemetry opt-out, and the **hard rule that no SDK ships a default embedded API key**.

#### GEO-24.TS — TypeScript reference SDK completion
**Depends-On:** GEO-24.1, GEO-24.2 · **Blocks:** GEO-24.3–24.8
**Acceptance criteria:** The existing `@ghanageo/core`, `client`, `react`, `node`, `data`, `proto` and `ghanageo` packages are the first executable conformance target. Complete missing public operations (including boundaries/nested geography), cursor iterators, approved retry policy and telemetry opt-out; compile every example; preserve anonymous-by-default and injectable-fetch behavior; expose API and tested dataset versions independently; pass the full language-neutral suite and clean-consumer artifact verification. Resolve the TypeScript compiler-policy mismatch from the actual locked toolchain before declaring support.

| Story | Package | Registry | Idiomatic requirements |
|---|---|---|---|
| **GEO-24.3** | `ghanageo` (Python) | PyPI | Sync **and** `async`/`httpx` clients; full type hints + `py.typed`; Pydantic result models; pandas/GeoPandas interop helper for dataset downloads; supports the 3 most recent CPython releases |
| **GEO-24.4** | `github.com/ghanageo/ghanageo-go` | Go modules | `context.Context` first arg everywhere; functional options; errors via `errors.Is`/`As` on typed sentinels; generated from the same protobufs as the server |
| **GEO-24.5** | `ghanageo` (Dart/Flutter) | pub.dev | Null-safe; `Future`/`Stream` APIs; `CancelToken`; a `GhanaGeoLocationPicker` widget mirroring the React `LocationCombobox`; offline region datasets (ties to GEO-26.3) |
| **GEO-24.6** | `dev.ghanageo:ghanageo-java` | Maven Central | Java 21+; immutable records; `CompletableFuture` async; a Spring Boot starter with auto-configuration |
| **GEO-24.7** | `GhanaGeo` (.NET) | NuGet | `IGhanaGeoClient` for DI; `IAsyncEnumerable<T>` pagination; `CancellationToken` throughout; nullable reference types enabled |
| **GEO-24.8** | `ghanageo/ghanageo-php` | Packagist | PSR-18 HTTP client, PSR-7 messages, PSR-3 logging; a Laravel service provider and facade |

#### GEO-24.9 — SDK release automation
**Depends-On:** GEO-24.3–24.8
**Acceptance criteria:** One tagged release publishes every SDK. Each carries the **API version it targets** and the **dataset version it was tested against**, versioned independently (SemVer for code, `YYYY.MM.patch` for data — Spec §18). A contract change auto-opens a regeneration PR in every SDK repository.

#### GEO-24.10 — Docs parity
**Depends-On:** GEO-24.9
**Acceptance criteria:** Every quick start in the docs hub exists in all nine documented targets (TypeScript, React, Python, Go, Dart, Java, C#, PHP, curl). Snippets are **extracted from compiled, tested example projects** — never hand-written in Markdown, so they cannot rot.

---

### 23.3 EP-24 — Enterprise Access (Wave V2.1)

| Story | Scope | Spec |
|---|---|---|
| GEO-25.1 | **OAuth2 client credentials** flow for machine-to-machine, alongside API keys | §28 |
| GEO-25.2 | **SSO/SAML + OIDC** for enterprise organizations, with SCIM user provisioning | §28 |
| GEO-25.3 | **Service accounts** — non-human identities with scoped, rotatable credentials and their own audit identity | §28 |
| GEO-25.4 | **Private datasets** — government/enterprise layers over the public canonical geography, with per-organization visibility enforced in the repository layer, not the transport | §28 |
| GEO-25.5 | **Funding sustainability** — recurring donations, institutional sponsorship agreements, and grant reporting. Still no paid access tier; see §24. | product decision |
| GEO-25.6 | **Enterprise SLAs** — per-contract availability targets, priority support routing, configurable log retention (Spec §21) | §20, §21 |

---

### 23.4 EP-25 — Data Depth (Wave V2.2)

| Story | Scope | Notes |
|---|---|---|
| GEO-26.1 | **Business/POI registry** with confidence and freshness metadata | Spec §28. Every POI carries a last-verified timestamp; stale entries decay in ranking rather than disappearing. |
| GEO-26.2 | **Road network expansion** — full OSM road ingestion with classification and geometry | Spec §25 Phase 6. Volume makes this the first genuine sharding candidate. |
| GEO-26.3 | **Offline mobile datasets segmented by region** | Spec §28. Feeds `@ghanageo/data` and the Dart SDK (GEO-24.5). |
| GEO-26.4 | **Vector tiles and map layers** | Spec §28. **Revisits the V1 Leaflet decision:** vector tiles need a GL renderer, so this story either adds MapLibre GL alongside Leaflet or migrates. Requires an ADR. |
| GEO-26.5 | **Address validation API** for checkout and logistics forms | Spec §28. The highest-commercial-value v2 item; likely the first paid endpoint. |
| GEO-26.6 | **Postal metadata expansion** — `postal_areas` populated and served | Spec §3.1. Independent of GhanaPostGPS digital addresses (R1). |

---

### 23.5 EP-26 — Ecosystem & Change Distribution (Wave V2.3)

| Story | Scope | Spec |
|---|---|---|
| GEO-27.1 | **Webhooks** — signed, retried, replayable delivery of dataset-change events, built on the existing transactional outbox | §28 |
| GEO-27.2 | **Change feed API** — cursor-resumable HTTP feed complementing the gRPC `StreamDatasetChanges` of GEO-11.3 | §28 |
| GEO-27.3 | **Contributor reputation** — scored contribution history that weights a submission's review priority; never auto-approves | §28 |
| GEO-27.4 | **Regional data-steward programme** — scoped stewardship where a steward's authority is bounded to their region, enforced by RBAC | §28 |
| GEO-27.5 | **Public contribution portal** — submit a correction with evidence without a developer account | §16, §28 |

---

### 23.6 EP-27 — GhanaPostGPS Adapter (Gated — no date)

> **This epic is blocked by law, not by engineering.** No story in it may start — not a spike, not a prototype, not a "just the interface" PR beyond the empty port that already exists in V1 — until a signed licence or written integration agreement is filed in `docs/licensing-register.md` and countersigned by the Engineering Lead. R1 stands until then, and the CI exclusion test (GEO-21.7) stays enabled permanently, including after a licence is signed.

| Story | Scope |
|---|---|
| GEO-28.1 | Licence review and the data-handling terms it imposes — **legal deliverable, not engineering** |
| GEO-28.2 | Implement `ports/ghanapost.go` against the official API. **Lookup-through only: no bulk retrieval, no local persistence of digital addresses beyond the permitted cache window.** |
| GEO-28.3 | Digital-address resolution endpoint, feature-flagged and separately scoped (`ghanapost:read`), separately quota-classed |
| GEO-28.4 | Attribution, terms display, and a per-key entitlement check so only licensed consumers reach it |

---

### 23.7 What v2 explicitly does *not* include

Naming these prevents scope drift as much as naming what it does include.

- A general-purpose worldwide geocoder (Spec §2.2) — GhanaGeo stays Ghana-specific.
- Turn-by-turn routing or a navigation engine (Spec §2.2).
- Real-time traffic (Spec §2.2).
- Property ownership or personal household data (Spec §2.2, R3).
- Emergency-service dispatch (Spec §2.2).
- Redistribution of any GhanaPostGPS digital address, licensed or not — the licence in EP-27 permits *lookup*, never *redistribution* (R1).

---

## 24. Funding Model — Free Forever

**Decision, 2026-08-29: GhanaGeo is free. There is no paid tier, and there will
not be one.** It is funded by voluntary donations and institutional sponsorship.
This supersedes every pricing and billing item in Build Specification §14, §16
and §28.

### 24.1 Why this is written down as a rule

A free product drifts toward a paid one one exception at a time: an enterprise
asks for a higher limit, a plan field appears "just in case", and eventually
access is for sale. The rules below exist so that drift requires an explicit,
visible decision rather than a quiet PR.

| # | Rule |
|---|---|
| F1 | **No code path may charge a developer for access.** No payment provider, no card on file, no invoice, no plan upgrade. CI fails on a dependency on a payments SDK. |
| F2 | **Fair-use limits are identical for everyone.** Quota resolution takes account of environment and key class only. It must not read donation or sponsorship state — a test asserts this. |
| F3 | **Sponsorship buys recognition, never capability.** A sponsor gets a logo on the marketing site and in the docs. It does not get a higher limit, priority support, earlier data, or any endpoint an anonymous developer cannot reach. |
| F4 | **A limit may be raised on documented need, never on payment.** A steward can lift a specific application's ceiling for a research, humanitarian or government use, recording the reason in the audit log. Money is never the reason. |
| F5 | **Donation is never a dark pattern.** No interstitial, no rate-limit page that suggests donating as the fix, no countdown, no guilt. One honest, dismissible ask. |
| F6 | **Funding is reported publicly.** What came in, what it cost to run, what it paid for. A public-good project asking for public money shows its books. |

### 24.2 What donations actually fund

Stated plainly on the support page, because a vague ask gets vague support:

- Hosting: the API, database, search index and CDN.
- Data acquisition: licensed source data and the steward time to reconcile it.
- The bandwidth cost of bulk dataset downloads, which is the single largest variable expense.
- Nothing else. No salaries are implied until the page says so.

### 24.3 Stories

#### GEO-29.1 — Support page (`apps/web`)
**Depends-On:** GEO-18.1 · **Lane:** L8 · **Replaces** GEO-18.5
**Acceptance criteria:**
- States the free-forever commitment in the first screenful, without qualification.
- Shows current monthly running cost and current funding against it. Real numbers or none — no fabricated progress bar.
- A donate action supporting one-off and recurring gifts, with a Ghana-appropriate rail (mobile money) alongside card, since a Ghanaian audience should not be forced through a card-only flow.
- A sponsor wall, ordered by longevity rather than amount, so the page does not become a leaderboard.
- An explicit line: *"Donating does not change your rate limits."*
- Accessible, keyboard-operable, and rendering correctly in all three materials.

#### GEO-29.2 — Support control in the shell
**Depends-On:** GEO-14.5, GEO-29.1 · **Lane:** L7/L10
**Acceptance criteria:** A **Support** entry in the navbar help menu and in the marketing footer. It is a link, never a modal, never an interstitial (F5). It never appears on a `429` page.

#### GEO-29.3 — Sponsor model and recognition
**Depends-On:** GEO-16.3 · **Lane:** L10
**Acceptance criteria:** An organization may record a sponsor profile. Recognition renders on the marketing site and docs footer. **A test asserts that quota resolution produces an identical result for a sponsored and an unsponsored organization** (F2, F3).

#### GEO-29.4 — Transparency report
**Depends-On:** GEO-29.1 · **Lane:** L8
**Acceptance criteria:** A public page showing income, running costs and what was funded, updated on a stated cadence. Generated from recorded figures, never hand-written prose about them (F6).

---

## Appendix A — API Scope Catalog

Spec §12.3. Every operation declares exactly one required scope.

| Scope | Grants | Browser-key safe |
|---|---|---|
| `locations:read` | regions, districts, places, aliases | ✅ |
| `search:read` | `/search`, `/autocomplete` | ✅ |
| `geocode:read` | `/geocode`, `/reverse`, `/nearby` | ✅ |
| `boundaries:read` | `/boundaries/{id}`, geometry fields | ⚠️ quota-heavy; allowed with a lower cap |
| `datasets:read` | `/datasets`, download metadata | ✅ |
| `graphql:access` | `POST /graphql` | ✅ with complexity cap |
| `grpc:access` | gRPC and ConnectRPC methods | ❌ server keys only |

**Unsafe Browser-key combinations rejected by GEO-9.3:** any scope set including `grpc:access`; `boundaries:read` without an origin allow-list.

---

## Appendix B — Error Code Catalog

Spec §19. Source of truth is `contracts/errors/catalog.yaml`; this table is the human summary.

| Code | HTTP | gRPC | When |
|---|---|---|---|
| `INVALID_ARGUMENT` | 400 | `INVALID_ARGUMENT` | Malformed parameter, bad coordinate, bad radius |
| `INVALID_COORDINATES` | 400 | `INVALID_ARGUMENT` | Latitude/longitude outside valid ranges |
| `RADIUS_OUT_OF_RANGE` | 400 | `INVALID_ARGUMENT` | Nearby radius above the documented maximum |
| `QUERY_TOO_SHORT` | 400 | `INVALID_ARGUMENT` | Search/autocomplete below minimum length |
| `PAYLOAD_TOO_LARGE` | 413 | `RESOURCE_EXHAUSTED` | Request body or gRPC message over limit |
| `UNAUTHENTICATED` | 401 | `UNAUTHENTICATED` | Missing or malformed key |
| `KEY_REVOKED` | 401 | `UNAUTHENTICATED` | Key revoked or expired |
| `PERMISSION_DENIED` | 403 | `PERMISSION_DENIED` | Scope not granted |
| `ORIGIN_NOT_ALLOWED` | 403 | `PERMISSION_DENIED` | Browser key used from an unlisted origin |
| `NOT_FOUND` | 404 | `NOT_FOUND` | Unknown ID |
| `RESOURCE_GONE` | 410 | `NOT_FOUND` | Deprecated with no successor |
| `RATE_LIMIT_EXCEEDED` | 429 | `RESOURCE_EXHAUSTED` | Burst limit hit; includes retry guidance |
| `QUOTA_EXCEEDED` | 429 | `RESOURCE_EXHAUSTED` | Monthly quota consumed |
| `QUERY_TOO_COMPLEX` | 400 | `INVALID_ARGUMENT` | GraphQL depth or complexity budget exceeded |
| `DEADLINE_EXCEEDED` | 504 | `DEADLINE_EXCEEDED` | Query or statement timeout |
| `INTERNAL` | 500 | `INTERNAL` | Unhandled — always logged with request ID |

Every error carries `requestId` and a `docs` link to `/docs/errors/{CODE}`.

---

## Appendix C — RBAC Permission Catalog

Spec §17.1. Six roles. Permissions are enforced at the API; UI gating is presentation only.

| Permission | Super Admin | Data Admin | Data Reviewer | Data Contributor | Developer Support | Security/Auditor |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| View geography | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Edit geography | ✅ | ✅ | — | — | — | — |
| Edit geometry | ✅ | ✅ | — | — | — | — |
| Propose change request | ✅ | ✅ | ✅ | ✅ | — | — |
| Review / approve change request | ✅ | ✅ | ✅ | — | — | — |
| Run import job | ✅ | ✅ | — | ✅ (mapping only) | — | — |
| Publish dataset release | ✅ | ✅ | — | — | — | — |
| Roll back dataset release | ✅ | ✅ | — | — | — | — |
| View organizations / keys | ✅ | — | — | — | ✅ | ✅ |
| Suspend key | ✅ | — | — | — | ✅ | — |
| Edit rate-limit plans | ✅ | — | — | — | — | — |
| Manage roles | ✅ | — | — | — | — | — |
| System configuration | ✅ | — | — | — | — | — |
| View audit log | ✅ | ✅ | — | — | — | ✅ |
| Mutate any content | ✅ | ✅ | partial | propose only | — | **never** |

**MFA is mandatory for every role in this table** (Spec §12.1).

---

## Appendix D — Quota Cost Class Registry

Spec §13.

| Class | Example operations | Relative cost |
|---|---|---|
| Cheap | `GET /regions/{id}`, `/places/{id}`, `GetRegion`, `GetPlace` | **1** |
| Normal | `/search`, `/autocomplete` | **2** |
| Spatial | `/reverse`, `/nearby` | **4** |
| Geometry | `/boundaries/{id}`, large GeoJSON fields | **5–10** (by vertex count) |
| GraphQL | Computed from the requested field set and per-field multipliers | **dynamic** |
| gRPC stream | Connection cost + per-message accounting | **dynamic** |

Burst limits are tracked separately from monthly quotas. Sandbox limits are strictly tighter than authenticated production.

**These are fair-use limits, not a price ladder.** Every authenticated developer gets the same allowance regardless of whether they donate. Limits exist so one consumer cannot degrade the service for everyone else; they are never a lever to sell an upgrade (§24).

---

## Appendix E — Seed Data Contract

Spec §31. Source files are already in the repository and move to `data/seed-data/` in GEO-1.1.

| File | Rows | Notes |
|---|---|---|
| `regions.csv` | **16** | `id, country_code, name, capital, official_code, source_code, status, verification_status, source_url, retrieved_at, notes`. `official_code` intentionally blank pending GNHR/GSS reconciliation. `verification_status = REFERENCE`. |
| `districts.csv` | **261** | `id, region_id, region_name, name, classification_hint, official_code, capital, status, verification_status, source_url, retrieved_at, notes`. All rows `SEED_NEEDS_CANONICAL_RECONCILIATION`. |
| `places.csv` | **16** | Regional capitals only. Coordinates and district assignment intentionally blank pending geospatial ingest. |
| `sources.csv` | — | To be authored in GEO-4.3 from the licensing register. |
| `manifest.json` | — | Dataset version, per-file checksums, `generated_at`, source revisions. |

**Hard CI assertions (Spec §31.1):** exactly 16 seed regions; exactly 261 unique district/MMDA seed rows; seed IDs are GhanaGeo internal bootstrap IDs and are never presented as government codes; seed import is idempotent; no `SEED_NEEDS_CANONICAL_RECONCILIATION` row reaches `PUBLISHED` via an automated path.

---

## Appendix F — Service / Port / Owner Registry

| Component | Local port | Owner lane | Deploy target |
|---|---|---|---|
| `services/api` — REST + GraphQL | 8180 | L2/L3/L4 | Render |
| Redis — fair-use buckets | 6679 | L4 | Render |
| `services/api` — gRPC | 9190 | L3 | Render |
| `services/api` — ConnectRPC (browser) | 8081 | L3 | Render |
| `services/worker` | — | L1/L12 | Render |
| MongoDB 8.3 (replica set `rs0`) | 27017 | L1 | MongoDB Atlas |
| Typesense (local search only) | 8108 | L5 | local/CI only |
| Redis 7 | 6379 | L4 | Render |
| `apps/web` (marketing + docs) | 3000 | L8 | Vercel |
| `apps/sandbox` | 3001 | L9 | Vercel |
| `apps/portal` | 3002 | L10 | Vercel |
| `apps/admin` | 3103 | L11 | Vercel |
| Prism mock (OpenAPI) | 4010 | L0 | local/CI only |
| `cli/` — public CLI binary | — | L6 | npm · Homebrew · `go install` |

### Domain architecture — `digitalghana.dev`

GhanaGeo is the **first** product on a Ghanaian digital-public-infrastructure
platform, not the whole of it. The naming reflects that from day one, because
retrofitting a namespace after launch means breaking every published URL,
every SDK default and every developer's stored configuration.

```
digitalghana.dev                        the platform — what exists, who runs it
└── geo.digitalghana.dev                GhanaGeo: marketing, docs, changelog, status
    ├── api.geo.digitalghana.dev        REST /v1 and GraphQL
    ├── grpc.geo.digitalghana.dev:443   gRPC and ConnectRPC
    ├── sandbox.geo.digitalghana.dev    public playground, capped dataset
    ├── console.geo.digitalghana.dev    developer portal
    └── admin.geo.digitalghana.dev      admin and data stewardship
```

A second product takes `<product>.digitalghana.dev` and the same internal
shape, so the platform's URL grammar is learnable after seeing it once.

**Operational note.** A single `*.digitalghana.dev` wildcard certificate covers
`geo.` but **not** `api.geo.` — wildcards match one label only. Two options:
issue per-host certificates automatically (Vercel and Cloudflare both do this,
and it is the default assumption here), or buy `*.geo.digitalghana.dev` as
well. Decide before the first public DNS record, because moving a published
API host later is a breaking change for every consumer.

**Shared across products:** one identity system, one design system
(`packages/ui`), one status page, one support and funding surface (§24). A
developer should need one account for the whole platform, not one per product.

**Public surfaces:** `https://geo.digitalghana.dev` (marketing/docs) · `https://api.geo.digitalghana.dev/v1` (REST) · `https://api.geo.digitalghana.dev/graphql` · `grpc.geo.digitalghana.dev:443` · `https://sandbox.geo.digitalghana.dev` · `https://console.geo.digitalghana.dev` (portal) · `https://admin.geo.digitalghana.dev`.

---

*End of plan. Update §1a after every batch of work.*
