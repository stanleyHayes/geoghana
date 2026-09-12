# Contributing to GhanaGeo

GhanaGeo is public-interest infrastructure: source-linked Ghanaian administrative geography with visible provenance, pending canonical reconciliation against official GSS/GNHR sources. Contributions to code, data provenance, documentation and developer experience are welcome.

This repository is a monorepo. A change usually touches one lane, and lanes own their directories outright — read [`AGENTS.md`](AGENTS.md) before you start, so two contributors do not edit the same file at once.

## Your first contribution

1. **Find out whether the work is already claimed.** The live task board is §1a of [`agent_plan.md`](agent_plan.md); [`ROADMAP.md`](ROADMAP.md) is the readable summary. Check whether the item is blocked on an external gate before you write code for it.
2. **Read the rules that reject a pull request on sight.** They are the five in [`CLAUDE.md`](CLAUDE.md) and the numbered rules R1–R17 in [`agent_plan.md`](agent_plan.md) §2. Most of them exist because something went wrong once.
3. **Set up locally and confirm a clean checkout passes verification before you change anything.** If it does not pass, that is itself worth reporting.
4. **Open an issue first for anything non-trivial** — a contract change, a new source, a data correction, a locked-technology change. Evidence is easier to agree on before code exists.
5. **Keep the change small.** One concern per pull request. Contract changes (`contracts/`, `proto/`) always ship in their own pull request.

### Prerequisites

| Tool | Version | Where it is pinned |
|---|---|---|
| Node.js | ≥ 22 | `engines` in [`package.json`](package.json) |
| pnpm | 11.11.0 | `packageManager` in [`package.json`](package.json) |
| Go | 1.26.6 | [`go.work`](go.work) and each `go.mod` |
| Docker | any recent version | [`docker-compose.yml`](docker-compose.yml) — MongoDB, Redis, Typesense |
| Ruby | 3.x | only for `production-preflight` and the conformance matrix validator |

SDK lanes need their own toolchains — Python 3.12+, Dart, Java 21, .NET 8, PHP 8.2+ — but only if you are working in `sdks/`.

### Local setup

```sh
git clone https://github.com/stanleyHayes/geoghana.git
cd geoghana
pnpm install
./scripts/gen-env.sh          # safe to re-run; it only fills blanks

make up                       # MongoDB (rs0) + Redis + Typesense on the dedicated port block
make seed                     # 16 regions / 261 districts / 16 places, idempotent
make api                      # REST + GraphQL + gRPC on :8180 / :9190
```

Find anything the generator could not supply with `grep -rn 'PASTE_' --include='.env*' . | grep -v node_modules`. Nothing `gen-env.sh` writes is ever committed.

Front-end work additionally wants an app running — `pnpm --filter @ghanageo/admin dev` (`:3103`), or `./scripts/dev.sh` to start all four.

### Verification

Run these before opening a pull request. They are the same commands CI runs.

```sh
make test              # go test ./... in services/api and services/worker, then pnpm test
make lint              # go vet ./... in both services, then pnpm lint
pnpm typecheck
pnpm build
```

Then whichever gates your change touches:

```sh
make contracts             # you changed contracts/ or proto/
make conformance           # you changed an SDK facade or the conformance kit
make sdk-matrix-verify     # you changed something every SDK lane depends on
make docs-sdk-check        # you changed generated SDK documentation or its examples
make check-licensed-data   # always, if you touched data/
pnpm check:links           # you changed routes or documentation links
make perf-indexes          # you changed a query or an index
```

CI is [`.github/workflows/quality.yml`](.github/workflows/quality.yml) and [`.github/workflows/security.yml`](.github/workflows/security.yml). A red gate is treated as a blocking defect, not a flake.

**Front-end changes are not done until you have looked at them.** Render the change in all three materials against both light and dark, and walk the Design-QA gates in [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md) §23. Verify in a browser, not by reading the code: both the CORS gap and the nested dark-selector bug were invisible in review and obvious in Playwright. Recorded fixes and evidence are in [`design-qa.md`](design-qa.md).

## Good first contributions

Each of these is a real, currently open gap in this repository, not a made-up starter task.

1. **Write the four missing ADRs.** [`agent_plan.md`](agent_plan.md) §3 states that changing a locked technology choice requires an ADR in `docs/adr/`, and the EP-01 acceptance criteria name `0001-modular-monolith`, `0002-postgis-canonical-store`, `0003-contracts-first` and `0004-tri-morphic-design-system`. The directory is empty. The decisions and their rationale are already written down in §3 and in the MongoDB deviation note — they need to be turned into real records.
2. **Author `data/seed-data/sources.csv`.** Appendix E of [`agent_plan.md`](agent_plan.md) lists it as still to be authored from [`docs/licensing-register.md`](docs/licensing-register.md). Every other seed file exists; this one does not, so source metadata currently lives only in prose.
3. **Add an index for the error catalogue.** [`docs/errors/`](docs/errors) holds 16 individual pages and [`contracts/errors/catalog.yaml`](contracts/errors/catalog.yaml) holds the machine-readable catalogue, but nothing ties them together. An index page — ideally generated from the catalogue so it cannot drift — would make the `docs` field in every error envelope land somewhere useful.
4. **Split the stale GEO-12.5 / GEO-12.6 board item.** The 2026-08-29 reconciliation in [`agent_plan.md`](agent_plan.md) records this explicitly: `/nearby` already works nationwide, so the implemented path should close independently of the p95 load gate that is still measurable work.
5. **Populate `data/schemas/` and `data/transforms/`.** Both directories are empty, although the folder structure in [`agent_plan.md`](agent_plan.md) §4 describes them as holding JSON Schema for source payloads and the normalisation/matching rules. Those rules currently exist only as Go code, so they cannot be reviewed by anyone who does not read Go.
6. **Publish the unassigned-geometry tail.** [`docs/boundary-coverage.md`](docs/boundary-coverage.md) records that 19,601 of 19,686 points of interest and 19,627 of 19,728 roads resolve to a district, and explains why the rest do not — offshore, or across a land border. There is no published list of *which* records those are, so nobody can check the explanation.

