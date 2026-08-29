"use client";

import { Badge, Skeleton, SkipLink, ThemeMenu, resolveApiBase } from "@ghanageo/ui";
import { ArrowLeft, Braces, Check, ChevronRight, Clock3, Code2, Copy, ExternalLink, FlaskConical, Gauge, Play, Search, Sparkles, Terminal } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);
const SAMPLES = [
  { label: "Kumasi, misspelled", path: "/search?q=kumsai", group: "Search", why: "Typo-tolerant ranking" },
  { label: "Tema communities", path: "/search?q=tema%20comm", group: "Search", why: "Ghanaian abbreviations" },
  { label: "Kwabɛnya", path: "/search?q=kwabenya", group: "Search", why: "Orthography folding" },
  { label: "Reverse in Kumasi", path: "/reverse?lat=6.688&lng=-1.624", group: "Spatial", why: "Coordinates to district" },
  { label: "Nearby central Accra", path: "/nearby?lat=5.556&lng=-0.182&radius=2000", group: "Spatial", why: "Places within 2 km" },
  { label: "Greater Accra boundary", path: "/boundaries/gh-region-greater-accra", group: "Spatial", why: "GeoJSON boundary" },
  { label: "First five regions", path: "/regions?limit=5", group: "Browse", why: "Cursor pagination" },
] as const;

