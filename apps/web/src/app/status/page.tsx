import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { StatusClient } from "./status-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({ title: "Service status", description: "Check the current availability, response time, dataset version and incident history of GhanaGeo's public services.", path: "/status" });

export default function StatusPage(){return <div className="site-page"><MarketingHeader active="/status"/><main id="main" className="launch-page"><section className="launch-hero gg-page gg-page--mid"><p className="site-eyebrow">Service status</p><h1>Operational truth,<br/><em>without the green theatre.</em></h1><p>This page checks the public API from your browser. A failed check is shown as unknown—not quietly rewritten as operational.</p></section><StatusClient/></main><MarketingFooter/></div>}
