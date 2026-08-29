import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ArrowRight, Braces, Database, Download, Map, Search, Terminal } from "lucide-react";

const products = [
  { icon: Search, title: "Search and geocoding", copy: "Resolve the names people actually use, including aliases, misspellings and Ghanaian orthography.", proof: "Search · autocomplete · geocode · reverse" },
  { icon: Map, title: "Administrative geography", copy: "Move consistently between Ghana, its 16 regions, 261 districts and the places assigned to them.", proof: "Stable IDs · redirects · provenance" },
  { icon: Database, title: "Boundaries and datasets", copy: "Download versioned CSV and GeoJSON artifacts with checksums, licences and source records attached.", proof: "248 current boundary geometries" },
  { icon: Braces, title: "Three API protocols", copy: "Use REST, GraphQL or gRPC over the same application services and fair-use controls.", proof: "One semantic contract" },
  { icon: Terminal, title: "CLI and typed clients", copy: "Explore from a terminal or integrate with TypeScript and React without rebuilding pagination and errors.", proof: "npx · Homebrew · Go install" },
  { icon: Download, title: "Open bulk access", copy: "Take the dataset with you. Public infrastructure should remain useful without permanent API dependence.", proof: "CSV · GeoJSON · SHA-256" },
] as const;

export default function ProductsPage() {
  return <div className="site-page"><MarketingHeader active="/products"/><main id="main" className="launch-page">
    <section className="launch-hero gg-page gg-page--mid"><p className="site-eyebrow">The public location layer</p><h1>One geography.<br/><em>Six useful ways in.</em></h1><p>GhanaGeo turns a documented national dataset into practical tools for public services, research, journalism and software.</p><div className="launch-hero__actions"><a className="site-button site-button--primary" href="http://localhost:3101">Try a real request <ArrowRight size={16}/></a><a className="site-button" href="/coverage">Inspect coverage</a></div></section>
    <section className="launch-grid gg-page gg-page--mid" aria-label="GhanaGeo products">{products.map(({icon:Icon,title,copy,proof})=><article key={title}><Icon size={20}/><h2>{title}</h2><p>{copy}</p><code>{proof}</code></article>)}</section>
    <section className="launch-callout gg-page gg-page--mid"><div><p className="site-eyebrow">No lock-in</p><h2>Use the interface. Keep the data.</h2></div><p>Every public interface is free, and bulk exports remain part of the product—not an enterprise escape hatch.</p><a href="/docs">Choose an integration <ArrowRight size={15}/></a></section>
  </main><MarketingFooter/></div>;
}
