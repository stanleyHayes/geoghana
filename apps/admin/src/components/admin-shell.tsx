"use client";

import { usePathname, useRouter } from "next/navigation";
import { useCallback, useMemo, useState, type ReactNode } from "react";
import {
  CommandPalette, Field, Logo, Navbar, Select, Sidebar, SkipLink,
  type Crumb, type PlaceResult, type Role,
} from "@ghanageo/ui";
import { NAVIGATION } from "@/config/navigation";

const API = process.env.NEXT_PUBLIC_GHANAGEO_API_URL ?? "http://localhost:8180/v1";

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
  const [role, setRole] = useState<Role>("DATA_ADMIN");

  const navigate = useCallback(
    (href: string) => {
      router.push(href);
      setMobileOpen(false);
    },
    [router],
  );

  // Real search against the running API, with AbortSignal propagation so an
  // abandoned query cancels its network work.
  const search = useCallback(async (q: string, signal: AbortSignal): Promise<PlaceResult[]> => {
    const res = await fetch(`${API}/places?q=${encodeURIComponent(q)}&limit=8`, { signal });
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
  }, []);

  /* Breadcrumbs come from the nav registry rather than from the URL segments,
     so the trail reads "Geography / Districts / MMDAs" and not "geography /
     districts". The registry is already the source of truth for the rail. */
  const crumbs = useMemo<Crumb[]>(() => {
    const trail: Crumb[] = [{ label: "Ghana", href: "/" }];
    if (pathname === "/") return [...trail, { label: "Overview", href: "/" }];
    for (const group of NAVIGATION) {
      for (const item of group.items) {
        if (item.href !== pathname) continue;
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
        return trail;
      }
    }
    return [...trail, { label: "Not found", href: pathname }];
  }, [pathname]);

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
          !collapsed ? (
            <Field label="Preview as role" htmlFor="role-picker">
              <Select
                ariaLabel="Preview as role"
                value={role}
                onValueChange={(v) => setRole(v as Role)}
                options={[
                  { value: "SUPER_ADMIN", label: "Super Admin", hint: "Everything" },
                  { value: "DATA_ADMIN", label: "Data Admin", hint: "Canonical geography and releases" },
                  { value: "DATA_REVIEWER", label: "Data Reviewer", hint: "Review and approve changes" },
                  { value: "DATA_CONTRIBUTOR", label: "Data Contributor", hint: "Propose edits, cannot publish" },
                  { value: "DEVELOPER_SUPPORT", label: "Developer Support", hint: "Accounts and keys" },
                  { value: "SECURITY_AUDITOR", label: "Security / Auditor", hint: "Read-only, never mutates" },
                ]}
              />
            </Field>
          ) : null
        }
      />

      <div className="gg-shell__content">
        <Navbar
          crumbs={crumbs}
          collapsed={collapsed}
          onToggleCollapse={() => setCollapsed((c) => !c)}
          onOpenMobileNav={() => setMobileOpen(true)}
          onOpenPalette={() => setPaletteOpen(true)}
          datasetVersion="2026.08.1-seed"
          datasetStatus="working"
          environment="local"
          pipelineHealth="ok"
          reviewCount={12}
          notificationCount={3}
          canCreate={role !== "SECURITY_AUDITOR"}
        />

        <main id="main" className="gg-main">{children}</main>
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
