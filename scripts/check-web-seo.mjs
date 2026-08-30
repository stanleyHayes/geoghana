#!/usr/bin/env node

const base = new URL(process.env.GHANAGEO_WEB_AUDIT_URL ?? "http://127.0.0.1:3100");
const expectedOrigin = (process.env.GHANAGEO_EXPECTED_ORIGIN ?? base.origin).replace(/\/$/, "");
const expectIndexable = process.env.GHANAGEO_EXPECT_INDEXABLE === "true";
const routes = ["/", "/products", "/developers", "/coverage", "/support", "/docs", "/changelog", "/status", "/about", "/contact", "/transparency"];
const failures = [];

function match(html, pattern) {
  return pattern.test(html);
}

for (const route of routes) {
  const response = await fetch(new URL(route, base), { redirect: "error" }).catch((error) => ({ error }));
  if (response.error || !response.ok) {
    failures.push(`${route}: expected HTTP 200${response.error ? ` (${response.error.message})` : `, got ${response.status}`}`);
    continue;
  }
  const html = await response.text();
  const canonical = `${expectedOrigin}${route === "/" ? "" : route}`;
  const checks = [
    ["title", /<title>[^<]{8,70}<\/title>/i],
    ["description", /<meta[^>]+name=["']description["'][^>]+content=["'][^"']{40,180}["']/i],
    ["canonical", new RegExp(`<link[^>]+rel=["']canonical["'][^>]+href=["']${canonical.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/?["']`, "i")],
    ["Open Graph title", /<meta[^>]+property=["']og:title["'][^>]+content=/i],
    ["Open Graph description", /<meta[^>]+property=["']og:description["'][^>]+content=/i],
    ["Open Graph image", /<meta[^>]+property=["']og:image["'][^>]+content=/i],
    ["Twitter card", /<meta[^>]+name=["']twitter:card["'][^>]+content=["']summary_large_image["']/i],
    [expectIndexable ? "index directive" : "noindex directive", new RegExp(`<meta[^>]+name=["']robots["'][^>]+content=["'][^"']*${expectIndexable ? "index, follow" : "noindex, nofollow"}`, "i")],
  ];
  for (const [label, pattern] of checks) if (!match(html, pattern)) failures.push(`${route}: missing or invalid ${label}`);

  if (route === "/") {
    for (const type of ["Organization", "WebSite", "SoftwareApplication"]) {
      if (!html.includes(`\"@type\":\"${type}\"`)) failures.push(`/: missing ${type} JSON-LD`);
    }
  }
}

const robots = await fetch(new URL("/robots.txt", base)).then((response) => response.text());
if (expectIndexable) {
  if (!robots.includes("Allow: /") || !robots.includes(`${expectedOrigin}/sitemap.xml`)) failures.push("robots.txt: production crawl or sitemap directive is invalid");
  const sitemap = await fetch(new URL("/sitemap.xml", base)).then((response) => response.text());
  for (const route of routes) {
    const canonical = `${expectedOrigin}${route === "/" ? "" : route}`;
    if (!sitemap.includes(`<loc>${canonical}</loc>`)) failures.push(`sitemap.xml: missing ${canonical}`);
  }
} else if (!robots.includes("Disallow: /")) {
  failures.push("robots.txt: non-production deployment must disallow crawling");
}

if (failures.length) {
  console.error(`SEO audit failed (${failures.length}):\n- ${failures.join("\n- ")}`);
  process.exit(1);
}
console.log(`SEO audit passed for ${routes.length} routes at ${base.origin} (${expectIndexable ? "indexable" : "noindex"}).`);
