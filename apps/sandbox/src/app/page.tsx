"use client";

import { Badge, Select, Skeleton, SkipLink, ThemeMenu, resolveApiBase } from "@ghanageo/ui";
import { useGhanaGeoClient } from "@ghanageo/react";
import { ArrowLeft, Braces, Check, ChevronRight, Clock3, Code2, Copy, ExternalLink, FlaskConical, Gauge, Play, Search, Sparkles, Terminal } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";
import { ResponseMap } from "./response-map";
import { portalOrigin, webOrigin } from "@/lib/public-origins";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);
const GRAPHQL = process.env.NEXT_PUBLIC_GHANAGEO_GRAPHQL_URL ?? API.replace(/\/v1\/?$/, "/graphql");
type Protocol = "rest" | "graphql" | "grpc";
const SAMPLES = [
  { label: "Osu", path: "/search?q=osu", group: "Search", why: "Exact locality search" },
  { label: "Adenta", path: "/search?q=adenta", group: "Search", why: "Municipality and locality" },
  { label: "Tema Community 25", path: "/search?q=tema%20community%2025", group: "Search", why: "Numbered community name" },
  { label: "Kumasi", path: "/search?q=kumasi", group: "Search", why: "Regional capital" },
  { label: "Kumasi, misspelled", path: "/search?q=kumsai", group: "Search", why: "Typo-tolerant ranking" },
  { label: "Tema communities", path: "/search?q=tema%20comm", group: "Search", why: "Ghanaian abbreviations" },
  { label: "Kwabɛnya", path: "/search?q=kwabenya", group: "Search", why: "Orthography folding" },
  { label: "Reverse in Kumasi", path: "/reverse?lat=6.688&lng=-1.624", group: "Spatial", why: "Coordinates to district" },
  { label: "Nearby central Accra", path: "/nearby?lat=5.556&lng=-0.182&radius=2000", group: "Spatial", why: "Places within 2 km" },
  { label: "Greater Accra boundary", path: "/boundaries/01KDVDNA00A63NSRPVSM94SCQD", group: "Spatial", why: "GeoJSON boundary" },
  { label: "First five regions", path: "/regions?limit=5", group: "Browse", why: "Cursor pagination" },
] as const;

type Snippet = "curl" | "javascript" | "go";
type Run = { label: string; request: string; payload: string; protocol: Protocol; status: number | null; ms: number; at: string };
type SchemaField = { name: string; description?: string | null };

const GRAPHQL_SAMPLE = `query SearchGhana($query: String!) {
  search(query: $query, first: 5) {
    datasetVersion
    nodes { place { id name type } score matchReason }
  }
}`;
const GRPC_SAMPLE = JSON.stringify({ query: "Kumasi", limit: 5 }, null, 2);
const GRAPHQL_TEMPLATES = [
  { label: "Search", query: GRAPHQL_SAMPLE, variables: '{"query":"Kumasi"}' },
  { label: "Regions", query: "query Regions { regions(first: 5) { datasetVersion nodes { id name capital } } }", variables: "{}" },
  { label: "Reverse", query: "query Reverse($lat: Float!, $lng: Float!) { reverse(latitude: $lat, longitude: $lng) { datasetVersion region { id name } district { id name } } }", variables: '{"lat":5.556,"lng":-0.182}' },
] as const;
const GRPC_METHODS = {
  Search: { query: { label: "Search query", type: "text", value: "Kumasi" }, limit: { label: "Result limit", type: "number", value: 5 } },
  Autocomplete: { query: { label: "Prefix", type: "text", value: "Acc" }, limit: { label: "Result limit", type: "number", value: 5 } },
  Geocode: { query: { label: "Place name", type: "text", value: "Osu" }, limit: { label: "Result limit", type: "number", value: 5 } },
  ListRegions: { limit: { label: "Result limit", type: "number", value: 10 } },
  ListDistricts: { regionId: { label: "Region ID", type: "text", value: "01KDVDNA00A63NSRPVSM94SCQD" }, limit: { label: "Result limit", type: "number", value: 10 } },
  ListPlaces: { regionId: { label: "Region ID", type: "text", value: "01KDVDNA00A63NSRPVSM94SCQD" }, limit: { label: "Result limit", type: "number", value: 10 } },
  ReverseGeocode: { latitude: { label: "Latitude", type: "number", value: 5.556 }, longitude: { label: "Longitude", type: "number", value: -0.182 } },
  Nearby: { latitude: { label: "Latitude", type: "number", value: 5.556 }, longitude: { label: "Longitude", type: "number", value: -0.182 }, radiusMeters: { label: "Radius (metres)", type: "number", value: 2000 }, limit: { label: "Result limit", type: "number", value: 10 } },
  GetBoundary: { id: { label: "Region or district ID", type: "text", value: "01KDVDNA00A63NSRPVSM94SCQD" } },
  StreamDatasetChanges: { sinceCursor: { label: "Resume cursor (empty starts live)", type: "text", value: "" } },
} as const;
type GrpcMethod = keyof typeof GRPC_METHODS;

