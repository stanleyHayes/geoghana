# GhanaGeo

Source-linked Ghanaian administrative geography — 16 regions, 261 districts, 15,924 localities, complete district boundary geometry — served over REST, GraphQL and gRPC, with source provenance and a dataset version on every single record. No record is canonical yet: districts carry `SEED_NEEDS_CANONICAL_RECONCILIATION`, regions and places carry `REFERENCE`, pending reconciliation against an official GSS/GNHR source.

![Licence Apache-2.0](https://img.shields.io/badge/licence-Apache--2.0-blue.svg)
![CI local only](https://img.shields.io/badge/CI-local%20gates%20only-lightgrey)
![Live at geo.digitalghana.dev](https://img.shields.io/badge/live-geo.digitalghana.dev-brightgreen)
![Go 1.26.6](https://img.shields.io/badge/Go-1.26.6-00ADD8)
![Next.js 16.3.3](https://img.shields.io/badge/Next.js-16.3.3-black)

Hosted GitHub checks are not running: Actions are blocked on account billing (see [`ROADMAP.md`](ROADMAP.md)). The same gates run locally via `make test`, `make lint`, `make contracts`.

GhanaGeo is an independent [Digital Ghana](https://digitalghana.dev) product. It is free, and it is designed to stay free: there is no paid tier, and fair-use limits are identical for everyone (see [§24 of `agent_plan.md`](agent_plan.md)).

---

## Live now

State measured against the public hostnames on **2026-09-12**. Nothing in this table is aspirational.

| Surface | Host | State |
|---|---|---|
| Marketing site and documentation | [geo.digitalghana.dev](https://geo.digitalghana.dev) | live |
| Interactive sandbox | [sandbox-geo.digitalghana.dev](https://sandbox-geo.digitalghana.dev) | live |
| Developer console | [console-geo.digitalghana.dev](https://console-geo.digitalghana.dev) | live |
| Admin and data-steward portal | [admin-geo.digitalghana.dev](https://admin-geo.digitalghana.dev) | live |
| Public REST/GraphQL API | `api-geo.digitalghana.dev` | **not deployed** |
| Native gRPC | `grpc-geo.digitalghana.dev` | **not deployed** |

**Be clear about what that means.** The four Next.js frontends are deployed, on canonical hosts, behind a valid certificate. The API is not. It is blocked on production provider values — the Render preflight still reports 14 missing API/worker settings, and production Redis and Typesense (or Atlas Search) have not been provisioned — not on missing code. Evidence: [`docs/runbooks/evidence/digitalghana-launch-reconciliation-2026-09-01.md`](docs/runbooks/evidence/digitalghana-launch-reconciliation-2026-09-01.md).

You can verify both statements yourself:

```sh
curl -s -o /dev/null -w '%{http_code}\n' https://geo.digitalghana.dev
# 200

curl -s -o /dev/null -w '%{http_code}\n' https://api-geo.digitalghana.dev/health
# 404 — the API host is not serving an application yet
```

Until it is deployed, the working API is the local one, and it is four commands away — see [Quickstart](#quickstart). When the API does land on Render's container tier, the first request after an idle period will take 30–60 seconds to wake the instance; the second is fast.

---

## The problem this solves

### For developers

If you have shipped software that has to know where things are in Ghana, you have already paid these costs:

- **You hardcoded the region list, and then Ghana changed it.** Ghana went from 10 regions to 16 in 2018–2019 and the district count settled at 261 MMDAs. Both numbers ended up pasted into a constants file in every codebase that needed them, each copy a slightly different vintage, and joins between two systems silently stopped matching.
- **You could not get district boundaries at all.** The widely used open release, geoBoundaries `gbOpen/GHA/ADM2`, represents **2019**. Guan District was inaugurated on 8 October 2021 and simply is not in it; Atwima Nwabiagya appears pre-split; several districts have since been renamed. So "which district is this coordinate in" either failed for those areas or fell back to nearest-locality guessing, which quietly reports the wrong district rather than reporting nothing.
- **You wrote your own fuzzy matcher for place names.** Ghanaian place names carry Twi, Ga and Ewe orthography, multiple accepted spellings and heavy alias use (`Osu RE`, `Tema Comm 25`). Exact matching fails constantly, so every team builds an ad-hoc normaliser, and no two agree.
- **You had no provenance to point at.** When someone asks "where did this district code come from, and when", a scraped CSV has no answer. There is no retrieval date, no licence record, no dataset version, so the number cannot be defended, reproduced or corrected.
- **You risked shipping data you were not licensed to redistribute.** GhanaPostGPS digital addresses are owned by Ghana Post. Scraping them into your own database is a legal exposure that is very easy to create by accident and very hard to unwind.

What GhanaGeo gives you instead:

| Instead of | You get |
|---|---|
| A pasted district list | `GET /v1/districts` — 261 MMDAs with stable ULIDs that never change across reimports |
| Nearest-locality guessing | `GET /v1/reverse` — polygon containment, because **261 of 261 districts have geometry** ([`docs/boundary-coverage.md`](docs/boundary-coverage.md)) |
| Your own matcher | `GET /v1/search` — typo-tolerant, alias-aware, every result carrying a confidence score and a `matchReason` |
| An undated CSV | Every record carrying `provenance.sourceId`, `provenance.retrievedAt` and `datasetVersion`; every response carrying the version it was served from |
| A breaking rename | Merged and deprecated identifiers return `410` with `error.details.mergedInto`, so stored ids keep resolving |
| Legal risk | A hard exclusion rule, enforced in CI, that no GhanaPostGPS digital address can enter the dataset |

Seven official SDKs, a CLI, an offline dataset package and three transports over one shared application layer, so a REST, GraphQL and gRPC query for the same thing return the same answer.

### For the community

Administrative geography decides who gets counted. A district boundary determines which clinic a village is reported against, which constituency a household votes in, and whether a delivery, a survey or a subsidy reaches a place at all. When the only usable copy of that geography lives in a private spreadsheet or an undated scrape, nobody can audit it, nobody can correct it, and every organisation quietly pays to rebuild the same thing badly.

GhanaGeo exists as open, source-linked, independent infrastructure so that four things are true at once. **Provenance is visible, not asserted** — every source is recorded in [`docs/licensing-register.md`](docs/licensing-register.md) *before* its adapter is written, and attribution travels on the records themselves, in API responses and inside every export file, because ODbL is share-alike and a footer is not good enough. **Results are reproducible** — datasets are immutable, versioned and checksummed, and publication is decoupled from code deployment and independently rollback-able. **Uncertainty is published rather than hidden** — bootstrap rows are labelled `SEED_NEEDS_CANONICAL_RECONCILIATION` and can never be promoted to canonical by an automated path, and GeoNames-derived places land as `REFERENCE`, not as truth about Ghanaian administrative geography. **There is no lock-in** — Apache-2.0 code, published OpenAPI, GraphQL and protobuf contracts, bulk CSV and GeoJSON downloads, no proprietary map provider and no API key needed for tiles; if this project stops, the data and the code remain forkable. And a correction with evidence has a real path: a human steward reviews it, the decision enters an immutable audit chain including refusals, and it ships as a new dataset version.

### What this is not

Drawn from the stated non-goals in [`agent_plan.md`](agent_plan.md) §2.1 and §23.7:

- **Not a government service.** Not operated by, endorsed by or affiliated with the Government of Ghana, the Ghana Statistical Service, Ghana Post or any agency.
- **Not a worldwide geocoder.** GhanaGeo is Ghana-specific and stays Ghana-specific.
- **Not routing, navigation or real-time traffic.** No turn-by-turn engine, ever.
- **Not GhanaPostGPS.** Digital addresses are proprietary to Ghana Post and are never scraped, stored or redistributed. A port exists with no implementation, behind a licence gate, and the CI exclusion test stays enabled permanently — including after any licence is signed, because such a licence would permit *lookup*, never *redistribution*.
- **Not household, property-ownership or personal data.** PII is stripped at ingestion, before the raw landing write, not at serving time.
- **Not emergency dispatch.** Verified emergency contacts are a different Digital Ghana product.
- **Not a complete national gazetteer.** The place corpus is a source-linked reference subset, and says so on every record.

---

## Quickstart

Prerequisites: **Node.js ≥ 22** (`engines` in `package.json`), **pnpm 11.11.0** (pinned via `packageManager`), **Go 1.26.6** (`go.work`), and Docker for the local data stack.

```sh
git clone https://github.com/stanleyHayes/geoghana.git
cd geoghana
pnpm install
./scripts/gen-env.sh          # writes .env files; fills every secret it can generate itself

make up                       # MongoDB (replica set rs0) + Redis + Typesense
make seed                     # 16 regions / 261 districts / 16 places — idempotent
make api                      # REST + GraphQL + gRPC on :8180 / :9190
```

`make dev` runs those last three in order. `./scripts/dev.sh` additionally starts all four Next.js apps and reports what is listening where; `./scripts/dev.sh --stop` stops them.

GhanaGeo claims a dedicated local port block, because a development machine usually already has something on the defaults: MongoDB `27117`, Redis `6679`, Typesense `8108`, API `8180`, gRPC `9190`, worker metrics `9091`, and the four apps on `3100` (web), `3101` (sandbox), `3102` (portal) and `3103` (admin).

---

## Usage

Public reads are anonymous by default. A key is sent only when you supply one; no package, example or fixture in this repository contains an API key.

```sh
curl -s http://localhost:8180/health
```

```json
{ "status": "ok", "datasetVersion": "2026.08.3-ulid" }
```

`datasetVersion` echoes whatever `DATASET_VERSION` the process was started with — `2026.08.3-ulid` in both [`docker-compose.yml`](docker-compose.yml) and [`render.yaml`](render.yaml).

```sh
curl -s "http://localhost:8180/v1/regions?limit=1"
```

Response shape from [`contracts/openapi/v1.yaml`](contracts/openapi/v1.yaml); the record values are the first row of the published export [`data/exports/2026.08.4-complete/regions.csv`](data/exports/2026.08.4-complete/regions.csv), re-stamped with the dataset version a locally seeded stack reports:

```json
{
  "data": [
    {
      "id": "01KDVDNA003K3M2XJ72RTWQE83",
      "countryCode": "GH",
      "name": "Upper East",
      "capital": "Bolgatanga",
      "status": "ACTIVE",
      "verificationStatus": "REFERENCE",
      "provenance": { "sourceId": "seed-bootstrap" },
      "datasetVersion": "2026.08.3-ulid"
    }
  ],
  "datasetVersion": "2026.08.3-ulid"
}
```

Every list envelope is `{ data, nextCursor?, datasetVersion }`. `nextCursor` is opaque: pass it back, never parse it.

### Read endpoints

| Endpoint | Returns | Cost class |
|---|---|---|
| `GET /v1/regions`, `/v1/regions/{id}`, `/v1/regions/{id}/districts` | Regions and their districts | cheap (1) |
| `GET /v1/districts`, `/v1/districts/{id}`, `/v1/districts/{id}/places` | MMDAs and their localities | cheap (1) |
| `GET /v1/places`, `/v1/places/{id}` | Localities with aliases, type, population | cheap (1) |
| `GET /v1/search?q=` | Typo-tolerant ranked candidates with `score` and `matchReason` | normal (2) |
| `GET /v1/autocomplete?q=` | Prefix suggestions for typeahead | normal (2) |
| `GET /v1/geocode` | Name → ranked coordinate candidates | normal (2) |
| `GET /v1/reverse` | Coordinate → containing geography | spatial (4) |
| `GET /v1/nearby` | Places near a coordinate | spatial (4) |
| `GET /v1/boundaries/{id}` | GeoJSON Polygon/MultiPolygon with attribution | geometry (5–10 by vertex count) |
| `GET /v1/roads`, `GET /v1/pois` | OpenStreetMap-derived roads and points of interest | cheap (1) |
| `GET /v1/datasets`, `/v1/datasets/{version}/downloads` | Published versions, artifact URLs and checksums | cheap (1) |

GraphQL is at `/graphql` (schema: [`contracts/graphql/schema.graphql`](contracts/graphql/schema.graphql)); gRPC speaks `ghanageo.v1` ([`proto/ghanageo/v1/geography.proto`](proto/ghanageo/v1/geography.proto)). All three share one application layer, so equivalent queries return semantically identical results — that is asserted by a cross-protocol conformance suite, not assumed.

Error codes are catalogued in [`contracts/errors/catalog.yaml`](contracts/errors/catalog.yaml) and documented one page each under [`docs/errors/`](docs/errors) — for example [`RATE_LIMIT_EXCEEDED.md`](docs/errors/RATE_LIMIT_EXCEEDED.md) and [`RADIUS_OUT_OF_RANGE.md`](docs/errors/RADIUS_OUT_OF_RANGE.md).

### SDKs and CLI

Seven official SDK lanes share one behaviour contract, [`docs/sdk-charter.md`](docs/sdk-charter.md), and are held to it by a language-neutral conformance suite in [`contracts/conformance/`](contracts/conformance). Each is verified locally; **none is published to a public registry yet** — publication is gated on registry credentials and on the V1 API launch.

| Language | Package / module id | Protocols | Local verification |
|---|---|---|---|
| TypeScript | `@ghanageo/client` (+ `core`, `react`, `node`, `data`, `proto`, umbrella `ghanageo`) | REST, GraphQL | `pnpm test`, `make conformance` |
| Python | `ghanageo` | REST | `make sdk-python-verify` |
| Go | `github.com/ghanageo/ghanageo-go` | REST, native gRPC | `make sdk-go-verify` |
| Dart | `ghanageo` (+ Flutter companion) | REST | `make sdk-dart-verify`, `make sdk-flutter-verify` |
| Java | `dev.ghanageo:ghanageo-java` (+ Spring Boot starter) | REST | `make sdk-java-verify` |
| .NET | `GhanaGeo` | REST | `make sdk-dotnet-verify` |
| PHP | `ghanageo/ghanageo-php` (+ Laravel integration) | REST | `make sdk-php-verify` |

`make sdk-matrix-verify` runs every lane. The Go CLI builds with `make cli` to `./bin/ghanageo`; see [`cli/README.md`](cli/README.md) and the npm bundle in [`packages/cli/README.md`](packages/cli/README.md).

---

## Data and provenance

Published exports live in [`data/exports/`](data/exports), one immutable directory per dataset version, in CSV and GeoJSON. The current complete release, `2026.08.4-complete`, contains:

| Entity | Rows | Notes |
|---|---|---|
| Regions | 16 | Every region has boundary geometry |
| Districts | 261 | **261 of 261 have boundary geometry** |
| Places | 15,924 | Populated places; district assignment where resolvable |
| Points of interest | 19,686 | 99.6% resolve to a district (19,601 of 19,686) |
| Roads | 19,728 | 99.5% resolve to a district |

### Sources and licence boundary

Recorded in full in [`docs/licensing-register.md`](docs/licensing-register.md). A source's licence is registered before its adapter is written.

| Source | Licence | Redistribution | Used for |
|---|---|---|---|
| geoBoundaries `gbOpen/GHA/ADM2` | CC BY 4.0 | Yes, with attribution | 248 district and 16 region boundaries (2019 vintage) |
| OpenStreetMap (Geofabrik Ghana) | ODbL 1.0 | Yes, with attribution and share-alike | The 13 post-2019 districts, plus roads and POIs |
| GeoNames | CC BY 4.0 | Yes, with attribution | Populated places (feature class `P`), landing as `REFERENCE` |
| Ghana Statistical Service | Official files; terms confirmed per dataset | Verify before publication | Canonical reconciliation — not yet ingested |
| **GhanaPostGPS** | **Proprietary — Ghana Post** | **Never** | **Nothing. No licence exists.** |

Apache-2.0 covers this repository's original code and configuration. It does **not** relicense any ingested dataset: upstream terms always govern, which is why ODbL attribution is written into the records and the export files rather than into a page footer.

### Versioning and corrections

Dataset versions are immutable, checksummed and independent of code releases — the seed manifest ([`data/seed-data/manifest.json`](data/seed-data/manifest.json)) carries a SHA-256 for every file — and publication moves through `draft → validation → review → approved → published`, with rollback available, all permission-gated and audited.

To propose a correction, open an issue or pull request with the stable identifier, the current value, the proposed value, the authoritative source, the source publication date and whether history changes. Automation may draft; a human steward approves canonical publication. See [`CONTRIBUTING.md`](CONTRIBUTING.md).

---

## Project layout

```text
geoghana/
├── apps/
│   ├── web/            # marketing site + documentation hub      → geo.digitalghana.dev
│   ├── sandbox/        # anonymous playground, all 3 protocols   → sandbox-geo.digitalghana.dev
│   ├── portal/         # authenticated developer console         → console-geo.digitalghana.dev
│   └── admin/          # admin + data-steward portal             → admin-geo.digitalghana.dev
├── services/
│   ├── api/            # Go modular monolith: REST + GraphQL + gRPC, ports/adapters
│   └── worker/         # transactional-outbox consumer, ETL, export, search index
├── packages/           # @ghanageo/{core,client,react,node,data,proto,ui,config} + the `ghanageo` npm CLI
├── sdks/               # python · go · dart(+flutter) · java(+spring) · dotnet · php(+laravel)
├── cli/                # Go source for the `ghanageo` terminal client
├── contracts/          # openapi/v1.yaml · graphql/schema.graphql · errors/catalog.yaml · conformance/
├── proto/              # ghanageo/v1/geography.proto — protobuf source of truth
├── data/               # seed-data/ (bootstrap CSVs + manifest) and exports/ (published versions)
├── docs/               # licensing register, boundary coverage, SLOs, security, runbooks, evidence, errors
├── infra/vercel/       # per-app Vercel deployment configuration
├── scripts/            # gen-env, dev, conformance runners, release and preflight tooling
├── tools/conformance/  # language-neutral SDK conformance runner kit
└── .github/workflows/  # quality.yml · security.yml · cli-release.yml · sdk-*.yml
```

Root-level `render.yaml`, `docker-compose.yml` and `Makefile` define the production blueprint, the local stack and every verification entry point respectively. Directory ownership is assigned by lane in [`AGENTS.md`](AGENTS.md); operating rules for automated contributors are in [`CLAUDE.md`](CLAUDE.md) and the UI specification is [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md).

---

## Verification

```sh
make test                 # go test ./... in both services, then pnpm test
make lint                 # go vet ./... in both services, then pnpm lint
pnpm typecheck            # turbo run typecheck across the workspace
pnpm build                # turbo run build

make contracts            # lint OpenAPI, protobuf and the GraphQL SDL; regenerate error artifacts
make conformance          # SDK contract, runner kit and TypeScript reference adapter
make sdk-matrix-verify    # every SDK lane, plus the cross-language conformance matrix
make docs-sdk-check       # reject generated SDK documentation drift; compile every extracted example
make check-licensed-data  # fail on any GhanaPostGPS payload in distributable data
pnpm check:links          # internal route and link audit

# these need a running stack
make perf                 # p95 acceptance gate; requires PERF_API_KEY
make perf-indexes         # assert every hot query is an IXSCAN, never a COLLSCAN
make restore-drill        # prove Mongo backup and isolated scratch restoration
```

CI runs the same commands. [`.github/workflows/quality.yml`](.github/workflows/quality.yml) gates Go, contracts, every SDK lane, the conformance matrix, SDK docs, the web build and the design-QA matrix; [`.github/workflows/security.yml`](.github/workflows/security.yml) runs dependency audits, CodeQL and the licensed-data exclusion check.

---

## Status and roadmap

**Lifecycle: externally blocked.** V1 implementation and local verification are substantially complete and the four frontends are live in production. The public API is not deployed, and that is the single blocking item — it needs production Redis, Typesense (or Atlas Search) and the remaining provider values, all of which are external state rather than engineering work.

[`ROADMAP.md`](ROADMAP.md) sets out what has shipped, the open gates with their blocking dependencies, deferred scope and what is explicitly out of scope. It is directional, not a commitment; the machine-readable state lives in [`agent_plan.md`](agent_plan.md).

---

## Contributing and policy

- [`CONTRIBUTING.md`](CONTRIBUTING.md) — prerequisites, local setup, verification, commit and PR conventions, data corrections, good first contributions.
- [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) — Contributor Covenant v2.1; reports go privately to the private channel documented there.
- [`SECURITY.md`](SECURITY.md) — vulnerability reporting to the private channel documented there, scope and release gates. Do not open a public issue containing exploit details.
- [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE) — Apache License 2.0, inbound = outbound. Ingested datasets keep their own terms.
- [`docs/slo.md`](docs/slo.md) — the service-level objectives that apply once the public service is stabilised.

---

## Independence

GhanaGeo is an independent open-source project. It is **not operated by, endorsed by, or affiliated with the Government of Ghana**, the Ghana Statistical Service, Ghana Post or any ministry, agency or public institution. Records that reference official sources do so by citation only, with the source and retrieval date attached. Nothing published here should be treated as an official record, as a government code, or as evidence of official status.
