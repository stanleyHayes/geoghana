import { Card, Badge } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ExternalLink, Braces, ShieldCheck } from "lucide-react";
import { SDK_API_VERSION, SDK_EXAMPLES, SDK_TESTED_DATASET_VERSION } from "@/content/sdk-docs.generated";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "GhanaGeo documentation",
  description: "Quick starts for REST, GraphQL, gRPC, the CLI and npm. No account required.",
  path: "/docs",
});

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
  ["GET /v1/roads", "OpenStreetMap-derived roads with ODbL attribution", "cheap"],
  ["GET /v1/pois", "OpenStreetMap-derived points of interest", "cheap"],
  ["GET /v1/datasets", "Published versions and immutable downloads", "cheap"],
];

const ACCOUNT_ENDPOINTS = [
  ["POST /v1/auth/register", "Create a developer account and begin email verification"],
  ["POST /v1/auth/login", "Start a rotating HttpOnly browser session"],
  ["POST /v1/auth/passkeys/login/begin", "Begin passwordless passkey login"],
  ["GET · POST /v1/developer/organizations", "List or create organizations"],
  ["GET · POST /v1/developer/organizations/{orgId}/applications", "List or create test/live applications"],
  ["GET · POST /v1/developer/organizations/{orgId}/applications/{appId}/keys", "List keys or create a one-time secret"],
  ["GET /v1/developer/organizations/{orgId}/applications/{appId}/usage", "Inspect attributed usage"],
  ["GET /v1/developer/organizations/{orgId}/applications/{appId}/requests", "Page through redacted request logs"],
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
          <div className="gg-stack-row" style={{ alignItems: "end", marginBottom: "var(--space-5)" }}>
            <div style={{ flex: 1 }}>
              <h2 style={{ fontSize: "var(--text-xl)", margin: 0 }}>Choose your language</h2>
              <p style={{ color: "var(--fg-muted)", margin: "var(--space-2) 0 0", maxWidth: "64ch" }}>
                Every quick start is extracted from source that is compiled in CI, so the docs stay aligned with the shipped SDK.
              </p>
            </div>
            <div className="gg-stack-row" aria-label="Compatibility versions">
              <Badge tone="canonical">API {SDK_API_VERSION}</Badge>
              <Badge tone="reviewed">Dataset {SDK_TESTED_DATASET_VERSION}</Badge>
            </div>
          </div>
          <div className="gg-auto-grid gg-auto-grid--pair">
            {SDK_EXAMPLES.map((sdk) => (
              <Card key={sdk.id} data-intensity="restrained">
                <div className="gg-stack-row" style={{ marginBottom: "var(--space-3)" }}>
                  <Braces size={18} style={{ color: "var(--brand)" }} aria-hidden />
                  <strong style={{ flex: 1, fontSize: "var(--text-lg)" }}>{sdk.label}</strong>
                  <Badge tone="canonical">tested</Badge>
                </div>
                <p className="gg-code" style={{ marginBottom: "var(--space-3)" }}>{sdk.install}</p>
                <pre className="gg-code gg-code--lg" style={{ maxHeight: 320 }}>{sdk.code}</pre>
                <p style={{ color: "var(--fg-subtle)", fontSize: "var(--text-xs)", margin: "var(--space-3) 0 0" }}>
                  Source: <code>{sdk.source}</code>
                </p>
              </Card>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-12)" }}>
          <div className="gg-stack-row" style={{ marginBottom: "var(--space-4)" }}>
            <ShieldCheck size={20} style={{ color: "var(--brand)" }} aria-hidden />
            <h2 style={{ fontSize: "var(--text-xl)", margin: 0 }}>Capability and compatibility</h2>
          </div>
          <Card data-intensity="restrained" style={{ padding: 0, overflow: "hidden" }}>
            <div style={{ overflowX: "auto" }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "var(--text-sm)" }}>
                <thead><tr style={{ background: "var(--bg-subtle)" }}>
                  {["SDK", "Runtime", "Protocols", "Cancellation", "Retries", "Pagination", "Typed error"].map((heading) => (
                    <th key={heading} style={{ textAlign: "start", padding: "var(--space-3) var(--space-4)", fontSize: "var(--text-2xs)", textTransform: "uppercase", letterSpacing: ".08em", color: "var(--fg-subtle)", whiteSpace: "nowrap" }}>{heading}</th>
                  ))}
                </tr></thead>
                <tbody>{SDK_EXAMPLES.map((sdk) => (
                  <tr key={sdk.id} style={{ borderTop: "1px solid var(--border)" }}>
                    <td style={{ padding: "var(--space-3) var(--space-4)", fontWeight: 700, whiteSpace: "nowrap" }}>{sdk.label}</td>
                    <td style={{ padding: "var(--space-3) var(--space-4)", color: "var(--fg-muted)", minWidth: 150 }}>{sdk.runtime}</td>
                    <td style={{ padding: "var(--space-3) var(--space-4)", whiteSpace: "nowrap" }}>{sdk.protocols.join(" · ")}</td>
                    <td style={{ padding: "var(--space-3) var(--space-4)", color: "var(--fg-muted)", minWidth: 180 }}>{sdk.cancellation}</td>
                    <td style={{ padding: "var(--space-3) var(--space-4)", color: "var(--fg-muted)", minWidth: 190 }}>{sdk.retry}</td>
                    <td style={{ padding: "var(--space-3) var(--space-4)", color: "var(--fg-muted)", minWidth: 190 }}>{sdk.pagination}</td>
                    <td style={{ padding: "var(--space-3) var(--space-4)", fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", whiteSpace: "nowrap" }}>{sdk.errors}</td>
                  </tr>
                ))}</tbody>
              </table>
            </div>
          </Card>
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

        <section style={{ marginBottom: "var(--space-12)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-2)" }}>Accounts and application keys</h2>
          <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-5)", maxWidth: "66ch" }}>
            Public geography remains anonymous by default. Create an account only when you need
            application attribution, origin or IP controls, key rotation, usage analytics or request logs.
            Account and developer routes use a rotating HttpOnly session cookie; browser mutations also
            enforce the configured origin allow-list.
          </p>
          <Card data-intensity="restrained" style={{ padding: 0, overflow: "hidden" }}>
            <div style={{ overflowX: "auto" }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "var(--text-sm)" }}>
                <thead><tr style={{ background: "var(--bg-subtle)" }}>
                  {['Endpoint', 'Purpose'].map((heading) => <th key={heading} style={{ textAlign: "start", padding: "var(--space-3) var(--space-4)", fontSize: "var(--text-2xs)", textTransform: "uppercase", letterSpacing: ".08em", color: "var(--fg-subtle)" }}>{heading}</th>)}
                </tr></thead>
                <tbody>{ACCOUNT_ENDPOINTS.map(([endpoint, purpose]) => <tr key={endpoint} style={{ borderTop: "1px solid var(--border)" }}>
                  <td style={{ padding: "var(--space-3) var(--space-4)", fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", whiteSpace: "nowrap" }}>{endpoint}</td>
                  <td style={{ padding: "var(--space-3) var(--space-4)", color: "var(--fg-muted)", minWidth: 260 }}>{purpose}</td>
                </tr>)}</tbody>
              </table>
            </div>
          </Card>
          <p style={{ marginTop: "var(--space-4)", fontSize: "var(--text-sm)" }}>
            <a href="https://github.com/stanleyHayes/geoghana/blob/main/contracts/openapi/v1.yaml"
               rel="noopener noreferrer" target="_blank" style={{ color: "var(--brand)" }}>
              Complete OpenAPI 3.1 contract <ExternalLink size={12} style={{ display: "inline" }} aria-hidden />
            </a>
          </p>
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
    "details": { "mergedInto": "01KDVDNA00N6BFFK8VF5K8YXPW" },
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
