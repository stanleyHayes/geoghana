"use client";

import { ThemeMenu } from "@ghanageo/ui";
import {
  ArrowUpRight,
  BookOpenText,
  Database,
  FlaskConical,
  Heart,
  Info,
  Menu,
  ReceiptText,
  X,
} from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { sandboxOrigin } from "@/lib/seo";

const primary = [
  { href: "/products", label: "Products" },
  { href: "/coverage", label: "Coverage" },
  { href: "/developers", label: "Developers" },
  { href: "/docs", label: "Documentation" },
];

const footerLinks = [
  { href: "/docs", label: "Start building", note: "REST, GraphQL, gRPC and CLI", icon: BookOpenText },
  { href: sandboxOrigin, label: "Open sandbox", note: "Run a request without an account", icon: FlaskConical },
  { href: "/about", label: "How the data works", note: "Sources, limits and governance", icon: Database },
  { href: "/support", label: "Support the public good", note: "Costs, donations and sponsors", icon: Heart },
];

export function MarketingHeader({ active }: { active?: string }) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const close = (event: KeyboardEvent) => event.key === "Escape" && setOpen(false);
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, [open]);

  return (
    <header className="marketing-header" data-intensity="expressive">
      <div className="marketing-header__inner">
        <a className="marketing-brand" href="/">
          <span className="marketing-brand__mark" aria-hidden>GG</span>
          <span>
            <strong>GhanaGeo</strong>
            <small>Public location infrastructure</small>
          </span>
        </a>

        <nav className="marketing-nav" aria-label="Primary navigation">
          {primary.map((item) => (
            <a key={item.href} href={item.href} aria-current={active === item.href ? "page" : undefined}>
              {item.label}
            </a>
          ))}
        </nav>

        <div className="marketing-header__actions">
          <ThemeMenu />
          <a className="marketing-header__sandbox" href={sandboxOrigin}>
            Try the API <ArrowUpRight size={15} aria-hidden />
          </a>
          <button
            className="marketing-menu-button"
            type="button"
            aria-label={open ? "Close navigation" : "Open navigation"}
            aria-expanded={open}
            aria-controls="marketing-mobile-nav"
            onClick={() => setOpen((value) => !value)}
          >
            {open ? <X size={20} aria-hidden /> : <Menu size={20} aria-hidden />}
          </button>
        </div>
      </div>

      {open ? (
        <nav id="marketing-mobile-nav" className="marketing-mobile-nav" aria-label="Mobile navigation">
          {primary.map((item, index) => (
            <a key={item.href} href={item.href} aria-current={active === item.href ? "page" : undefined}>
              <span>0{index + 1}</span>{item.label}
            </a>
          ))}
          <a href="/support"><span>0{primary.length + 1}</span>Support GhanaGeo</a>
          <a className="marketing-mobile-nav__cta" href={sandboxOrigin}>
            Open the sandbox <ArrowUpRight size={17} aria-hidden />
          </a>
        </nav>
      ) : null}
    </header>
  );
}

export function MarketingFooter({ children }: { children?: ReactNode }) {
  return (
    <footer className="marketing-footer">
      <div className="marketing-footer__glow" aria-hidden />
      <div className="gg-page gg-page--mid marketing-footer__inner">
        <div className="marketing-footer__lead">
          <div>
            <p className="marketing-footer__eyebrow">Built here. Open to everyone.</p>
            <h2>Ghana should not need to guess where Ghana is.</h2>
          </div>
          <p>
            One dependable layer for regions, districts, localities and boundaries — free to
            query, clear about its sources, and designed for the ways Ghanaian names are written.
          </p>
        </div>

        <div className="marketing-footer__links">
          {footerLinks.map((item) => {
            const Icon = item.icon;
            return (
              <a key={item.href} href={item.href}>
                <Icon size={18} aria-hidden />
                <span><strong>{item.label}</strong><small>{item.note}</small></span>
                <ArrowUpRight size={15} aria-hidden />
              </a>
            );
          })}
        </div>

        {children ? <div className="marketing-footer__custom">{children}</div> : null}

        <div className="marketing-footer__base">
          <div className="marketing-footer__identity">
            <span className="marketing-brand__mark" aria-hidden>GG</span>
            <span><strong>GhanaGeo</strong><small>A digitalghana.dev public good</small></span>
          </div>
          <p>GeoNames + geoBoundaries · CC BY 4.0</p>
          <nav aria-label="Footer policies">
            <a href="/status">Status</a>
            <a href="/changelog">Changelog</a>
            <a href="/contact">Contact</a>
            <a href="/about"><Info size={14} aria-hidden /> About</a>
            <a href="/transparency"><ReceiptText size={14} aria-hidden /> Public ledger</a>
          </nav>
        </div>
      </div>
    </footer>
  );
}
