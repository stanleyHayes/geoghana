function isLoopback(hostname: string): boolean {
  return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "[::1]";
}

export function resolvePublicOrigin(value: string | undefined, fallback: string, development = false): string {
  try {
    const url = new URL(value ?? fallback);
    const loopback = isLoopback(url.hostname);
    const securePublic = url.protocol === "https:" && !loopback;
    const localDevelopment = development && url.protocol === "http:" && loopback;
    const exactOrigin = url.pathname === "/" && !url.search && !url.hash;
    if (!exactOrigin || (!securePublic && !localDevelopment) || url.username || url.password) throw new Error("unsafe public origin");
    return url.origin;
  } catch {
    return new URL(fallback).origin;
  }
}

const development = process.env.NODE_ENV === "development";

export const webOrigin = resolvePublicOrigin(
  process.env.NEXT_PUBLIC_GHANAGEO_WEB_URL,
  development ? "http://localhost:3100" : "https://geo.digitalghana.dev",
  development,
);

export const portalOrigin = resolvePublicOrigin(
  process.env.NEXT_PUBLIC_GHANAGEO_PORTAL_URL,
  development ? "http://localhost:3102" : "https://console-geo.digitalghana.dev",
  development,
);
