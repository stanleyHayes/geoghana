import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { StatusClient } from "./status-client";

export default function StatusPage(){return <div className="site-page"><MarketingHeader active="/status"/><main id="main" className="launch-page"><section className="launch-hero gg-page gg-page--mid"><p className="site-eyebrow">Service status</p><h1>Operational truth,<br/><em>without the green theatre.</em></h1><p>This page checks the public API from your browser. A failed check is shown as unknown—not quietly rewritten as operational.</p></section><StatusClient/></main><MarketingFooter/></div>}
