import type { ReactNode } from "react";
import { Badge, Card } from "@ghanageo/ui";
import { CircleDashed, Compass } from "lucide-react";

/** Standard page heading, so every admin screen opens the same way. */
export function PageHeader({
  title, lede, actions, eyebrow,
}: {
  title: string;
  lede?: ReactNode | undefined;
  actions?: ReactNode | undefined;
  eyebrow?: string | undefined;
}) {
  return (
    <header style={{ marginBottom: "var(--space-6)" }}>
      {eyebrow ? (
        <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".14em", textTransform: "uppercase",
                    color: "var(--brand)", fontWeight: 800, margin: "0 0 var(--space-2)" }}>{eyebrow}</p>
      ) : null}
      <div className="gg-stack-row" style={{ alignItems: "flex-start", gap: "var(--space-4)" }}>
        <div style={{ flex: 1, minWidth: 240 }}>
          <h1 style={{ fontSize: "clamp(1.4rem, 4vw, 1.9rem)", margin: 0, lineHeight: 1.2 }}>{title}</h1>
          {lede ? (
            <p style={{ color: "var(--fg-muted)", margin: "var(--space-2) 0 0", maxWidth: "70ch" }}>{lede}</p>
          ) : null}
        </div>
        {actions ? <div className="gg-stack-row" style={{ flex: "0 0 auto" }}>{actions}</div> : null}
      </div>
    </header>
  );
}

/**
 * The screen for a route that exists in the navigation but has no
 * implementation yet.
 *
 * Every rail entry must LAND somewhere. A link that silently does nothing (the
 * bug this replaces) or 404s teaches people the admin is broken; a page that
 * says exactly what it will do, which story builds it, and where to go
 * meanwhile is honest and still useful. It deliberately does NOT render fake
 * charts or placeholder rows — mock data in an admin console is
 * indistinguishable from real data at a glance, which is how it ends up quoted
 * in a meeting.
 */
export function PlannedScreen({
  title, story, purpose, capabilities, dependsOn,
}: {
  title: string;
  story?: string | undefined;
  purpose: string;
  capabilities: readonly string[];
  dependsOn?: string | undefined;
}) {
  return (
    <>
      <PageHeader
        title={title}
        lede={purpose}
        actions={story ? <Badge tone="needsRecon">! planned · {story}</Badge> : null}
      />

      <Card style={{ marginBottom: "var(--space-5)" }}>
        <div className="gg-stack-row" style={{ alignItems: "flex-start" }}>
          <CircleDashed size={20} style={{ color: "var(--fg-subtle)", flexShrink: 0 }} aria-hidden />
          <div style={{ flex: 1, minWidth: 220 }}>
            <p style={{ margin: 0, fontWeight: 650 }}>Not built yet</p>
            <p style={{ margin: "var(--space-1) 0 0", color: "var(--fg-muted)",
                        fontSize: "var(--text-sm)", lineHeight: 1.6 }}>
              This screen is specified but not implemented. Nothing here is mocked, because a
              placeholder chart in an admin console is indistinguishable from a real one.
              {dependsOn ? ` Blocked on ${dependsOn}.` : ""}
            </p>
          </div>
        </div>
      </Card>

      <h2 style={{ fontSize: "var(--text-lg)", marginBottom: "var(--space-3)" }}>What it will do</h2>
      <Card data-intensity="restrained" style={{ marginBottom: "var(--space-5)" }}>
        <ul style={{ margin: 0, paddingInlineStart: "var(--space-5)", display: "grid",
                     gap: "var(--space-2)", lineHeight: 1.55 }}>
          {capabilities.map((c) => (
            <li key={c} style={{ color: "var(--fg-muted)" }}>{c}</li>
          ))}
        </ul>
      </Card>

      <Card data-intensity="restrained">
        <div className="gg-stack-row" style={{ alignItems: "flex-start" }}>
          <Compass size={18} style={{ color: "var(--brand)", flexShrink: 0 }} aria-hidden />
          <p style={{ margin: 0, fontSize: "var(--text-sm)", color: "var(--fg-muted)", flex: 1, minWidth: 200 }}>
            Working screens today: <a href="/geography/regions">Regions</a>,{" "}
            <a href="/geography/districts">Districts</a>, <a href="/geography/places">Places</a> and the{" "}
            <a href="/explorer">Location Explorer</a> — all reading the live API.
          </p>
        </div>
      </Card>
    </>
  );
}
