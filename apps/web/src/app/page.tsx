"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Badge, SkipLink, verificationTone } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ArrowRight, Braces, Check, ChevronRight, Database, Globe2, Map, MapPin, Search, ShieldCheck, Sparkles, Terminal } from "lucide-react";

const API = process.env.NEXT_PUBLIC_GHANAGEO_API_URL ?? "http://localhost:8180/v1";
const COVERAGE = [{ value: "16", label: "Regions" }, { value: "261", label: "Districts / MMDAs" }, { value: "15,925", label: "Mapped places" }, { value: "248", label: "Boundaries" }];
type Hit = { id:string; name:string; kind:string; type:string; regionName?:string; districtName?:string; score:number };

export default function Home() {
  const [q, setQ] = useState(""); const [hits, setHits] = useState<Hit[]>([]); const [loading, setLoading] = useState(false); const abort = useRef<AbortController|null>(null);
  const run = useCallback(async (query:string) => { if(query.trim().length<2){setHits([]);return} abort.current?.abort(); const c=new AbortController(); abort.current=c; setLoading(true); try{const res=await fetch(`${API}/search?q=${encodeURIComponent(query)}&limit=5`,{signal:c.signal}); if(res.ok)setHits((await res.json()).data??[])}catch{}finally{if(!c.signal.aborted)setLoading(false)} },[]);
  useEffect(()=>{const t=setTimeout(()=>run(q),200);return()=>clearTimeout(t)},[q,run]);

  return <div className="site-page"><SkipLink/><MarketingHeader active="/"/><main id="main">
    <section className="home-hero">
      <div className="home-hero__map" aria-hidden><span>ACCRA</span><span>KUMASI</span><span>TAMALE</span><i/><i/><i/></div>
      <div className="home-hero__content"><p className="site-eyebrow"><Sparkles size={14}/> Open data. Built for Ghana.</p><h1>Every place in Ghana,<br/><em>finally in one place.</em></h1><p className="home-hero__lede">A dependable, open location layer for regions, districts, towns and boundaries—designed for the way Ghanaian places are actually named.</p>
        <div className="home-search-wrap"><label className="home-search"><Search size={20}/><input value={q} onChange={e=>setQ(e.target.value)} placeholder="Search Osu, Kwabenya, Tema Community 25…" aria-label="Search Ghanaian places"/>{loading?<span>Searching…</span>:<kbd>⌘ K</kbd>}</label>
          {hits.length>0?<div className="home-results">{hits.map(h=>{const v=verificationTone("REFERENCE");return <div key={h.id}><MapPin size={16}/><span><strong>{h.name}</strong><small>{[h.districtName,h.regionName].filter(Boolean).join(" · ")||h.kind}</small></span><Badge tone={v.tone}>{h.kind}</Badge></div>})}</div>:q.trim().length>=2&&!loading?<p className="home-no-result">No match yet. Try “kumsai” for typo-tolerant search.</p>:null}
        </div>
        <div className="home-hero__actions"><a href="http://localhost:3101" className="site-button site-button--primary">Explore the API <ArrowRight size={16}/></a><a href="/docs" className="site-button">Read documentation</a></div>
      </div>
      <div className="home-proof"><div><ShieldCheck size={17}/><span><strong>Source-aware</strong><small>Provenance on every record</small></span></div><div><Globe2 size={17}/><span><strong>Free forever</strong><small>No account or API key</small></span></div><div><Check size={17}/><span><strong>Ghana-ready</strong><small>Twi, Ga and Ewe preserved</small></span></div></div>
    </section>

    <section className="home-stats gg-page gg-page--mid"><div className="section-intro"><p className="site-eyebrow">One canonical layer</p><h2>From the national view<br/>to the name on your street.</h2></div><div className="home-stats__grid">{COVERAGE.map((s,i)=><div key={s.label}><span>0{i+1}</span><strong>{s.value}</strong><p>{s.label}</p></div>)}</div></section>

    <section className="home-platform gg-page gg-page--mid"><div className="home-platform__copy"><p className="site-eyebrow">Made to be used</p><h2>One dataset.<br/><em>Every interface.</em></h2><p>Use GhanaGeo from a browser, terminal or production service. The same semantics and source metadata travel everywhere.</p><a href="/docs">See all developer options <ArrowRight size={15}/></a></div><div className="home-platform__cards">
      <a href="http://localhost:3101"><span><Terminal size={21}/><small>01</small></span><h3>REST API</h3><p>Simple HTTP endpoints for search, geocoding and boundaries.</p><code>GET /v1/search?q=osu</code><ChevronRight size={17}/></a>
      <a href="/docs"><span><Braces size={21}/><small>02</small></span><h3>GraphQL</h3><p>Navigate nested geography in one strongly typed request.</p><code>place → district → region</code><ChevronRight size={17}/></a>
      <a href="/docs"><span><Database size={21}/><small>03</small></span><h3>CLI & SDKs</h3><p>Human-friendly output and typed packages for your stack.</p><code>npx ghanageo search "tema"</code><ChevronRight size={17}/></a>
    </div></section>

    <section className="home-trust"><div className="gg-page gg-page--mid home-trust__inner"><div><p className="site-eyebrow">Designed for trust</p><h2>Know where every answer came from.</h2><p>Location data becomes infrastructure only when teams can explain it. GhanaGeo keeps the source, retrieval date, verification status and licensing context attached.</p><a className="site-button" href="/about">How the data works <ArrowRight size={15}/></a></div><div className="home-trust__visual" aria-hidden><Map size={44}/><span className="trust-pin trust-pin--one"><i/>Accra · verified</span><span className="trust-pin trust-pin--two"><i/>Ho · source linked</span><span className="trust-pin trust-pin--three"><i/>Wa · canonical</span></div></div></section>

    <section className="home-cta gg-page gg-page--mid"><p className="site-eyebrow">Start anywhere</p><h2>Ask Ghana where Ghana is.</h2><p>No signup. No trial. Make a real request now.</p><div><a className="site-button site-button--primary" href="http://localhost:3101">Open the sandbox <ArrowRight size={16}/></a><a className="site-button" href="http://localhost:3102">Developer portal</a></div></section>
  </main><MarketingFooter/></div>;
}