function grpcPayload(method: GrpcMethod) {
  return Object.fromEntries(Object.entries(GRPC_METHODS[method]).map(([key, field]) => [key, field.value]));
}

function estimateGraphqlComplexity(query: string) {
  const fields = Math.max(1, (query.match(/\b[a-zA-Z_]\w*\b/g) ?? []).length - 2);
  const first = (operation: string, fallback: number) => Number(query.match(new RegExp(`${operation}\\s*\\([^)]*first\\s*:\\s*(\\d+)`, "i"))?.[1] ?? fallback);
  let weighted = fields;
  if (/\bsearch\s*\(/i.test(query)) weighted += 6 * first("search", 20);
  if (/\bnearby\s*\(/i.test(query)) weighted += 12 * first("nearby", 20);
  if (/\breverse\s*\(/i.test(query)) weighted += 12;
  weighted += (query.match(/\bboundary\b/gi) ?? []).length * 40;
  for (const operation of ["regions", "districts", "places"]) {
    if (new RegExp(`\\b${operation}\\s*\\(`, "i").test(query)) weighted += 2 * first(operation, 20);
  }
  return weighted;
}

function snippets(path: string): Record<Snippet, string> {
  const url = `${API}${path}`;
  return {
    curl: `curl --request GET \\\n  --url "${url}" \\\n  --header "Accept: application/json"`,
    javascript: `const response = await fetch("${url}", {
  headers: { Accept: "application/json" },
});

const result = await response.json();`,
    go: `req, _ := http.NewRequest(http.MethodGet, "${url}", nil)
req.Header.Set("Accept", "application/json")

response, err := http.DefaultClient.Do(req)`,
  };
}

export default function Sandbox() {
  const sdk = useGhanaGeoClient();
  const [protocol, setProtocol] = useState<Protocol>("rest");
  const [path, setPath] = useState<string>(SAMPLES[0].path);
  const [payload, setPayload] = useState('{"query":"Kumasi"}');
  const [body, setBody] = useState("");
  const [status, setStatus] = useState<number | null>(null);
  const [ms, setMs] = useState<number | null>(null);
  const [cost, setCost] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const [snippet, setSnippet] = useState<Snippet>("curl");
  const [copied, setCopied] = useState(false);
  const [history, setHistory] = useState<Run[]>([]);
  const [schemaFields, setSchemaFields] = useState<SchemaField[]>([]);
  const activeRequest = useRef<AbortController | null>(null);
  const code = useMemo(() => {
    let parsedPayload: unknown = {};
    try { parsedPayload = JSON.parse(payload || "{}"); } catch { parsedPayload = {}; }
    if (protocol === "rest") return snippets(path)[snippet];
    if (protocol === "graphql") {
      const body = JSON.stringify({ query: path, variables: parsedPayload }, null, 2);
      return snippet === "curl"
        ? `curl --request POST --url "${GRAPHQL}" --header "Content-Type: application/json" --data '${body}'`
        : snippet === "javascript"
          ? `const response = await fetch("${GRAPHQL}", {\n  method: "POST",\n  headers: { "Content-Type": "application/json" },\n  body: JSON.stringify(${body}),\n});`
          : `// POST the GraphQL document and variables to ${GRAPHQL}`;
    }
    return snippet === "curl"
      ? `grpcurl -plaintext -d '${payload}' localhost:9190 ghanageo.v1.GeographyService/${path}`
      : snippet === "javascript"
        ? `const response = await fetch("/api/grpc", {\n  method: "POST",\n  headers: { "Content-Type": "application/json" },\n  body: JSON.stringify({ method: "${path}", payload: ${payload} }),\n});`
        : `conn, _ := grpc.NewClient("localhost:9190", grpc.WithTransportCredentials(insecure.NewCredentials()))\n// Invoke ghanageo.v1.GeographyService/${path}`;
  }, [path, payload, protocol, snippet]);

  const complexity = useMemo(() => protocol === "graphql" ? estimateGraphqlComplexity(path) : null, [path, protocol]);
  const portalHref = useMemo(() => {
    const params = new URLSearchParams({ from: "sandbox", protocol, request: path, payload });
    return `${portalOrigin}?${params.toString()}`;
  }, [path, payload, protocol]);

  async function loadGraphqlSchema() {
    if (schemaFields.length > 0) return;
    try {
      const response = await fetch(GRAPHQL, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ query: "query SandboxSchema { __type(name: \"Query\") { fields { name description } } }" }),
      });
      const result = await response.json() as { data?: { __type?: { fields?: SchemaField[] } } };
      setSchemaFields(result.data?.__type?.fields ?? []);
    } catch { setSchemaFields([]); }
  }

  function chooseProtocol(next: Protocol) {
    setProtocol(next);
    setStatus(null);
    setMs(null);
    setBody("");
    if (next === "rest") setPath(SAMPLES[0].path);
    if (next === "graphql") { setPath(GRAPHQL_SAMPLE); setPayload('{"query":"Kumasi"}'); void loadGraphqlSchema(); }
    if (next === "grpc") { setPath("Search"); setPayload(GRPC_SAMPLE); }
  }

  function chooseGrpcMethod(method: GrpcMethod) {
    setPath(method);
    setPayload(JSON.stringify(grpcPayload(method), null, 2));
  }

  function updateGrpcField(field: string, value: string, type: string) {
    let current: Record<string, unknown> = {};
    try { current = JSON.parse(payload || "{}"); } catch { /* replace malformed input with the typed form */ }
    current[field] = type === "number" ? Number(value) : value;
    setPayload(JSON.stringify(current, null, 2));
  }

  const send = useCallback(async (requestPath: string) => {
    setPath(requestPath);
    setBusy(true);
    setError(false);
    const started = performance.now();
    const controller = new AbortController();
    activeRequest.current = controller;
    try {
      let response: Response;
      if (protocol === "rest") {
        const url = new URL(requestPath, `${API}/`);
        const params = Object.fromEntries(url.searchParams.entries());
        const result = await sdk.requestRaw<Record<string, unknown>>(url.pathname, params, controller.signal);
        response = new Response(JSON.stringify(result.body), { status: result.response.status, headers: result.response.headers });
      } else {
        let parsed: unknown;
        try { parsed = JSON.parse(payload || "{}"); }
        catch { throw new Error("Variables/request payload must be valid JSON."); }
        response = await fetch(protocol === "graphql" ? GRAPHQL : "/api/grpc", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(protocol === "graphql" ? { query: requestPath, variables: parsed } : { method: requestPath, payload: parsed }),
          signal: controller.signal,
        });
      }
      const elapsed = Math.round(performance.now() - started);
      setStatus(response.status);
      setMs(elapsed);
      setCost(response.headers.get("X-RateLimit-Cost"));
      let text = "";
      if (response.headers.get("content-type")?.includes("application/x-ndjson") && response.body) {
        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          text += decoder.decode(value, { stream: true });
          setBody(text.trim());
        }
      } else {
        text = await response.text();
      }
      try { setBody(JSON.stringify(JSON.parse(text), null, 2)); }
      catch { setBody(text || "The server returned an empty response."); }
      setError(!response.ok);
      const historyLabel = protocol === "graphql" ? "GraphQL query" : protocol === "grpc" ? `gRPC ${requestPath}` : requestPath;
      setHistory((items) => [{ label: historyLabel, request: requestPath, payload, protocol, status: response.status, ms: elapsed, at: new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) }, ...items.filter((item) => item.request !== requestPath || item.protocol !== protocol)].slice(0, 4));
    } catch (reason) {
      const elapsed = Math.round(performance.now() - started);
      if (reason instanceof DOMException && reason.name === "AbortError") {
        setMs(elapsed);
        setBody((current) => `${current}\n${JSON.stringify({ disconnected: true, message: "Stream stopped." })}`.trim());
        return;
      }
      setStatus(null);
      setMs(elapsed);
      setError(true);
      setBody(`Could not reach ${API}\n\n${String(reason)}`);
      setHistory((items) => [{ label: requestPath, request: requestPath, payload, protocol, status: null, ms: elapsed, at: "now" }, ...items].slice(0, 4));
    } finally { activeRequest.current = null; setBusy(false); }
  }, [payload, protocol, sdk]);

  async function copySnippet() {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  }

  return (
    <div className="sandbox-shell" data-intensity="balanced">
      <SkipLink />
      <header className="sandbox-header">
        <a className="sandbox-brand" href={webOrigin} aria-label="Back to GhanaGeo">
          <span className="sandbox-brand__mark" aria-hidden>GG</span>
          <span><strong>GhanaGeo</strong><small>API sandbox</small></span>
        </a>
        <nav className="sandbox-protocols" aria-label="Protocol">
          {(["rest", "graphql", "grpc"] as const).map((item) => <button key={item} type="button" aria-current={protocol === item ? "page" : undefined} onClick={() => chooseProtocol(item)}>{item === "rest" ? "REST" : item === "graphql" ? "GraphQL" : "gRPC"}</button>)}
        </nav>
        <div className="sandbox-header__actions">
          <span className="sandbox-live"><i aria-hidden /> Public API</span>
          <ThemeMenu />
          <a className="sandbox-home" href={webOrigin} aria-label="Back to GhanaGeo website"><ArrowLeft size={15} aria-hidden /> <span>Website</span></a>
        </div>
      </header>

      <main id="main" className="sandbox-main">
        <aside className="sandbox-sidebar" aria-label="Sample requests" tabIndex={0}>
          <div className="sandbox-sidebar__intro"><p>Request library</p><span>{protocol === "rest" ? `${SAMPLES.length} useful starting points` : `${protocol === "graphql" ? "GraphQL" : "gRPC"} live workspace`}</span></div>
          <div className="sandbox-filter"><Search size={15} aria-hidden /><span>Curated examples</span><kbd>{SAMPLES.length}</kbd></div>
          {protocol === "rest" ? <div className="sandbox-samples">
            {SAMPLES.map((sample) => (
              <button key={sample.path} type="button" className={path === sample.path ? "is-active" : undefined} aria-pressed={path === sample.path} onClick={() => void send(sample.path)}>
                <span className="sandbox-samples__icon" aria-hidden>{sample.group === "Spatial" ? <Gauge size={15} /> : sample.group === "Browse" ? <Braces size={15} /> : <Search size={15} />}</span>
                <span><strong>{sample.label}</strong><small>{sample.why}</small></span>
                <ChevronRight size={14} aria-hidden />
              </button>
            ))}
          </div> : <div className="sandbox-protocol-note"><strong>{protocol === "graphql" ? "Schema-aware query" : "Allow-listed native RPC"}</strong><p>{protocol === "graphql" ? "Edit the document and variables, then inspect the live complexity estimate before sending." : "The server-side bridge exposes only read operations and never forwards privileged credentials."}</p>{protocol === "graphql" ? <div className="sandbox-schema"><span>Query completions</span><div>{GRAPHQL_TEMPLATES.map((template) => <button key={template.label} type="button" onClick={() => { setPath(template.query); setPayload(template.variables); }}>{template.label}</button>)}</div><details><summary>Schema fields ({schemaFields.length || "offline"})</summary><ul>{schemaFields.map((field) => <li key={field.name}><code>{field.name}</code>{field.description ? <small>{field.description}</small> : null}</li>)}</ul></details></div> : null}</div>}
          {history.length > 0 ? <div className="sandbox-history"><p>Recent runs</p>{history.map((run) => <button key={`${run.protocol}-${run.request}-${run.at}`} type="button" onClick={() => { setProtocol(run.protocol); setPath(run.request); setPayload(run.payload); }}><span>{run.label.split("?")[0]}</span><small>{run.status ?? "offline"} · {run.ms} ms</small></button>)}</div> : null}
        </aside>

        <section className="sandbox-workbench" aria-label="API request workbench" tabIndex={0}>
          <div className="sandbox-workbench__head">
            <div><p className="sandbox-kicker"><Sparkles size={14} aria-hidden /> No account or API key</p><h1>Make a real request.</h1><p>Explore Ghana&rsquo;s location data against the public API, then copy the exact code into your project.</p></div>
            <a href={portalHref}>Create developer account <ExternalLink size={14} aria-hidden /></a>
          </div>
          <div className={`sandbox-requestbar ${protocol !== "rest" ? "sandbox-requestbar--editor" : ""}`}>
            <span className="sandbox-method">{protocol === "rest" ? "GET" : protocol === "graphql" ? "GQL" : "RPC"}</span>
            {protocol === "grpc" ? (
              <div className="sandbox-requestbar__select">
                <Select
                  value={path}
                  onValueChange={(value) => chooseGrpcMethod(value as GrpcMethod)}
                  ariaLabel="gRPC method"
                  options={Object.keys(GRPC_METHODS).map((method) => ({ value: method, label: method }))}
                />
              </div>
            ) : (
              <label><span className="sr-only">{protocol === "rest" ? "Request path" : "GraphQL query"}</span>{protocol === "rest" ? <><span className="sandbox-requestbar__origin" suppressHydrationWarning>{API}</span><input value={path} onChange={(event) => setPath(event.target.value)} spellCheck={false} /></> : <textarea value={path} onChange={(event) => setPath(event.target.value)} spellCheck={false} />}</label>
            )}
            <button type="button" disabled={busy && !(protocol === "grpc" && path === "StreamDatasetChanges")} onClick={() => busy ? activeRequest.current?.abort() : void send(path)} aria-label={busy ? (protocol === "grpc" && path === "StreamDatasetChanges" ? "Stop stream" : "Sending request") : undefined}>{busy ? (protocol === "grpc" && path === "StreamDatasetChanges" ? <>Stop stream</> : <Skeleton className="sandbox-button-skeleton" />) : <><Play size={16} aria-hidden />Send request</>}</button>
          </div>
          {protocol !== "rest" ? <div className="sandbox-payload">{protocol === "graphql" ? <label><span>Variables</span><textarea value={payload} onChange={(event) => setPayload(event.target.value)} spellCheck={false} /></label> : <div className="sandbox-grpc-fields" aria-label={`${path} request fields`}>{Object.entries(GRPC_METHODS[path as GrpcMethod] ?? {}).map(([field, definition]) => { let values: Record<string, unknown> = {}; try { values = JSON.parse(payload); } catch { /* keep empty values */ } return <label key={field}><span>{definition.label}</span><input type={definition.type} value={String(values[field] ?? "")} step={definition.type === "number" ? "any" : undefined} onChange={(event) => updateGrpcField(field, event.target.value, definition.type)} /></label>; })}</div>}{complexity !== null ? <div className={complexity > 240 ? "is-warning" : undefined}><strong>Complexity {complexity} / 300</strong><span>{complexity > 240 ? "Close to the server budget" : "Within the anonymous query budget"}</span></div> : null}</div> : null}

          <div className="sandbox-output-grid">
            <section className="sandbox-panel sandbox-response" data-intensity="restrained" aria-live="polite" aria-busy={busy}>
              <div className="sandbox-panel__bar"><div><Braces size={15} aria-hidden /><strong>Response</strong></div>{status !== null || ms !== null ? <div className="sandbox-metrics">{status !== null ? <Badge tone={status < 400 ? "canonical" : "danger"}>HTTP {status}</Badge> : <Badge tone="danger">Offline</Badge>}{ms !== null ? <span><Clock3 size={13} aria-hidden /> {ms} ms</span> : null}{cost ? <span>cost {cost}</span> : null}</div> : <span className="sandbox-panel__hint">JSON appears here</span>}</div>
              {busy ? <div className="sandbox-response-skeleton" role="status" aria-label="Loading response">{[88,64,76,52,70,38].map((width,index)=><Skeleton key={index} style={{width:`${width}%`}}/>)}</div> : <pre className={error ? "is-error" : undefined}>{body || `{\n  "ready": true,\n  "hint": "Choose a sample or edit the request path"\n}`}</pre>}
            </section>
            <section className="sandbox-panel sandbox-snippet" data-intensity="restrained">
              <div className="sandbox-panel__bar"><div><Code2 size={15} aria-hidden /><strong>Use this request</strong></div><button type="button" onClick={() => void copySnippet()} aria-label="Copy code snippet">{copied ? <Check size={15} aria-hidden /> : <Copy size={15} aria-hidden />}{copied ? "Copied" : "Copy"}</button></div>
              <div className="sandbox-code-tabs" role="tablist" aria-label="Code language">{(["curl", "javascript", "go"] as const).map((language) => <button key={language} type="button" role="tab" aria-selected={snippet === language} onClick={() => setSnippet(language)}>{language === "curl" ? <Terminal size={14} aria-hidden /> : language === "javascript" ? <Code2 size={14} aria-hidden /> : <Braces size={14} aria-hidden />}{language === "curl" ? "cURL" : language === "javascript" ? "JavaScript" : "Go"}</button>)}</div>
              <pre suppressHydrationWarning>{code}</pre>
            </section>
          </div>
          {!busy && !error ? <ResponseMap response={body} /> : null}
        </section>
      </main>

      <footer className="sandbox-footer"><p><FlaskConical size={15} aria-hidden /> Anonymous requests use stricter fair-use limits. They never use privileged credentials.</p><nav aria-label="Sandbox footer"><a href={`${webOrigin}/docs`}>API docs</a><a href={`${webOrigin}/about`}>Data sources</a><a href="https://github.com" rel="noopener noreferrer">GitHub <ExternalLink size={13} aria-hidden /></a></nav></footer>
    </div>
  );
}
