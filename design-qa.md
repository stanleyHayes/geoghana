# GhanaGeo Design QA

## 2026-08-29 — Shared primitive and matrix-gate run

### Evidence

- `pnpm --filter @ghanageo/ui typecheck` — passed.
- Web, sandbox, portal and admin TypeScript checks — passed.
- `pnpm --filter @ghanageo/web build` — passed after the shared stylesheet repair.
- `pnpm --filter @ghanageo/ui design:qa` — executed 180 combinations across four surfaces, three materials, three themes and five viewports.
- Representative rerun: glass, light, 390px across all four surfaces — all pages rendered after restarting stale development compilers; axe findings remained.

### Findings

- **P1 accessibility:** secondary and small accent text has insufficient contrast on existing web, sandbox, portal and admin screens. Representative counts: web 18 nodes, sandbox 4, portal 1, admin 19.
- **P1 accessibility:** one sandbox link has no accessible name.
- **P2 correctness:** the admin home route logs a duplicate React key for `/`.
- **P2 delivery:** the matrix harness is available as `@ghanageo/ui` script `design:qa`, but root required-CI wiring belongs to L0 and is not changed from the L7 lane.

### Fixes in this run

- Added the missing shared primitive inventory for GEO-14.3.
- Added the Playwright + axe material/theme/viewport matrix harness.
- Fixed an unbalanced shared form-control CSS rule exposed by the first matrix run.
- Added focused-matrix environment controls so a failing combination can be reproduced quickly before a full rerun.

### Post-fix evidence

- Shared UI and all four consuming application typechecks pass.
- Web production build compiles the repaired shared stylesheet.
- The representative matrix reaches all four applications and reports product-level accessibility findings rather than compiler failures.

final result: blocked

Blocked on the recorded cross-lane accessibility fixes and L0 required-CI wiring. GEO-14.7 must remain partial until the full matrix returns zero violations.