type Snippet = "curl" | "javascript" | "go";
type Run = { path: string; status: number | null; ms: number; at: string };

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
  const [path, setPath] = useState<string>(SAMPLES[0].path);
  const [body, setBody] = useState("");
  const [status, setStatus] = useState<number | null>(null);
  const [ms, setMs] = useState<number | null>(null);
  const [cost, setCost] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  const [snippet, setSnippet] = useState<Snippet>("curl");
  const [copied, setCopied] = useState(false);
  const [history, setHistory] = useState<Run[]>([]);
  const code = useMemo(() => snippets(path)[snippet], [path, snippet]);

  const send = useCallback(async (requestPath: string) => {
    setPath(requestPath);
    setBusy(true);
    setError(false);
    const started = performance.now();
    try {
      const response = await fetch(API + requestPath);
      const elapsed = Math.round(performance.now() - started);
      setStatus(response.status);
      setMs(elapsed);
      setCost(response.headers.get("X-RateLimit-Cost"));
      const text = await response.text();
      try { setBody(JSON.stringify(JSON.parse(text), null, 2)); }
      catch { setBody(text || "The server returned an empty response."); }
      setError(!response.ok);
      setHistory((items) => [{ path: requestPath, status: response.status, ms: elapsed, at: new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) }, ...items.filter((item) => item.path !== requestPath)].slice(0, 4));
    } catch (reason) {
      const elapsed = Math.round(performance.now() - started);
      setStatus(null);
      setMs(elapsed);
      setError(true);
      setBody(`Could not reach ${API}\n\n${String(reason)}`);
      setHistory((items) => [{ path: requestPath, status: null, ms: elapsed, at: "now" }, ...items].slice(0, 4));
    } finally { setBusy(false); }
  }, []);

  async function copySnippet() {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  }

  return (
    <div className="sandbox-shell" data-intensity="balanced">
      <SkipLink />
      <header className="sandbox-header">
        <a className="sandbox-brand" href="http://localhost:3100" aria-label="Back to GhanaGeo">
          <span className="sandbox-brand__mark" aria-hidden>GG</span>
          <span><strong>GhanaGeo</strong><small>API sandbox</small></span>
        </a>
        <nav className="sandbox-protocols" aria-label="Protocol">
          <button type="button" aria-current="page">REST</button>
          <button type="button" disabled title="GraphQL explorer is planned in GEO-15.3">GraphQL <small>soon</small></button>
          <button type="button" disabled title="gRPC playground depends on GEO-11.4">gRPC <small>soon</small></button>
        </nav>
        <div className="sandbox-header__actions">
          <span className="sandbox-live"><i aria-hidden /> Public API</span>
          <ThemeMenu />
          <a className="sandbox-home" href="http://localhost:3100"><ArrowLeft size={15} aria-hidden /> <span>Website</span></a>
        </div>
      </header>

      <main id="main" className="sandbox-main">
        <aside className="sandbox-sidebar" aria-label="Sample requests">
          <div className="sandbox-sidebar__intro"><p>Request library</p><span>Seven useful starting points</span></div>
          <div className="sandbox-filter"><Search size={15} aria-hidden /><span>Curated examples</span><kbd>7</kbd></div>
          <div className="sandbox-samples">
            {SAMPLES.map((sample) => (
              <button key={sample.path} type="button" className={path === sample.path ? "is-active" : undefined} aria-pressed={path === sample.path} onClick={() => void send(sample.path)}>
                <span className="sandbox-samples__icon" aria-hidden>{sample.group === "Spatial" ? <Gauge size={15} /> : sample.group === "Browse" ? <Braces size={15} /> : <Search size={15} />}</span>
                <span><strong>{sample.label}</strong><small>{sample.why}</small></span>
                <ChevronRight size={14} aria-hidden />
              </button>
            ))}
          </div>
          {history.length > 0 ? <div className="sandbox-history"><p>Recent runs</p>{history.map((run) => <button key={`${run.path}-${run.at}`} type="button" onClick={() => setPath(run.path)}><span>{run.path.split("?")[0]}</span><small>{run.status ?? "offline"} · {run.ms} ms</small></button>)}</div> : null}
        </aside>

        <section className="sandbox-workbench">
          <div className="sandbox-workbench__head">
            <div><p className="sandbox-kicker"><Sparkles size={14} aria-hidden /> No account or API key</p><h1>Make a real request.</h1><p>Explore Ghana&rsquo;s location data against the public API, then copy the exact code into your project.</p></div>
            <a href="http://localhost:3102">Developer console <ExternalLink size={14} aria-hidden /></a>
          </div>
          <div className="sandbox-requestbar">
            <span className="sandbox-method">GET</span>
            <label><span className="sr-only">Request path</span><span className="sandbox-requestbar__origin">{API}</span><input value={path} onChange={(event) => setPath(event.target.value)} spellCheck={false} /></label>
            <button type="button" disabled={busy} onClick={() => void send(path)} aria-label={busy ? "Sending request" : undefined}>{busy ? <Skeleton className="sandbox-button-skeleton" /> : <><Play size={16} aria-hidden />Send request</>}</button>
          </div>

          <div className="sandbox-output-grid">
            <section className="sandbox-panel sandbox-response" data-intensity="restrained" aria-live="polite" aria-busy={busy}>
              <div className="sandbox-panel__bar"><div><Braces size={15} aria-hidden /><strong>Response</strong></div>{status !== null || ms !== null ? <div className="sandbox-metrics">{status !== null ? <Badge tone={status < 400 ? "canonical" : "danger"}>HTTP {status}</Badge> : <Badge tone="danger">Offline</Badge>}{ms !== null ? <span><Clock3 size={13} aria-hidden /> {ms} ms</span> : null}{cost ? <span>cost {cost}</span> : null}</div> : <span className="sandbox-panel__hint">JSON appears here</span>}</div>
              {busy ? <div className="sandbox-response-skeleton" role="status" aria-label="Loading response">{[88,64,76,52,70,38].map((width,index)=><Skeleton key={index} style={{width:`${width}%`}}/>)}</div> : <pre className={error ? "is-error" : undefined}>{body || `{\n  "ready": true,\n  "hint": "Choose a sample or edit the request path"\n}`}</pre>}
            </section>
            <section className="sandbox-panel sandbox-snippet" data-intensity="restrained">
              <div className="sandbox-panel__bar"><div><Code2 size={15} aria-hidden /><strong>Use this request</strong></div><button type="button" onClick={() => void copySnippet()} aria-label="Copy code snippet">{copied ? <Check size={15} aria-hidden /> : <Copy size={15} aria-hidden />}{copied ? "Copied" : "Copy"}</button></div>
              <div className="sandbox-code-tabs" role="tablist" aria-label="Code language">{(["curl", "javascript", "go"] as const).map((language) => <button key={language} type="button" role="tab" aria-selected={snippet === language} onClick={() => setSnippet(language)}>{language === "curl" ? <Terminal size={14} aria-hidden /> : language === "javascript" ? <Code2 size={14} aria-hidden /> : <Braces size={14} aria-hidden />}{language === "curl" ? "cURL" : language === "javascript" ? "JavaScript" : "Go"}</button>)}</div>
              <pre>{code}</pre>
            </section>
          </div>
        </section>
      </main>

      <footer className="sandbox-footer"><p><FlaskConical size={15} aria-hidden /> Anonymous requests use stricter fair-use limits. They never use privileged credentials.</p><nav aria-label="Sandbox footer"><a href="http://localhost:3100/docs">API docs</a><a href="http://localhost:3100/about">Data sources</a><a href="https://github.com" rel="noopener noreferrer">GitHub <ExternalLink size={13} aria-hidden /></a></nav></footer>
    </div>
  );
}
