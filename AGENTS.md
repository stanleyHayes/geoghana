# AGENTS.md — lane map and coordination

Companion to [`CLAUDE.md`](CLAUDE.md). Full detail in
[`agent_plan.md`](agent_plan.md) §8.

## Lanes and exclusive ownership

Each lane owns its directories outright. Two agents never edit the same file
concurrently.

| Lane | Name | Owns |
|---|---|---|
| L0 | Contracts & Foundation | `contracts/`, `proto/`, `packages/config/`, root CI, this file |
| L1 | Data & ETL | `data/`, `services/api/migrations/`, `services/worker/internal/etl/` |
| L2 | Domain & Use Cases | `internal/domain/`, `internal/app/`, `internal/ports/` |
| L3 | Transports | `internal/transport/` |
| L4 | Identity & Gateway | `internal/platform/`, `internal/domain/identity/` |
| L5 | Search & Spatial | `internal/adapters/search/`, `internal/app/search/` |
| L6 | SDKs | `packages/core|client|react|node|data|proto/` |
| L7 | Design System | `packages/ui/` |
| L8 | Marketing & Docs | `apps/web/` |
| L9 | Sandbox | `apps/sandbox/` |
| L10 | Developer Portal | `apps/portal/` |
| L11 | Admin | `apps/admin/` |
| L12 | Platform Ops | `infra/`, `docs/runbooks/` |

## Shared files, edited centrally by their owner only

- `apps/admin/src/config/navigation.ts` — **L11**. Request an entry; do not
  edit concurrently.
- `services/api/cmd/api/main.go` (DI wiring) — **L0**.
- `packages/ui/src/styles/tokens.css` — **L7**. An app never redefines a token
  locally; a needed token is a request to L7.
- `contracts/` and `proto/` — **L0**, always in a separate PR.

## Coordination protocol

1. Claim your story on the task board in `agent_plan.md` §1a before starting.
2. Build against a dependency's published **contract**, never its live
   database or internal code.
3. Preserve unrelated in-flight work in a shared worktree. Leave a
   coordination note rather than reverting someone else's changes.
4. Update the task board when your story changes state.
