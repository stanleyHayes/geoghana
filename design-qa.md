# GhanaGeo Design QA

## 2026-08-29 — V1 accessibility and responsive matrix

### Gate

The required matrix covers all four frontend surfaces across three materials (neumorphic, glassmorphic and claymorphic), three themes (light, dark and a custom brand hue), and five viewports (320, 390, 768, 1280 and 1920 pixels).

That is 45 checks per surface and 180 checks for the V1 suite. Every check requires a successful page response, resolved theme, `main` and `h1` landmarks, no horizontal body overflow, no page or network errors, no HTTP 5xx response and zero axe WCAG A/AA violations.

### Fixes

- Corrected the shared light/dark/custom semantic and brand contrast tokens, including the runtime theme clamp that supplies inline CSS variables.
- Removed the unnamed sandbox home link and the duplicate admin breadcrumb key.
- Corrected marketing proof/map labels, portal status colors and the admin notification badge in both modes.
- Suppressed only the expected sandbox server/client API-origin text hydration difference while continuing to fail genuine page and request errors.
- Made the harness await webfonts and theme settlement, report failed requests and HTTP 5xx responses, and support focused reruns.
- Added the full production-build matrix to `.github/workflows/quality.yml` as the `Design QA (180-point matrix)` frontend check.

### Verification evidence

- `pnpm --filter @ghanageo/web build` and 45 production matrix checks — passed.
- `pnpm --filter @ghanageo/sandbox build` and 45 production matrix checks — passed.
- `NEXT_PUBLIC_GHANAGEO_API_URL=http://localhost:8180/v1 pnpm --filter @ghanageo/portal build` and 45 production matrix checks — passed.
- `pnpm --filter @ghanageo/admin build` and 45 production matrix checks — passed.
- Aggregate result: 180/180 checks passed with zero axe violations, console errors, request failures, theme-resolution failures or body overflow.

The portal production test intentionally embeds the local API URL at build time. A build made with the public production URL correctly failed local QA on DNS request errors, proving the harness does not hide unreachable dependencies.

final result: passed
