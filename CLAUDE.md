# CLAUDE.md — AI operating procedures for GhanaGeo

**Read before writing any code.** This file, `AGENTS.md`, [`agent_plan.md`](agent_plan.md)
and [`DESIGN_SYSTEM.md`](DESIGN_SYSTEM.md) together define how work happens here.
Required by the AI Development Workflow Training Manual, "CLAUDE.md and AGENTS.md".

## The five rules that get a PR rejected on sight

1. **No GhanaPostGPS digital-address data.** Not scraped, not stored, not
   redistributed, not in a fixture. Integration exists only as an adapter
   interface behind a licence gate. CI fails on any such payload.
2. **No business logic in a transport.** REST handlers, GraphQL resolvers and
   gRPC services validate wire format, call a use case, map the result back.
   A domain `if` inside a handler is a bug.
3. **No bson tags in `internal/domain`.** Persistence mapping lives in
   `internal/adapters/mongo` so the storage shape can change freely.
4. **No hardcoded colour, shadow, radius or blur in a component.** Tokens only.
   A `material === "glass"` branch defeats the entire design system.
5. **No secret in source or a client bundle.** Config comes from the
   environment. CI secret-scans every commit.

## Local setup

```bash
make up      # MongoDB (rs0) + Redis + Typesense, on dedicated ports
make seed    # 16 regions / 261 districts / 16 places, idempotent
make api     # REST v1 on :8180
pnpm --filter @ghanageo/admin dev   # admin shell on :3103
```

This machine runs several projects, so GhanaGeo uses a dedicated port block:
Mongo `27117`, Redis `6679`, Typesense `8108`, API `8180`, gRPC `9190`,
admin `3103`. Check a port is free before claiming a new one.

## Workflow

| Step | Rule |
|---|---|
| Branch | `feature/GEO-12.3-short-slug` |
| Commit | `GEO-12.3 imperative summary` |
| PR | `GEO-12.3 Title` — the only merge path |
| Jira | Lead → Discovery → Requirements → Solution Design → Backlog Grooming → Sprint Planning → Development → Code Review → QA → Staging → UAT → Beta → Production → Sign Off → Support → Closed |

Every story carries: User Story · Business Value · Acceptance Criteria ·
Technical Notes · Definition of Done · Estimates · Dependencies.

## Before claiming a task is done

- `cd services/api && go build ./... && go vet ./... && go test ./...`
- `pnpm typecheck && pnpm test && pnpm --filter @ghanageo/admin build`
- Frontend: render it in all three materials × light/dark and check the
  `DESIGN_SYSTEM.md` §23 Design-QA gates. **Verify in a browser, not by
  reading the code** — the CORS gap and the nested-dark-selector bug were
  both invisible in review and obvious in Playwright.
- Update the task board in `agent_plan.md` §1a.

## Things that have already bitten us

- `next start` shows up as `next-server`, so `pkill -f "next start"` misses it
  and you end up testing a stale build. Kill by port.
- A nested element carrying `data-material` inherits `data-mode` from `<html>`,
  so dark blocks need the descendant selector form as well.
- The `latin` font subset drops the Latin Extended characters Twi, Ga and Ewe
  need. Always request `latin-ext`.
- MongoDB accepts a self-intersecting polygon and then returns silently wrong
  `$geoIntersects` results. Geometry validity is a blocking Go-layer gate.
