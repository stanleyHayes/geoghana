import { Badge } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ArrowRight, ArrowUpRight, Database, MapPin, ShieldCheck } from "lucide-react";
import { VISION, MISSION, CONTEXT, PLATFORM, AUDIENCES, UNVERIFIED } from "@/content/about";

export const metadata = {
  title: "About GhanaGeo — vision, mission and sources",
  description:
    "Why GhanaGeo exists, what it commits to, and the sourced evidence behind every figure we publish.",
};

export default function About() {
  return (
    <div className="site-page about-page">
      <MarketingHeader active="/about" />
      <main id="main">
        <section className="about-hero" aria-labelledby="about-title">
          <div className="about-hero__copy">
            <p className="site-eyebrow"><MapPin size={14} aria-hidden /> Why GhanaGeo exists</p>
            <h1 id="about-title">{VISION.headline}</h1>
          </div>
          <div className="about-hero__statement">
            <span className="about-index" aria-hidden>01 / Vision</span>
            {VISION.body.trim().split("\n\n").map((paragraph) => (
              <p key={paragraph}>{paragraph.replace(/\n/g, " ")}</p>
            ))}
            <a href="#mission">Read our commitments <ArrowRight size={16} aria-hidden /></a>
          </div>
          <div className="about-hero__terrain" aria-hidden>
            <span>Upper West</span><span>Ashanti</span><span>Greater Accra</span><i /><i /><i />
          </div>
          <div className="about-hero__principles" aria-label="GhanaGeo principles">
            <span><strong>Open</strong><small>Free to use and leave</small></span>
            <span><strong>Traceable</strong><small>A source on every record</small></span>
            <span><strong>Ghanaian</strong><small>Names preserved as written</small></span>
          </div>
        </section>

        <section id="mission" className="gg-page gg-page--mid about-mission" aria-labelledby="mission-title">
          <header className="about-section-intro">
            <div>
              <p className="site-eyebrow"><ShieldCheck size={14} aria-hidden /> Our contract</p>
              <span className="about-index" aria-hidden>02 / Mission</span>
            </div>
            <h2 id="mission-title">{MISSION.headline}</h2>
          </header>
          <div className="about-mission__grid">
            {MISSION.pillars.map((pillar, index) => (
              <article key={pillar.title}>
                <span aria-hidden>{String(index + 1).padStart(2, "0")}</span>
                <h3>{pillar.title}</h3>
                <p>{pillar.body.replace(/\n/g, " ")}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="about-context" aria-labelledby="context-title">
          <div className="gg-page gg-page--mid about-context__inner">
            <header>
              <p className="site-eyebrow"><Database size={14} aria-hidden /> The context</p>
              <h2 id="context-title">Why now?</h2>
              <p>Every figure links to its primary source and states the date it applies to. We publish nothing we cannot point at.</p>
            </header>
            <div className="about-ledger">
              {CONTEXT.map((fact, index) => (
                <article key={fact.label}>
                  <span className="about-ledger__number" aria-hidden>{String(index + 1).padStart(2, "0")}</span>
                  <strong>{fact.value}</strong>
                  <div>
                    <h3>{fact.label}</h3>
                    <a href={fact.url} rel="noopener noreferrer" target="_blank">
                      {fact.source} <ArrowUpRight size={13} aria-hidden />
                    </a>
                    <small>{fact.asOf}</small>
                    {fact.caveat ? <p>{fact.caveat}</p> : null}
                  </div>
                </article>
              ))}
            </div>
          </div>
        </section>

        <section className="gg-page gg-page--mid about-audiences" aria-labelledby="audiences-title">
          <header className="about-section-intro">
            <div><p className="site-eyebrow">Built for real work</p><span className="about-index" aria-hidden>03 / People</span></div>
            <h2 id="audiences-title">Who this is for</h2>
          </header>
          <div className="about-audiences__list">
            {AUDIENCES.map((audience, index) => (
              <article key={audience.who}>
                <span aria-hidden>{String(index + 1).padStart(2, "0")}</span>
                <h3>{audience.who}</h3>
                <p>{audience.breaks}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="gg-page gg-page--mid about-platform" aria-labelledby="platform-title">
          <div className="about-platform__visual" aria-hidden>
            <span>digitalghana.dev</span><i /><i /><i />
          </div>
          <div className="about-platform__copy">
            <p className="site-eyebrow">The wider platform</p>
            <h2 id="platform-title">{PLATFORM.headline}</h2>
            {PLATFORM.body.trim().split("\n\n").map((paragraph) => (
              <p key={paragraph}>{paragraph.replace(/\n/g, " ")}</p>
            ))}
            <div className="about-platform__actions">
              <a className="site-button site-button--primary" href="/docs">Read the documentation <ArrowRight size={16} aria-hidden /></a>
              <a className="site-button" href="http://localhost:3101">Try the API <ArrowUpRight size={16} aria-hidden /></a>
            </div>
          </div>
        </section>

        <section className="about-restraint" aria-labelledby="restraint-title">
          <div className="gg-page gg-page--mid">
            <header>
              <div><Badge tone="needsRecon">Editorial restraint</Badge><span className="about-index" aria-hidden>04 / Disclosures</span></div>
              <div><h2 id="restraint-title">What we chose not to claim</h2><p>Figures we considered and left out, with the reason. If the pitch is provenance, the pitch has to apply to the pitch.</p></div>
            </header>
            <div className="about-restraint__list">
              {UNVERIFIED.map((item, index) => (
                <article key={item.claim}>
                  <span aria-hidden>{String(index + 1).padStart(2, "0")}</span>
                  <h3>{item.claim}</h3>
                  <p>{item.why.replace(/\n/g, " ")}</p>
                </article>
              ))}
            </div>
          </div>
        </section>
      </main>
      <MarketingFooter />
    </div>
  );
}
