"use client";

import { useCallback, useState } from "react";
import {
  Badge, Card, CommandPalette, Navbar, Sidebar, SkipLink, ThemePicker,
  verificationTone, type PlaceResult, type Role,
} from "@ghanageo/ui";
import { Globe } from "lucide-react";
import { NAVIGATION } from "@/config/navigation";

const API = process.env.NEXT_PUBLIC_GHANAGEO_API_URL ?? "http://localhost:8180/v1";

export default function AdminHome() {
  const [collapsed, setCollapsed] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [pathname, setPathname] = useState("/");
  const [role, setRole] = useState<Role>("DATA_ADMIN");

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
        onNavigate={(href) => {
          setPathname(href);
          setMobileOpen(false);
        }}
        brand={
          <span style={{ display: "flex", alignItems: "center", gap: "0.5rem", minWidth: 0 }}>
            <Globe size={22} style={{ color: "var(--brand)", flexShrink: 0 }} aria-hidden />
            {!collapsed ? (
              <span style={{ display: "grid", lineHeight: 1.1, minWidth: 0 }}>
                <strong style={{ fontFamily: "var(--font-display)", fontSize: "1rem" }}>GhanaGeo</strong>
                <span style={{ fontSize: "10px", letterSpacing: "0.16em", textTransform: "uppercase", color: "var(--fg-subtle)", fontWeight: 700 }}>
                  Admin
                </span>
              </span>
            ) : null}
          </span>
        }
        footer={
          !collapsed ? (
            <label style={{ display: "grid", gap: 4, fontSize: "var(--text-2xs)", color: "var(--fg-subtle)" }}>
              <span style={{ textTransform: "uppercase", letterSpacing: "0.1em", fontWeight: 700 }}>
                Preview as role
              </span>
              <select
                className="gg-input"
                value={role}
                onChange={(e) => setRole(e.target.value as Role)}
                style={{ minHeight: 36 }}
              >
                <option value="SUPER_ADMIN">Super Admin</option>
                <option value="DATA_ADMIN">Data Admin</option>
                <option value="DATA_REVIEWER">Data Reviewer</option>
                <option value="DATA_CONTRIBUTOR">Data Contributor</option>
                <option value="DEVELOPER_SUPPORT">Developer Support</option>
                <option value="SECURITY_AUDITOR">Security / Auditor</option>
              </select>
            </label>
          ) : null
        }
      />

      <div className="gg-shell__content">
        <Navbar
          crumbs={[{ label: "Ghana", href: "/" }, { label: "Overview", href: "/" }]}
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

        <main id="main" className="gg-main">
          <h1 style={{ fontSize: "var(--text-3xl)", marginBottom: "var(--space-2)" }}>
            Ghana&rsquo;s location data, as infrastructure
          </h1>
          <p style={{ color: "var(--fg-muted)", marginTop: 0, marginBottom: "var(--space-6)" }}>
            Bootstrap dataset <code style={{ fontFamily: "var(--font-mono)" }}>2026.08.1-seed</code> · 16 regions ·
            261 districts · 16 places. Press <kbd className="gg-kbd">⌘K</kbd> to search.
          </p>

          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: "var(--space-4)", marginBottom: "var(--space-8)" }}>
            {[
              { label: "Regions", value: "16", status: "REFERENCE" },
              { label: "Districts / MMDAs", value: "261", status: "SEED_NEEDS_CANONICAL_RECONCILIATION" },
              { label: "Places", value: "16", status: "REFERENCE" },
              { label: "Review queue", value: "12", status: "REVIEWED" },
            ].map((s) => {
              const v = verificationTone(s.status);
              return (
                <Card key={s.label} interactive>
                  <p style={{ fontSize: "var(--text-2xs)", textTransform: "uppercase", letterSpacing: "0.1em", color: "var(--fg-subtle)", fontWeight: 700, margin: 0 }}>
                    {s.label}
                  </p>
                  <p style={{ fontSize: "var(--text-3xl)", fontWeight: 700, margin: "var(--space-2) 0", fontVariantNumeric: "tabular-nums" }}>
                    {s.value}
                  </p>
                  <Badge tone={v.tone}><span aria-hidden>{v.glyph}</span> {v.label}</Badge>
                </Card>
              );
            })}
          </div>

          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-4)" }}>Appearance</h2>
          <ThemePicker />
        </main>
      </div>

      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        groups={NAVIGATION}
        role={role}
        onNavigate={setPathname}
        search={search}
      />
    </div>
  );
}
