import { Card, Badge } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { Terminal, Code2, Package, Boxes, ExternalLink } from "lucide-react";

export const metadata = {
  title: "GhanaGeo documentation",
  description: "Quick starts for REST, GraphQL, gRPC, the CLI and npm. No account required.",
};

const QUICKSTARTS = [
  {
    icon: Terminal,
    title: "curl",
    body: "No account, no key. Works from any terminal.",
    code: `curl "https://api.geo.digitalghana.dev/v1/search?q=osu"`,
    ready: true,
  },
  {
    icon: Boxes,
    title: "CLI",
    body: "Table output for humans, --json for scripts, --csv for spreadsheets.",
    code: `npx ghanageo search "tema comm"
npx ghanageo reverse 6.688 -1.624
npx ghanageo regions --json | jq -r '.[].capital'`,
    ready: true,
  },
  {
    icon: Package,
    title: "React",
    body: "TanStack Query hooks with cancellation and a stable query-key factory.",
    code: `npm install @ghanageo/react

const { data } = useAutocomplete(query);`,
    ready: false,
    story: "GEO-13.3",
  },
  {
    icon: Code2,
    title: "GraphQL",
    body: "Nested geography in one round trip, with a complexity budget.",
    code: `query { place(id: "gh-place-gn-2306104") {
  name district { name region { name } }
} }`,
    ready: true,
  },
  {
    icon: Boxes,
    title: "gRPC",
    body: "Typed service-to-service access over the same use cases as REST. Reflection is on, so grpcurl needs no .proto.",
    code: `grpcurl -plaintext api.geo.digitalghana.dev:443 list

grpcurl -d '{"limit":2}' api.geo.digitalghana.dev:443 \\
  ghanageo.v1.GeographyService/ListRegions`,
    ready: true,
  },
];

const ENDPOINTS = [
  ["GET /v1/regions", "Ghana's 16 regions", "cheap"],
  ["GET /v1/regions/{id}/districts", "Districts in a region", "cheap"],
  ["GET /v1/districts", "Filter districts by region or name", "cheap"],
  ["GET /v1/places", "Filter localities", "cheap"],
  ["GET /v1/places/{id}", "Resolve a place; merged ids return 410 with mergedInto", "cheap"],
  ["GET /v1/search", "Typo-tolerant search with confidence scores", "normal"],
  ["GET /v1/autocomplete", "Low-latency typeahead", "normal"],
  ["GET /v1/geocode", "Text to ranked candidates", "normal"],
  ["GET /v1/reverse", "Coordinates to region and district", "spatial"],
  ["GET /v1/nearby", "Places within a radius", "spatial"],
  ["GET /v1/boundaries/{id}", "GeoJSON Feature with attribution", "geometry"],
];

export default function Docs() {
  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <MarketingHeader active="/docs" />

      <main id="main" className="gg-page gg-page--mid">
        <h1 className="gg-hero__title" style={{ maxWidth: "22ch" }}>Documentation</h1>
        <p className="gg-hero__lede" style={{ marginBottom: "var(--space-10)" }}>
          Every endpoint is public. There is no key to obtain before you start —
          anonymous access is a supported path, not a trial.
        </p>

        <section style={{ marginBottom: "var(--space-12)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-5)" }}>Quick starts</h2>
          {/* Two across, not auto-fit: these cards are sized by their code
              samples, and four across clips every one of them. */}
          <div className="gg-auto-grid gg-auto-grid--pair">
            {QUICKSTARTS.map((q) => (
              <Card key={q.title}>
                <div className="gg-stack-row" style={{ marginBottom: "var(--space-2)" }}>
                  <q.icon size={18} style={{ color: "var(--brand)" }} aria-hidden />
                  <strong style={{ flex: 1 }}>{q.title}</strong>
                  {!q.ready ? <Badge tone="needsRecon">! planned · {q.story}</Badge> : null}
                </div>
                <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)",
                            margin: "0 0 var(--space-3)" }}>{q.body}</p>
                <pre className="gg-code">{q.code}</pre>
              </Card>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-12)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-2)" }}>Endpoints</h2>
          <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-5)", maxWidth: "62ch" }}>
            Cost class is how much of your fair-use allowance a call consumes. Spatial and
            geometry work costs more because it costs more to serve — GhanaGeo is free, so
            it is never about money.
          </p>
          <Card data-intensity="restrained" style={{ padding: 0, overflow: "hidden" }}>
            <div style={{ overflowX: "auto" }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "var(--text-sm)" }}>
                <thead>
                  <tr style={{ background: "var(--bg-subtle)" }}>
                    {["Endpoint", "What it does", "Cost"].map((h) => (
                      <th key={h} style={{ textAlign: "start", padding: "var(--space-3) var(--space-4)",
                                           fontSize: "var(--text-2xs)", textTransform: "uppercase",
                                           letterSpacing: ".08em", color: "var(--fg-subtle)",
                                           whiteSpace: "nowrap" }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {ENDPOINTS.map(([ep, what, cost]) => (
                    <tr key={ep} style={{ borderTop: "1px solid var(--border)" }}>
                      <td style={{ padding: "var(--space-3) var(--space-4)",
                                   fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)",
                                   whiteSpace: "nowrap" }}>{ep}</td>
                      <td style={{ padding: "var(--space-3) var(--space-4)", color: "var(--fg-muted)",
                                   minWidth: 220 }}>{what}</td>
                      <td style={{ padding: "var(--space-3) var(--space-4)" }}>
                        <Badge tone={cost === "cheap" ? "canonical" : cost === "geometry" ? "needsRecon" : "reviewed"}>
                          {cost}
                        </Badge>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </section>

        <section>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-3)" }}>Errors</h2>
          <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-4)", maxWidth: "62ch" }}>
            Every failure returns a stable machine code, a request id and a link to its own
            documentation page. Branch on <code style={{ fontFamily: "var(--font-mono)" }}>code</code>,
            never on the message text.
          </p>
          <pre className="gg-code gg-code--lg">
{`{
  "error": {
    "code": "RESOURCE_GONE",
    "message": "Place was merged into another record.",
    "requestId": "req_01K3Y…",
    "details": { "mergedInto": "gh-place-accra" },
    "docs": "/docs/errors/RESOURCE_GONE"
  }
}`}
          </pre>
          <p style={{ marginTop: "var(--space-4)", fontSize: "var(--text-sm)" }}>
            <a href="https://github.com/stanleyHayes/geoghana/tree/main/docs/errors"
               rel="noopener noreferrer" target="_blank" style={{ color: "var(--brand)" }}>
              All 16 error codes <ExternalLink size={12} style={{ display: "inline" }} aria-hidden />
            </a>
          </p>
        </section>
      </main>

      <MarketingFooter />
    </div>
  );
}
