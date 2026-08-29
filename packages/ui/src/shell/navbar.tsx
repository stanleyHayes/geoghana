"use client";

import {
  Activity, Bell, ChevronRight, CircleHelp, ClipboardCheck, Grid3x3, Menu,
  Heart, Package, PanelLeftClose, PanelLeftOpen, Plus, Search,
} from "lucide-react";
import type { ReactNode } from "react";
import { ThemeMenu } from "../theme/menu";
import { cn } from "../lib/utils";

export interface Crumb {
  label: string;
  href: string;
}

export type Environment = "production" | "staging" | "sandbox" | "test" | "local";

const ENV_LABEL: Record<Environment, string> = {
  production: "Production", staging: "Staging", sandbox: "Sandbox",
  test: "Test", local: "Local",
};

export interface NavbarProps {
  crumbs: Crumb[];
  collapsed: boolean;
  onToggleCollapse: () => void;
  onOpenMobileNav: () => void;
  onOpenPalette: () => void;
  datasetVersion: string;
  datasetStatus: "published" | "working";
  environment: Environment;
  pipelineHealth: "ok" | "lagging" | "failing";
  reviewCount: number;
  notificationCount: number;
  userMenu?: ReactNode;
  canCreate?: boolean;
  supportHref?: string;
}

export function Navbar({
  crumbs, collapsed, onToggleCollapse, onOpenMobileNav, onOpenPalette,
  datasetVersion, datasetStatus, environment, pipelineHealth,
  reviewCount, notificationCount, userMenu, canCreate = true,
  supportHref = "/support",
}: NavbarProps) {
  return (
    <header
      className={cn("gg-navbar", environment === "production" && "gg-navbar--production")}
      data-intensity="balanced"
    >
      {/* LEFT — context */}
      <div className="gg-navbar__left">
        <button
          className="gg-button gg-button--ghost gg-button--icon gg-navbar__mobile-only"
          onClick={onOpenMobileNav}
          aria-label="Open navigation"
          title="Open navigation"
        >
          <Menu size={20} />
        </button>

        <button
          className="gg-button gg-button--ghost gg-button--icon gg-navbar__desktop-only"
          onClick={onToggleCollapse}
          aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
        >
          {collapsed ? <PanelLeftOpen size={18} /> : <PanelLeftClose size={18} />}
        </button>

        <button
          className="gg-button gg-button--ghost gg-button--icon gg-navbar__desktop-only"
          aria-label="Switch app"
          title="Admin · Portal · Sandbox · Docs"
        >
          <Grid3x3 size={18} />
        </button>

        {/* Breadcrumb: each segment is a dropdown of its siblings, so a steward
            moves laterally between districts without returning to a list. */}
        <nav className="gg-crumbs" aria-label="Breadcrumb">
          <ol>
            {crumbs.map((c, i) => (
              <li key={c.href}>
                {i > 0 ? <ChevronRight size={13} aria-hidden className="gg-crumbs__sep" /> : null}
                <a href={c.href} aria-current={i === crumbs.length - 1 ? "page" : undefined}>
                  {c.label}
                </a>
              </li>
            ))}
          </ol>
        </nav>
      </div>

      {/* CENTRE — search. The correction: all three reference shells lack a
          working global search, and this product is about finding places. */}
      <div className="gg-navbar__centre">
        <button className="gg-searchbar" onClick={onOpenPalette} aria-label="Search places, districts, keys, runs">
          <Search size={16} aria-hidden />
          <span className="gg-searchbar__placeholder">Search places, districts, keys, runs…</span>
          <kbd className="gg-kbd">⌘K</kbd>
        </button>
      </div>

      {/* RIGHT — actions */}
      <div className="gg-navbar__right">
        {canCreate ? (
          <button className="gg-button gg-button--primary gg-button--sm" aria-label="Create">
            <Plus size={16} aria-hidden />
            <span className="gg-navbar__desktop-only">Create</span>
          </button>
        ) : null}

        <button
          className="gg-dataset-pill"
          title="Dataset version — changes what every screen reads"
          aria-label={`Dataset ${datasetVersion}, ${datasetStatus}`}
        >
          <Package size={15} aria-hidden />
          <span className="gg-dataset-pill__text">
            <span className="gg-dataset-pill__version">{datasetVersion}</span>
            <span className="gg-dataset-pill__status">{datasetStatus}</span>
          </span>
        </button>

        <span className={cn("gg-env", `gg-env--${environment}`)} title={`Environment: ${ENV_LABEL[environment]}`}>
          {ENV_LABEL[environment]}
        </span>

        <button
          className={cn("gg-button gg-button--ghost gg-button--icon gg-health", `gg-health--${pipelineHealth}`)}
          aria-label={`Pipeline health: ${pipelineHealth}`}
          title={`Pipeline health: ${pipelineHealth}`}
        >
          <Activity size={18} aria-hidden />
          <span className="gg-health__dot" aria-hidden />
        </button>

        {reviewCount > 0 ? (
          <button className="gg-button gg-button--ghost gg-button--icon gg-counter" aria-label={`Review queue: ${reviewCount} items`} title="Review queue">
            <ClipboardCheck size={18} aria-hidden />
            <span className="gg-counter__badge">{reviewCount > 99 ? "99+" : reviewCount}</span>
          </button>
        ) : null}

        <button className="gg-button gg-button--ghost gg-button--icon gg-counter" aria-label={`Notifications: ${notificationCount} unread`} title="Notifications">
          <Bell size={18} aria-hidden />
          {notificationCount > 0 ? (
            <span className="gg-counter__badge">{notificationCount > 99 ? "99+" : notificationCount}</span>
          ) : null}
        </button>

        {/* A plain link. It must never become a modal, and never appear on a
            429 page where it would read as "pay to continue" (rule F5). */}
        <a
          className="gg-button gg-button--ghost gg-button--icon gg-navbar__desktop-only"
          href={supportHref}
          aria-label="Support GhanaGeo"
          title="Support GhanaGeo — it is free and stays free"
        >
          <Heart size={18} aria-hidden />
        </a>

        <button className="gg-button gg-button--ghost gg-button--icon gg-navbar__desktop-only" aria-label="Help" title="Help">
          <CircleHelp size={18} aria-hidden />
        </button>

        {/* All three axes, not just mode — the same control every other app
            carries, so the theme lives in one place across the platform. The
            signature circular wipe still fires from the click point inside it. */}
        <ThemeMenu />

        {userMenu}
      </div>
    </header>
  );
}
