import { SkipLink } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { HomeSearch } from "@/components/home-search";
import { ArrowRight, Braces, Check, ChevronRight, Database, Globe2, Map, ShieldCheck, Sparkles, Terminal } from "lucide-react";
import { pageMetadata, portalOrigin, sandboxOrigin } from "@/lib/seo";

export const metadata = pageMetadata({ title: "Ghana's open location infrastructure", description: "Search Ghana's regions, districts, towns and boundaries through one open, source-aware location layer.", path: "/" });
const COVERAGE = [{ value: "16", label: "Regions" }, { value: "261", label: "Districts / MMDAs" }, { value: "15,925", label: "Mapped places" }, { value: "248", label: "Boundaries" }];

export default function Home() {
  return <div className="site-page"><SkipLink/><MarketingHeader active="/"/><main id="main">
    <section className="home-hero">
      <div className="home-hero__map" aria-hidden><span>ACCRA</span><span>KUMASI</span><span>TAMALE</span><i/><i/><i/></div>
      <div className="home-hero__content"><p className="site-eyebrow"><Sparkles size={14}/> Open data. Built for Ghana.</p><h1>Every place in Ghana,<br/><em>finally in one place.</em></h1><p className="home-hero__lede">A dependable, open location layer for regions, districts, towns and boundaries—designed for the way Ghanaian places are actually named.</p>
        <HomeSearch />
        <div className="home-hero__actions"><a href={sandboxOrigin} className="site-button site-button--primary">Explore the API <ArrowRight size={16}/></a><a href="/docs" className="site-button">Read documentation</a></div>
      </div>
      <div className="home-proof"><div><ShieldCheck size={17}/><span><strong>Source-aware</strong><small>Provenance on every record</small></span></div><div><Globe2 size={17}/><span><strong>Free forever</strong><small>No account or API key</small></span></div><div><Check size={17}/><span><strong>Ghana-ready</strong><small>Twi, Ga and Ewe preserved</small></span></div></div>
    </section>

    <section className="home-stats gg-page gg-page--mid"><div className="section-intro"><p className="site-eyebrow">One canonical layer</p><h2>From the national view<br/>to the name on your street.</h2></div><div className="home-stats__grid">{COVERAGE.map((s,i)=><div key={s.label}><span>0{i+1}</span><strong>{s.value}</strong><p>{s.label}</p></div>)}</div></section>

    <section className="home-platform gg-page gg-page--mid"><div className="home-platform__copy"><p className="site-eyebrow">Made to be used</p><h2>One dataset.<br/><em>Every interface.</em></h2><p>Use GhanaGeo from a browser, terminal or production service. The same semantics and source metadata travel everywhere.</p><a href="/docs">See all developer options <ArrowRight size={15}/></a></div><div className="home-platform__cards">
      <a href={sandboxOrigin}><span><Terminal size={21}/><small>01</small></span><h3>REST API</h3><p>Simple HTTP endpoints for search, geocoding and boundaries.</p><code>GET /v1/search?q=osu</code><ChevronRight size={17}/></a>
      <a href="/docs"><span><Braces size={21}/><small>02</small></span><h3>GraphQL</h3><p>Navigate nested geography in one strongly typed request.</p><code>place → district → region</code><ChevronRight size={17}/></a>
      <a href="/docs"><span><Database size={21}/><small>03</small></span><h3>CLI & SDKs</h3><p>Human-friendly output and typed packages for your stack.</p><code>npx ghanageo search "tema"</code><ChevronRight size={17}/></a>
    </div></section>

    <section className="home-trust"><div className="gg-page gg-page--mid home-trust__inner"><div><p className="site-eyebrow">Designed for trust</p><h2>Know where every answer came from.</h2><p>Location data becomes infrastructure only when teams can explain it. GhanaGeo keeps the source, retrieval date, verification status and licensing context attached.</p><a className="site-button" href="/about">How the data works <ArrowRight size={15}/></a></div><div className="home-trust__visual" aria-hidden><Map size={44}/><span className="trust-pin trust-pin--one"><i/>Accra · verified</span><span className="trust-pin trust-pin--two"><i/>Ho · source linked</span><span className="trust-pin trust-pin--three"><i/>Wa · canonical</span></div></div></section>

    <section className="home-cta gg-page gg-page--mid"><p className="site-eyebrow">Start anywhere</p><h2>Ask Ghana where Ghana is.</h2><p>No signup. No trial. Make a real request now.</p><div><a className="site-button site-button--primary" href={sandboxOrigin}>Open the sandbox <ArrowRight size={16}/></a><a className="site-button" href={portalOrigin}>Developer portal</a></div></section>
  </main><MarketingFooter/></div>;
}
