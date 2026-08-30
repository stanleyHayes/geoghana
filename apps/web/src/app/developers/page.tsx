import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ArrowRight, Braces, Package, Radio, Terminal } from "lucide-react";
import { pageMetadata, sandboxOrigin } from "@/lib/seo";

export const metadata = pageMetadata({ title: "Developers", description: "Start using GhanaGeo with REST, GraphQL, gRPC, TypeScript and React. Public read APIs work without an account.", path: "/developers" });

const paths = [
  { icon: Terminal, label: "REST", command: "curl 'https://api.geo.digitalghana.dev/v1/search?q=osu'", note: "The shortest path from an idea to a sourced result." },
  { icon: Braces, label: "GraphQL", command: "query { search(query: \"Osu\") { nodes { place { id name } } } }", note: "Traverse nested administrative geography in one request." },
  { icon: Radio, label: "gRPC", command: "grpcurl api.geo.digitalghana.dev:443 list", note: "Typed HTTP/2 contracts, reflection, deadlines and stable status codes." },
  { icon: Package, label: "npm + React", command: "pnpm add @ghanageo/react", note: "Typed hooks and client primitives with dataset-version metadata." },
] as const;

export default function DevelopersPage(){return <div className="site-page"><MarketingHeader active="/developers"/><main id="main" className="launch-page">
  <section className="launch-hero launch-hero--developer gg-page gg-page--mid"><p className="site-eyebrow">Build in under five minutes</p><h1>Start with a question,<br/><em>not an account form.</em></h1><p>Every read API works anonymously. Choose the protocol that fits your stack; the records and error meanings stay the same.</p><div className="launch-hero__actions"><a className="site-button site-button--primary" href={sandboxOrigin}>Open the sandbox <ArrowRight size={16}/></a><a className="site-button" href="/docs">Read the full docs</a></div></section>
  <section className="developer-paths gg-page gg-page--mid">{paths.map(({icon:Icon,label,command,note},index)=><article key={label}><span>0{index+1}</span><Icon size={21}/><h2>{label}</h2><p>{note}</p><pre tabIndex={0}>{command}</pre></article>)}</section>
  <section className="launch-callout gg-page gg-page--mid"><div><p className="site-eyebrow">Same public contract</p><h2>Errors you can build around.</h2></div><p>Stable codes, documented limits, request IDs and explicit dataset versions travel with the response.</p><a href="/docs#errors">Review error handling <ArrowRight size={15}/></a></section>
  </main><MarketingFooter/></div>}
