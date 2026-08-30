# SEO and production-readiness evidence — 2026-08-30

## Implemented

- Every one of the 11 public marketing routes emits a unique title,
  description, canonical URL, Open Graph payload and Twitter card.
- Canonical, sandbox and portal origins come from validated public environment
  variables; production markup no longer links to localhost.
- `Organization`, `WebSite` and `SoftwareApplication` JSON-LD use the canonical
  origin. The software application declares a zero-price GHS `Offer`, as
  required for Google's software-app eligibility.
- `NEXT_PUBLIC_GHANAGEO_INDEXABLE` is an explicit launch switch. Production
  emits index/follow, a complete canonical sitemap and an allowing robots file;
  preview/staging emits noindex/nofollow, `Disallow: /` and an empty sitemap.
- A web manifest and baseline browser security headers are emitted. The
  framework-identifying `X-Powered-By` header is disabled.
- `scripts/check-web-seo.mjs` audits the final HTTP output rather than trusting
  source declarations.

## Verification

The following passed against optimized Next.js 16.3.3 production builds:

```text
pnpm --filter @ghanageo/web typecheck
pnpm --filter @ghanageo/web build
pnpm check:links
git diff --check

SEO audit passed for 11 routes (indexable).
SEO audit passed for 11 routes (noindex).

Lighthouse desktop, indexable production build:
performance: 93
accessibility: 100
best-practices: 100
seo: 100
largest-contentful-paint: 3.2 s
cumulative-layout-shift: 0
total-blocking-time: 30 ms
```

The throttled local LCP is not yet green, so this evidence does not claim that
the Core Web Vitals acceptance criterion is complete. Field Core Web Vitals
also require a public origin and real traffic.

## External launch gates observed on 2026-08-30

DNS returned no A records and HTTPS returned no response for:

- `geo.digitalghana.dev`
- `api.geo.digitalghana.dev`
- `sandbox.geo.digitalghana.dev`
- `console.geo.digitalghana.dev`
- `admin.geo.digitalghana.dev`

After deployment and TLS are live, set the canonical marketing environment's
indexability switch to true, run the public-host command in
`docs/runbooks/deployment.md`, submit the sitemap in Search Console, and monitor
field Core Web Vitals. Do not enable indexing on preview or staging projects.
