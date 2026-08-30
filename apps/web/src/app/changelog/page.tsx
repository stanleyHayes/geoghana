import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { Database, GitCommitHorizontal, Package } from "lucide-react";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({ title: "API and dataset changelog", description: "Track GhanaGeo API releases, dataset versions, identifier migrations and developer-tool changes.", path: "/changelog" });

const releases=[
	{date:"29 Aug 2026",version:"Dataset 2026.08.3-ulid",icon:Database,items:["Canonical region, district and place identifiers now use deterministic ULIDs.","Previously published slug identifiers return a tombstone with their replacement ID.","Provenance, relationships and 248 licensed district boundaries were preserved across the migration."]},
  {date:"29 Aug 2026",version:"API v1 · hardening",icon:GitCommitHorizontal,items:["REST, GraphQL and gRPC share authentication, weighted fair-use limits and scoped keys.","gRPC health, reflection, deadlines and message limits verified.","Anonymous three-protocol sandbox and response mapping completed."]},
  {date:"29 Aug 2026",version:"Dataset 2026.08.1-seed",icon:Database,items:["16 regions, 261 districts and 15,925 named places published.","248 licensed boundary geometries restored to the read model.","CSV and GeoJSON downloads include byte-derived checksums and sizes."]},
  {date:"29 Aug 2026",version:"CLI 0.1 release path",icon:Package,items:["Seven static platform binaries, npm packaging and Homebrew formula generation verified.","CLI version output states its target API version."]},
] as const;

export default function ChangelogPage(){return <div className="site-page"><MarketingHeader active="/changelog"/><main id="main" className="launch-page">
  <section className="launch-hero gg-page gg-page--mid"><p className="site-eyebrow">API and dataset history</p><h1>Changes should arrive<br/><em>with an explanation.</em></h1><p>Dataset releases and interface changes share one public record so consumers can distinguish new geography from changed software.</p></section>
  <section className="change-timeline gg-page gg-page--mid">{releases.map(({date,version,icon:Icon,items})=><article key={version}><div className="change-timeline__rail"><Icon size={18}/><span/></div><div><time>{date}</time><h2>{version}</h2><ul>{items.map(item=><li key={item}>{item}</li>)}</ul></div></article>)}</section>
  </main><MarketingFooter/></div>}
