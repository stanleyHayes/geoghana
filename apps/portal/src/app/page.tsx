"use client";

import { ThemeMenu } from "@ghanageo/ui";
import { Activity, ArrowRight, BookOpen, Braces, CheckCircle2, ChevronRight, CircleHelp, Clock3, Code2, Globe2, KeyRound, MapPin, Menu, Radio, Search, ShieldCheck, Sparkles, Terminal, X } from "lucide-react";
import { useState } from "react";

const tools = [
  { icon: Terminal, label: "REST API", meta: "11 endpoints", href: "http://localhost:3101" },
  { icon: Braces, label: "GraphQL", meta: "Schema ready", href: "http://localhost:3101" },
  { icon: Code2, label: "SDKs & CLI", meta: "7 platforms", href: "http://localhost:3100/docs" },
];
const activity = [
  { path: "/v1/search", detail: "q=osu", time: "42 ms" },
  { path: "/v1/reverse", detail: "5.6037,-0.1870", time: "61 ms" },
  { path: "/v1/regions", detail: "limit=16", time: "28 ms" },
];

export default function Portal() {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className="portal-shell">
      <a className="portal-skip" href="#main">Skip to workspace</a>
      <header className="portal-header">
        <a className="portal-brand" href="http://localhost:3100" aria-label="GhanaGeo home"><span className="portal-brand__mark"><MapPin size={17} /></span><span><strong>GhanaGeo</strong><small>Developer portal</small></span></a>
        <nav className="portal-nav" aria-label="Portal navigation"><a href="#overview" aria-current="page">Overview</a><a href="http://localhost:3101">API sandbox</a><a href="http://localhost:3100/docs">Documentation</a></nav>
        <div className="portal-header__actions"><span className="portal-status"><span /> All systems operational</span><ThemeMenu /><button className="portal-menu" onClick={() => setMenuOpen((v) => !v)} aria-expanded={menuOpen} aria-label="Toggle navigation">{menuOpen ? <X size={19} /> : <Menu size={19} />}</button></div>
        {menuOpen ? <nav className="portal-mobile-nav"><a href="#overview">Overview</a><a href="http://localhost:3101">API sandbox</a><a href="http://localhost:3100/docs">Documentation</a></nav> : null}
      </header>

      <main id="main" className="portal-main">
        <section className="portal-hero" id="overview">
          <div><p className="portal-eyebrow"><Sparkles size={14} /> Public infrastructure, developer ready</p><h1>Build with Ghana&rsquo;s geography.</h1><p className="portal-hero__copy">Search places, resolve coordinates and move through Ghana&rsquo;s administrative structure with one dependable dataset.</p><div className="portal-hero__actions"><a className="portal-primary" href="http://localhost:3101">Make your first request <ArrowRight size={16} /></a><a className="portal-secondary" href="http://localhost:3100/docs"><BookOpen size={16} /> Read the docs</a></div></div>
          <div className="portal-code-card" aria-label="API request example"><div className="portal-code-card__top"><span><i /><i /><i /></span><span>GET /v1/search</span><CheckCircle2 size={15} /></div><pre><code><span className="code-dim">curl</span> <span className="code-url">&quot;https://api.geo.digitalghana.dev</span>{`\n`}<span className="code-url">  /v1/search?q=kwabenya&quot;</span>{`\n\n`}<span className="code-dim"># No API key required</span></code></pre><div className="portal-code-card__result"><span>200 OK</span><span>42 ms</span><span>15.9 kB</span></div></div>
        </section>

        <section className="portal-stat-strip" aria-label="Dataset summary"><div><strong>15,925</strong><span>mapped places</span></div><div><strong>261</strong><span>districts / MMDAs</span></div><div><strong>16</strong><span>regions</span></div><div><strong>99.99%</strong><span>API availability</span></div></section>

        <div className="portal-dashboard">
          <section className="portal-panel"><div className="portal-section-head"><div><p>Workspace</p><h2>Developer tools</h2></div><a href="http://localhost:3100/docs">View all <ArrowRight size={14} /></a></div><div className="portal-tool-list">{tools.map(({ icon: Icon, label, meta, href }) => <a href={href} key={label}><span className="portal-tool-icon"><Icon size={18} /></span><span><strong>{label}</strong><small>{meta}</small></span><ChevronRight size={16} /></a>)}</div></section>
          <section className="portal-panel portal-panel--activity"><div className="portal-section-head"><div><p>Live sample</p><h2>Recent requests</h2></div><span className="portal-live"><Radio size={13} /> API online</span></div><div className="portal-activity">{activity.map((row) => <div key={row.path}><span className="portal-method">GET</span><span className="portal-path"><strong>{row.path}</strong><small>{row.detail}</small></span><span className="portal-ok">200</span><span className="portal-time">{row.time}</span></div>)}</div><p className="portal-disclosure"><CircleHelp size={14} /> Example traffic illustrates the response format. Connect a key to see account activity.</p></section>
          <aside className="portal-panel portal-panel--account"><div className="portal-orbit"><Globe2 size={25} /><i /><i /><i /></div><p className="portal-eyebrow">Optional account</p><h2>Anonymous by default.</h2><p>Every public endpoint works without signing in. Create a key only when you need usage attribution, origin controls or audit history.</p><button type="button" disabled><KeyRound size={15} /> Account access coming soon</button><span><ShieldCheck size={14} /> Donations never change your limits</span></aside>
        </div>
        <section className="portal-bottom-grid"><a href="http://localhost:3101"><Search size={19} /><span><strong>Explore the dataset</strong><small>Run search, reverse geocoding and boundary requests.</small></span><ArrowRight size={17} /></a><a href="http://localhost:3100/transparency"><Activity size={19} /><span><strong>Service transparency</strong><small>See operating commitments, reporting and provenance.</small></span><ArrowRight size={17} /></a></section>
      </main>
      <footer className="portal-footer"><span>GhanaGeo · a digitalghana.dev public good</span><span><Clock3 size={13} /> Dataset v2026.08</span><a href="mailto:support@digitalghana.dev">Get help</a></footer>
    </div>
  );
}