## Change expectations

- **Never add GhanaPostGPS digital-address data.** Not scraped, not stored, not redistributed, not in a fixture. This is rule R1 and CI fails on any such payload. Integration exists only as an empty adapter port behind a licence gate.
- **Transports contain no business logic.** REST handlers, GraphQL resolvers and gRPC services validate wire format, call a use case, and map the result back. A domain `if` inside a handler is a bug (R6).
- **No `bson` tags in `internal/domain`.** Persistence mapping belongs in `internal/adapters/mongo` (R3 of `CLAUDE.md`).
- **No hardcoded colour, shadow, radius or blur in a component.** Tokens only. A needed token is a request to the design-system lane, never a local override — rule 4 of [`CLAUDE.md`](CLAUDE.md), and R17 requires every front-end pull request to satisfy [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md).
- **No secrets in source or a client bundle.** Configuration comes from the environment; CI secret-scans every commit (R12).
- **Every canonical record carries provenance and a dataset version** (R2), and a source's licence is registered before its adapter is written (R4).
- **Every import is idempotent** and no source overwrites canonical data merely by arriving later (R8).
- **Stable identifiers are ULIDs and never change.** Merges create redirects and tombstones, not deletions (R7).
- **No code path may charge for access.** GhanaGeo is free; §24 of [`agent_plan.md`](agent_plan.md) explains why that is written down as a rule rather than left as an intention.
- **Do not imply government endorsement or official status** anywhere — in code, data, copy or metadata.
- **Update documentation in the same pull request** when a change affects a documented workflow or contract (R15), and update the task board in [`agent_plan.md`](agent_plan.md) §1a when a story changes state.

## Commit and pull-request conventions

This repository's history is ticket-prefixed. Follow it.

| Artefact | Format |
|---|---|
| Branch | `feature/GEO-12.3-short-slug` |
| Commit | `GEO-12.3 imperative summary` — no scope, no trailing full stop |
| Pull request | `GEO-12.3 Title` — the only merge path |

```text
GEO-UI-01 stop a scopeless API key blanking the key-management screen
GEO-SEC-02 count wrong passwords instead of accepting them forever
GEO-30.1 record frontend production evidence
GEO-4.7 Include roads and POIs in bulk downloads
```

A pull request should state:

- the scope of the change and which lane and directories it touches;
- the contract, source or evidence file it relies on;
- the verification actually performed — paste the commands you ran and their result;
- migration and rollback impact;
- any external gate that remains open.

Every story carries a User Story, Business Value, Acceptance Criteria, Technical Notes, Definition of Done, Estimates and Dependencies. The global Definition of Done is §10 of [`agent_plan.md`](agent_plan.md).

## Data corrections

Corrections need evidence, not confidence. Provide:

- the affected stable identifier (the ULID, not the name);
- the current published value;
- the proposed value;
- the authoritative source, with a stable URL or reference;
- the source publication or effective date;
- whether the correction changes historical records.

Automation may draft a correction; a human steward approves canonical publication. Rows marked `SEED_NEEDS_CANONICAL_RECONCILIATION` are never promoted to canonical by an automated path (R5). Unknown licence status is recorded as unknown, never assumed open — a new source must be added to [`docs/licensing-register.md`](docs/licensing-register.md) *before* its adapter is written, and attribution must travel with the data into API responses and export files, not just into a footer.

## Review expectations

- A maintainer reviews every pull request. Expect questions about provenance, licence and evidence before questions about style.
- Changes that alter published state — a dataset version, a contract, a hostname, a lifecycle word — need the evidence in the diff, not in the conversation.
- Contract and protobuf changes are reviewed separately from the code that consumes them, and breaking changes need a migration, a redirect or a tombstone.
- Preserve unrelated in-flight work. If you find someone else's change in a shared worktree, leave a coordination note rather than reverting it (R14).
- Discussion stays on evidence and public benefit. Conduct expectations are in [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
- Security issues do not go in a pull request or a public issue. See [`SECURITY.md`](SECURITY.md).

## Licensing

Unless explicitly stated otherwise, contributions intentionally submitted for inclusion are provided under the Apache License 2.0 under that licence's inbound = outbound terms (section 5). See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).

Contributions must not include third-party data or documents without recorded permission. The Apache-2.0 repository licence does not relicense any ingested dataset: GeoNames and geoBoundaries remain CC BY 4.0, OpenStreetMap-derived data remains ODbL 1.0 and share-alike, and upstream terms always govern.

## Independence

GhanaGeo is an independent open-source project. It is not operated by, endorsed by, or affiliated with the Government of Ghana, the Ghana Statistical Service, Ghana Post or any agency. Do not submit changes that imply otherwise.
