#!/usr/bin/env node
/**
 * Link integrity across the web surface.
 *
 * This exists because /docs shipped as an unreachable page: the route built
 * fine and returned 200, but no link anywhere pointed at it, so nothing in the
 * build, the typechecker or the tests noticed. A 404 is loud. An orphan is
 * silent, which makes it the more dangerous of the two.
 *
 * Two failures, both fatal:
 *   DEAD   — a link points at a route that does not exist.
 *   ORPHAN — a route exists that nothing links to.
 *
 * Run:  node scripts/check-links.mjs
 */
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const ROOT = new URL("..", import.meta.url).pathname.replace(/\/$/, "");

/** Shared components live in packages/ui but their links resolve against web. */
const APPS = [
  { name: "web", dir: "apps/web", extraSrc: ["packages/ui/src"] },
  { name: "sandbox", dir: "apps/sandbox", extraSrc: [] },
  {
    name: "portal",
    dir: "apps/portal",
    extraSrc: [],
    /* These are reached from identity-provider emails or a redirect carrying
       a short-lived token. Putting them in public navigation would be both
       misleading and, for reset-password, an invalid way to enter the flow. */
    implicitEntries: new Map([
      ["/reset-password", "password-reset email"],
      ["/verify-email", "email-verification link"],
    ]),
  },
  {
    name: "admin",
    dir: "apps/admin",
    extraSrc: [],
    /* Admin's sidebar is the planned information architecture: the nav was
       designed as a whole so the screens can be built against a fixed map.
       Unbuilt entries there are a backlog, not a bug. Dead links from anywhere
       ELSE in admin still fail — a rendered page linking into the void is the
       real defect this check is for. */
    plannedNav: "src/config/navigation.ts",
    /* Authentication middleware and authorization failures redirect here.
       They are entry/exit points in the auth state machine, not nav items. */
    implicitEntries: new Map([
      ["/login", "unauthenticated redirect"],
      ["/access-denied", "authorization-failure redirect"],
    ]),
  },
];

/** Routes that exist but are deliberately not linked from anywhere. */
const ALLOWED_ORPHANS = new Set([
  "/", // the root is reached by the domain itself
]);

function findOrphans(routes, linked, implicitEntries = new Map()) {
  return [...routes].filter((route) =>
    !linked.has(route)
    && !ALLOWED_ORPHANS.has(route)
    && !implicitEntries.has(route)
    // Dynamic segments are reached through interpolated hrefs, which cannot
    // be resolved statically by this checker.
    && !route.includes("[")
  );
}

/* Keep the exemption mechanism honest: an explicitly modeled auth entry may
   be unlinked, but a neighboring ordinary route must still be reported. */
{
  const syntheticRoutes = new Set(["/", "/login", "/dashboard", "/forgotten"]);
  const syntheticLinks = new Map([["/dashboard", "fixture.tsx"]]);
  const found = findOrphans(
    syntheticRoutes,
    syntheticLinks,
    new Map([["/login", "authentication redirect"]]),
  );
  if (found.length !== 1 || found[0] !== "/forgotten") {
    throw new Error("link checker regression: implicit entries hid an ordinary orphan");
  }
}

function walk(dir, out = []) {
  let entries;
  try { entries = readdirSync(dir); } catch { return out; }
  for (const e of entries) {
    if (e === "node_modules" || e === ".next" || e === "dist") continue;
    const p = join(dir, e);
    if (statSync(p).isDirectory()) walk(p, out);
    else out.push(p);
  }
  return out;
}

/** apps/web/src/app/about/page.tsx -> /about */
function routeOf(file, appDir) {
  const rel = relative(join(ROOT, appDir, "src/app"), file);
  const segs = rel.split("/").slice(0, -1)
    .filter((s) => !(s.startsWith("(") && s.endsWith(")"))); // route groups
  return "/" + segs.join("/");
}

/* Both JSX attributes (href="/docs") and object literals (href: "/docs"),
   because shared nav is declared as data — PRIMARY_NAV et al — and a checker
   that only reads JSX would report every one of those routes as an orphan. */
const HREF = /href\s*[=:]\s*(?:"([^"]+)"|\{`([^`]+)`\}|\{"([^"]+)"\}|`([^`$]+)`)/g;

let dead = 0, orphan = 0, planned = 0;

for (const app of APPS) {
  const appRoot = join(ROOT, app.dir);
  const routes = new Set(
    walk(join(appRoot, "src/app"))
      .filter((f) => /\/page\.tsx$/.test(f))
      .map((f) => routeOf(f, app.dir)),
  );
  if (routes.size === 0) continue;

  const sources = [
    ...walk(join(appRoot, "src")),
    ...app.extraSrc.flatMap((s) => walk(join(ROOT, s))),
  ].filter((f) => /\.(tsx|ts)$/.test(f));

  const linked = new Map(); // route -> first file that links it
  const implicitEntries = app.implicitEntries ?? new Map();

  // A stale exemption is almost as dangerous as a broad ignore: it can mask
  // a typo in this production gate. Every modeled entry must be a real route.
  for (const [route, reason] of implicitEntries) {
    if (!routes.has(route)) {
      console.error(`CONFIG ${app.name}: implicit entry ${route} (${reason}) does not exist`);
      dead++;
    }
  }

  for (const f of sources) {
    const text = readFileSync(f, "utf8");
    for (const m of text.matchAll(HREF)) {
      const raw = m[1] ?? m[2] ?? m[3] ?? m[4] ?? "";
      if (!raw.startsWith("/")) continue;            // external or anchor
      const path = raw.split(/[?#]/)[0].replace(/\/$/, "") || "/";
      if (!linked.has(path)) linked.set(path, relative(ROOT, f));
      // A href built from a template (`/places/${id}`) resolves at runtime;
      // only fully literal paths can be checked here.
      if (raw.includes("${")) continue;
      if (!routes.has(path)) {
        if (app.plannedNav && f.endsWith(app.plannedNav)) { planned++; continue; }
        console.error(`DEAD   ${app.name}: ${relative(ROOT, f)} -> ${path}`);
        dead++;
      }
    }
  }

  for (const r of findOrphans(routes, linked, implicitEntries)) {
    console.error(`ORPHAN ${app.name}: ${r} exists but nothing links to it`);
    orphan++;
  }

  const note = app.plannedNav && planned ? `, ${planned} planned (not yet built)` : "";
  const implicitNote = implicitEntries.size ? `, ${implicitEntries.size} implicit auth entries` : "";
  console.log(`  ${app.name}: ${routes.size} routes, ${linked.size} linked${implicitNote}${note}`);
}

if (dead || orphan) {
  console.error(`\n✗ ${dead} dead link(s), ${orphan} orphan route(s)`);
  process.exit(1);
}
console.log("\n✓ every route is reachable and every internal link resolves");
