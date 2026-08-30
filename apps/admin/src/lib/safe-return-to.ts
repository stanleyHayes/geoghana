/** Resolve an in-app destination without allowing URL parser backslash tricks. */
export function safeReturnTo(value: string | null | undefined, origin: string): string {
  if (!value) return "/";

  let decoded = value;
  try {
    for (let pass = 0; pass < 2; pass += 1) {
      const next = decodeURIComponent(decoded);
      if (next === decoded) break;
      decoded = next;
    }
  } catch {
    return "/";
  }

  if (decoded.includes("\\") || !decoded.startsWith("/") || decoded.startsWith("//")) return "/";

  try {
    const base = new URL(origin);
    const destination = new URL(decoded, base);
    if (destination.origin !== base.origin || destination.username || destination.password) return "/";
    return `${destination.pathname}${destination.search}${destination.hash}`;
  } catch {
    return "/";
  }
}
