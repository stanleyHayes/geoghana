"use client";

import { usePathname, useRouter } from "next/navigation";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  CommandPalette,
  Logo,
  Navbar,
  resolveApiBase,
  Sidebar,
  SkipLink,
  type Crumb,
  type PlaceResult,
  type Role,
} from "@ghanageo/ui";
import { Box, LogOut, MapPin } from "lucide-react";
import { NAVIGATION } from "@/config/navigation";
import { useSession } from "@/components/session";
import { getAdminDashboard, getAdminSystemHealth } from "@/lib/admin-api";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);

/**
 * The persistent admin shell.
 *
 * This lives in the LAYOUT, not in a page. Previously the whole admin was one
 * page holding `pathname` in React state: clicking a rail item called
 * setPathname, which moved the active highlight and nothing else — the rail
 * lit up and the content never changed. Routing state belongs to the router.
 *
 * Sidebar renders real anchors and calls preventDefault only when `onNavigate`
 * is supplied, so passing router.push here keeps client-side navigation while
 * middle-click, ⌘-click and "open in new tab" still work off the href.
 */
export function AdminShell({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const { session, signOut } = useSession();
  const role = session?.role as Role;
  const [operations, setOperations] = useState<{
    datasetVersion: string;
    pipelineHealth: "ok" | "lagging" | "failing";
  }>({ datasetVersion: "Not recorded", pipelineHealth: "lagging" });

  useEffect(() => {
    if (!session) return;
    const controller = new AbortController();
    Promise.all([
      getAdminDashboard(controller.signal),
      getAdminSystemHealth(controller.signal),
    ])
      .then(([dashboard, health]) => {
        setOperations({
          datasetVersion: dashboard.currentRelease?.version ?? "Not recorded",
          pipelineHealth:
            health.status === "healthy"
              ? "ok"
              : health.status === "degraded"
                ? "lagging"
                : "failing",
        });
      })
      .catch(() => {
        /* The page-level views surface quotable API errors. */
      });
    return () => controller.abort();
  }, [session]);

  const navigate = useCallback(
    (href: string) => {
      router.push(href);
      setMobileOpen(false);
    },
    [router],
  );

  // Real search against the running API, with AbortSignal propagation so an
  // abandoned query cancels its network work.
  const search = useCallback(
    async (q: string, signal: AbortSignal): Promise<PlaceResult[]> => {
      const res = await fetch(
        `${API}/places?q=${encodeURIComponent(q)}&limit=8`,
        { signal },
      );
      if (!res.ok) return [];
      const body = await res.json();
      return (body.data ?? []).map((p: Record<string, unknown>) => ({
        id: p.id as string,
        name: p.name as string,
        type: p.type as string,
        regionName: (p.region as { name?: string } | undefined)?.name,
        districtName: (p.district as { name?: string } | undefined)?.name,
        verificationStatus: p.verificationStatus as string,
      }));
    },
    [],
  );

  /* Breadcrumbs come from the nav registry rather than from the URL segments,
     so the trail reads "Geography / Districts / MMDAs" and not "geography /
     districts". The registry is already the source of truth for the rail. */
  const crumbs = useMemo<Crumb[]>(() => {
    const trail: Crumb[] = [{ label: "Ghana", href: "/" }];
    if (pathname === "/") return [{ label: "Overview", href: "/" }];
    const matchedItem = NAVIGATION.flatMap((group) =>
      group.items.map((item) => ({ group, item })),
    )
      .filter(
        ({ item }) =>
          pathname === item.href || pathname.startsWith(`${item.href}/`),
      )
      .sort((a, b) => b.item.href.length - a.item.href.length)[0];
    if (matchedItem) {
      const { group, item } = matchedItem;
      /* The group crumb points at the group's FIRST item, not at the current
           one. A group has no landing route of its own, and giving both crumbs
           the same href made "Geography > Regions" two entries with an
           identical key — React drops one. It also makes the crumb useful:
           clicking the group goes somewhere real. */
      const first = group.items[0];
      if (group.label && first && first.href !== item.href) {
        trail.push({ label: group.label, href: first.href });
      }
      trail.push({ label: item.label, href: item.href });
      if (pathname !== item.href)
        trail.push({ label: "Details", href: pathname });
      return trail;
    }
    return [...trail, { label: "Not found", href: pathname }];
  }, [pathname]);

  const routeAccess = useMemo(() => {
    if (pathname === "/") return { matched: true, allowed: true };
    const items = NAVIGATION.flatMap((group) => group.items)
      .filter(
        (item) =>
          pathname === item.href || pathname.startsWith(`${item.href}/`),
      )
      .sort((a, b) => b.href.length - a.href.length);
    const matched = items[0];
    return {
      matched: Boolean(matched),
      allowed: matched?.roles.includes(role) ?? false,
    };
  }, [pathname, role]);

  return (
    <div className={`gg-shell ${collapsed ? "gg-shell--collapsed" : ""}`}>
      <SkipLink />

      <Sidebar
        groups={NAVIGATION}
        role={role}
        pathname={pathname}
        collapsed={collapsed}
        mobileOpen={mobileOpen}
        onMobileClose={() => setMobileOpen(false)}
        onNavigate={navigate}
        brand={<Logo size={22} showWordmark={!collapsed} suffix="Admin" />}
        footer={
          !collapsed && session ? (
            <div className="admin-sidebar-footer">
              <div
                className="admin-sidebar-context"
                aria-label="Workspace context"
              >
                <span>
                  <MapPin aria-hidden />
                  {process.env.NODE_ENV === "production"
                    ? "Production"
                    : "Local workspace"}
                </span>
                <a href="/releases/versions">
                  <Box aria-hidden />
                  {operations.datasetVersion === "Not recorded"
                    ? "No published release"
                    : operations.datasetVersion}
                </a>
              </div>
              <div className="admin-identity">
                <span className="admin-identity__email">{session.email}</span>
                <span className="admin-identity__role">
                  {session.role.replaceAll("_", " ").toLowerCase()}
                </span>
              </div>
            </div>
          ) : null
        }
      />

      <div className="gg-shell__content">
        <Navbar
          crumbs={[]}
          collapsed={collapsed}
          onToggleCollapse={() => setCollapsed((c) => !c)}
          onOpenMobileNav={() => setMobileOpen(true)}
          onOpenPalette={() => setPaletteOpen(true)}
          datasetVersion={operations.datasetVersion}
          datasetStatus={
            operations.datasetVersion === "Not recorded"
              ? "working"
              : "published"
          }
          environment={
            process.env.NODE_ENV === "production" ? "production" : "local"
          }
          pipelineHealth={operations.pipelineHealth}
          reviewCount={0}
          notificationCount={0}
          canCreate={false}
          userMenu={
            <button
              className="gg-button gg-button--ghost gg-button--icon"
              type="button"
              onClick={() => void signOut()}
              aria-label="Sign out"
              title={`Sign out ${session?.email ?? ""}`}
            >
              <LogOut size={18} aria-hidden />
            </button>
          }
        />

        <main id="main" className="gg-main">
          <nav className="admin-page-crumbs" aria-label="Breadcrumb">
            <ol>
              {crumbs.map((crumb, index) => (
                <li key={crumb.href}>
                  {index > 0 ? <span aria-hidden>/</span> : null}
                  <a
                    href={crumb.href}
                    aria-current={
                      index === crumbs.length - 1 ? "page" : undefined
                    }
                  >
                    {crumb.label}
                  </a>
                </li>
              ))}
            </ol>
          </nav>
          {!routeAccess.matched || routeAccess.allowed ? (
            children
          ) : (
            <section className="admin-access-denied">
              <span className="admin-login__eyebrow">Access restricted</span>
              <h1>This workspace is not assigned to your role.</h1>
              <p>
                Your signed-in role is{" "}
                <strong>
                  {session?.role.replaceAll("_", " ").toLowerCase()}
                </strong>
                . Ask a super admin to change your access if this is unexpected.
              </p>
              <button
                className="gg-button gg-button--primary gg-button--md"
                type="button"
                onClick={() => navigate("/")}
              >
                Return to overview
              </button>
            </section>
          )}
        </main>
      </div>

      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        groups={NAVIGATION}
        role={role}
        onNavigate={navigate}
        search={search}
      />
    </div>
  );
}
