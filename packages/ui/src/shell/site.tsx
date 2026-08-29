"use client";

import * as Dialog from "@radix-ui/react-dialog";
import {
  BookOpen, ExternalLink, FlaskConical, Heart, Info, Menu, Receipt, Terminal, X,
} from "lucide-react";
import type { ComponentType, ReactNode } from "react";
import { Logo } from "../components/logo";
import { ThemeMenu } from "../theme/menu";

/**
 * One header, one drawer and one footer for every public page.
 *
 * These exist because hand-rolling a navbar per page went exactly how
 * DESIGN_SYSTEM.md §1.3 warns: the Docs link was dropped from one page during a
 * responsive pass and the page became unreachable, /about linked only to home,
 * and "Where the money goes" pointed at a page nobody had built. Navigation
 * defined in five places drifts in five directions.
 *
 * On mobile the links move into a DRAWER rather than a dropdown, so the full
 * set fits — primary pages, secondary pages and external links — instead of
 * only what a collapsed bar can hold.
 */

export interface NavLink {
  href: string;
  label: string;
  hint?: string;
  icon?: ComponentType<{ size?: number | string }>;
  external?: boolean;
}

/** Shown in the header bar on wide screens. */
export const PRIMARY_NAV: readonly NavLink[] = [
  { href: "/docs", label: "Docs", hint: "Quick starts and endpoints", icon: BookOpen },
  { href: "/about", label: "About", hint: "Vision, mission and sources", icon: Info },
  { href: "/support", label: "Support", hint: "How this stays free", icon: Heart },
];

/** Drawer and footer only — real pages that do not earn a slot in the bar. */
export const SECONDARY_NAV: readonly NavLink[] = [
  { href: "/transparency", label: "Transparency", hint: "Income, costs and what they funded", icon: Receipt },
];

export const TOOL_NAV: readonly NavLink[] = [
  { href: "http://localhost:3101", label: "Sandbox", hint: "Try the API with no account", icon: FlaskConical },
  { href: "http://localhost:3102", label: "Developer console", hint: "Keys, usage and logs", icon: Terminal },
];

function isCurrent(active: string | undefined, href: string) {
  return active === href ? "page" : undefined;
}

export function SiteHeader({
  active,
  suffix,
  sandboxUrl = "http://localhost:3101",
}: {
  active?: string | undefined;
  suffix?: string | undefined;
  sandboxUrl?: string | undefined;
}) {
  return (
    <header className="gg-navbar" data-intensity="balanced">
      <div className="gg-navbar__left">
        <a href="/" className="gg-logo-link" style={{ textDecoration: "none" }} aria-label="GhanaGeo home">
          <Logo size={24} {...(suffix ? { suffix } : {})} />
        </a>
      </div>

      <nav className="gg-sitenav gg-navbar__hide-sm" aria-label="Primary">
        {PRIMARY_NAV.map((l) => (
          <a key={l.href} href={l.href} className="gg-sitenav__link" aria-current={isCurrent(active, l.href)}>
            {l.label}
          </a>
        ))}
      </nav>

      <div className="gg-navbar__right">
        {/* Theme is a product feature here, not a preference buried in admin:
            the tri-morphic system is what a visitor is being shown. */}
        <ThemeMenu />
        <a className="gg-button gg-button--primary gg-button--sm gg-navbar__hide-xs" href={sandboxUrl}>
          Sandbox
        </a>
        <NavDrawer active={active} />
      </div>
    </header>
  );
}

/**
 * The mobile navigation drawer.
 *
 * Radix Dialog rather than a hand-rolled panel: it brings the focus trap,
 * Escape to close, body scroll lock, `aria-modal` and focus restoration on
 * close. Each of those is easy to forget and individually makes the drawer
 * unusable with a keyboard or a screen reader.
 */
function NavDrawer({ active }: { active?: string | undefined }) {
  return (
    <Dialog.Root>
      <Dialog.Trigger asChild>
        <button className="gg-button gg-button--ghost gg-button--icon gg-drawer__toggle" aria-label="Open navigation">
          <Menu size={20} />
        </button>
      </Dialog.Trigger>
      <Dialog.Portal>
        <Dialog.Overlay className="gg-drawer__overlay" />
        <Dialog.Content className="gg-drawer" aria-describedby={undefined}>
          <div className="gg-drawer__head">
            <Dialog.Title asChild>
              <span><Logo size={22} /></span>
            </Dialog.Title>
            <Dialog.Close asChild>
              <button className="gg-button gg-button--ghost gg-button--icon" aria-label="Close navigation">
                <X size={20} />
              </button>
            </Dialog.Close>
          </div>

          <nav className="gg-drawer__nav" aria-label="Site">
            <p className="gg-drawer__heading">Product</p>
            {PRIMARY_NAV.map((l) => (
              <DrawerLink key={l.href} link={l} active={active} />
            ))}

            <p className="gg-drawer__heading">More</p>
            {SECONDARY_NAV.map((l) => (
              <DrawerLink key={l.href} link={l} active={active} />
            ))}

            <p className="gg-drawer__heading">Tools</p>
            {TOOL_NAV.map((l) => (
              <DrawerLink key={l.href} link={l} active={active} />
            ))}
          </nav>

          <div className="gg-drawer__foot">
            <a className="gg-button gg-button--primary gg-button--md" href="http://localhost:3101">
              Try the sandbox
            </a>
            <p className="gg-drawer__note">
              Free. No account, no API key, no paid tier.
            </p>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function DrawerLink({ link, active }: { link: NavLink; active?: string | undefined }) {
  const Icon = link.icon;
  const external = link.href.startsWith("http");
  return (
    <Dialog.Close asChild>
      <a
        href={link.href}
        className="gg-drawer__link"
        aria-current={isCurrent(active, link.href)}
        {...(external ? { rel: "noopener noreferrer" } : {})}
      >
        {Icon ? <Icon size={18} /> : null}
        <span className="gg-drawer__labels">
          <span className="gg-drawer__label">{link.label}</span>
          {link.hint ? <span className="gg-drawer__hint">{link.hint}</span> : null}
        </span>
        {external ? <ExternalLink size={13} aria-hidden /> : null}
      </a>
    </Dialog.Close>
  );
}

export function SiteFooter({ children }: { children?: ReactNode }) {
  return (
    <footer className="gg-sitefooter">
      <div className="gg-page gg-page--mid" style={{ padding: 0 }}>
        <div className="gg-sitefooter__grid">
          <div>
            <Logo size={22} />
            <p className="gg-sitefooter__lede">
              Free public infrastructure for Ghana. The first product on{" "}
              <strong>digitalghana.dev</strong>.
            </p>
          </div>
          <nav aria-label="Footer">
            <p className="gg-sitefooter__heading">Product</p>
            {[...PRIMARY_NAV, ...SECONDARY_NAV].map((l) => (
              <a key={l.href} href={l.href} className="gg-sitefooter__link">
                {l.label}
              </a>
            ))}
          </nav>
          <nav aria-label="Tools">
            <p className="gg-sitefooter__heading">Tools</p>
            {TOOL_NAV.map((l) => (
              <a key={l.href} href={l.href} className="gg-sitefooter__link" rel="noopener noreferrer">
                {l.label}
              </a>
            ))}
          </nav>
        </div>
        {children}
        <p className="gg-sitefooter__attribution">
          Contains data from GeoNames and geoBoundaries, licensed CC BY 4.0.
        </p>
      </div>
    </footer>
  );
}
