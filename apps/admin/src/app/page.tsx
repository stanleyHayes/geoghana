"use client";

import { useCallback, useState } from "react";
import {
  Badge, Card, CommandPalette, Logo, Navbar, Sidebar, SkipLink, SponsorWall,
  SupportPanel, Select, Field, ThemePicker, verificationTone,
  type PlaceResult, type Role,
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
          <h1 style={{ fontSize: "clamp(1.5rem, 5vw, 2rem)", marginBottom: "var(--space-2)" }}>
            Ghana&rsquo;s location data, as infrastructure
          </h1>
          <p style={{ color: "var(--fg-muted)", marginTop: 0, marginBottom: "var(--space-6)" }}>
            Bootstrap dataset <code style={{ fontFamily: "var(--font-mono)" }}>2026.08.1-seed</code> · 16 regions ·
            261 districts · 16 places. Press <kbd className="gg-kbd">⌘K</kbd> to search.
          </p>

          <div className="gg-auto-grid" style={{ marginBottom: "var(--space-8)" }}>
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

          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-4)" }}>Support</h2>
          <div style={{ display: "grid", gap: "var(--space-5)", marginBottom: "var(--space-8)" }}>
            <SupportPanel
              donateHref="/support"
              figures={{ monthlyCostMinor: 48000, monthlyReceivedMinor: 17500, currency: "GHS" }}
            />
            <SponsorWall
              sponsors={[
                { id: "a", name: "Ghana Open Data Initiative", months: 14 },
                { id: "b", name: "Accra Dev Collective", months: 6 },
                { id: "c", name: "Individual supporters", months: 3 },
              ]}
            />
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
