/**
 * Resolve the GhanaGeo API base URL, preferring the local API whenever the
 * page itself is served from localhost.
 *
 * `NEXT_PUBLIC_*` is inlined at BUILD time, and `next build` runs in
 * production mode — so every production build bakes in `.env.production`,
 * which points at api-geo.digitalghana.dev. That domain does not exist yet,
 * so a locally served production build could never reach the API and every
 * screen reading it showed "unreachable" regardless of what was running.
 * The URL was decided before the machine it would run on was known.
 *
 * Being served from localhost is unambiguous — nobody reaches a developer's
 * laptop through the production hostname — so this cannot misfire in
 * production, where `window.location.hostname` is the real domain.
 */
const LOCAL_DEFAULT = "http://localhost:8180/v1";

export function resolveApiBase(configured: string | undefined): string {
  const base = configured ?? LOCAL_DEFAULT;

  // Server-side render has no window; the configured value is all there is.
  if (typeof window === "undefined") return base;

  const host = window.location.hostname;
  if (host !== "localhost" && host !== "127.0.0.1" && host !== "[::1]") return base;

  try {
    // A deliberate local override — a different port, say — is left exactly
    // as the developer set it. Only a remote host gets redirected.
    const configuredHost = new URL(base).hostname;
    if (configuredHost === "localhost" || configuredHost === "127.0.0.1") return base;
  } catch {
    /* an unparseable value falls through to the local default */
  }
  return LOCAL_DEFAULT;
}
