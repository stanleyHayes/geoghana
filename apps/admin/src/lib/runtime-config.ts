const PRODUCTION_API_ORIGIN = "https://api-geo.digitalghana.dev";
const PRODUCTION_WEB_ORIGIN = "https://geo.digitalghana.dev";
const PRODUCTION_PORTAL_ORIGIN = "https://console-geo.digitalghana.dev";

function validatedOrigin(value: string | undefined, fallback: string, production: boolean): string {
  if (!value) return fallback;
  try {
    const url = new URL(value);
    const protocolAllowed = production ? url.protocol === "https:" : ["http:", "https:"].includes(url.protocol);
    if (!protocolAllowed || url.username || url.password) return fallback;
    return url.origin;
  } catch {
    return fallback;
  }
}

export function adminApiOrigin(env: NodeJS.ProcessEnv = process.env): string {
  const production = env.NODE_ENV === "production";
  const fallback = production ? PRODUCTION_API_ORIGIN : "http://localhost:8180";
  return validatedOrigin(env.GHANAGEO_API_ORIGIN ?? env.NEXT_PUBLIC_GHANAGEO_API_URL, fallback, production);
}

export function publicWebOrigin(env: NodeJS.ProcessEnv = process.env): string {
  const production = env.NODE_ENV === "production";
  const fallback = production ? PRODUCTION_WEB_ORIGIN : "http://localhost:3100";
  return validatedOrigin(env.NEXT_PUBLIC_GHANAGEO_WEB_URL, fallback, production);
}

export function publicWebHref(pathname: string, env: NodeJS.ProcessEnv = process.env): string {
  const path = pathname.startsWith("/") && !pathname.startsWith("//") ? pathname : "/";
  return new URL(path, publicWebOrigin(env)).toString();
}

export function publicPortalHref(pathname: string, env: NodeJS.ProcessEnv = process.env): string {
  const production = env.NODE_ENV === "production";
  const fallback = production ? PRODUCTION_PORTAL_ORIGIN : "http://localhost:3102";
  const origin = validatedOrigin(env.NEXT_PUBLIC_GHANAGEO_PORTAL_URL, fallback, production);
  const path = pathname.startsWith("/") && !pathname.startsWith("//") ? pathname : "/";
  return new URL(path, origin).toString();
}
