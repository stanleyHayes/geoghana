"use client";

import { ChevronRight, PanelLeftClose, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { cn } from "../lib/utils";
import { filterNav, groupBadge, type NavGroup, type Role } from "./types";

const STORAGE_KEY = "ghanageo-sidebar";

interface Persisted {
  collapsed: boolean;
  openGroups: string[];
}

function readPersisted(groups: readonly NavGroup[]): Persisted {
  const fallback = {
    collapsed: false,
    openGroups: groups.filter((g) => g.defaultOpen).map((g) => g.id),
  };
  if (typeof window === "undefined") return fallback;
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return raw ? { ...fallback, ...(JSON.parse(raw) as Partial<Persisted>) } : fallback;
  } catch {
    return fallback;
  }
}

export interface SidebarProps {
  groups: readonly NavGroup[];
  role: Role;
  pathname: string;
  collapsed: boolean;
  onNavigate?: ((href: string) => void) | undefined;
  mobileOpen?: boolean;
  onMobileClose?: () => void;
  brand?: React.ReactNode;
  footer?: React.ReactNode;
}

export function Sidebar({
  groups, role, pathname, collapsed, onNavigate,
  mobileOpen = false, onMobileClose, brand, footer,
}: SidebarProps) {
  const visible = useMemo(() => filterNav(groups, role), [groups, role]);
  const [open, setOpen] = useState<string[]>(() => readPersisted(groups).openGroups);
  // Lets a user shut a group that force-opened because it holds the active
  // route — but only for that exact path, so it reopens on any navigation.
  const [dismissedPath, setDismissedPath] = useState<string | null>(null);

  useEffect(() => {
    setDismissedPath(null);
  }, [pathname]);

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ collapsed, openGroups: open }));
    } catch {
      /* storage blocked — collapse state simply will not persist */
    }
  }, [collapsed, open]);

  /**
   * Exactly one rail item is active: the one whose href is the LONGEST prefix
   * of the current path.
   *
   * A plain prefix test lights every ancestor, so `/ingest/duplicates` also
   * activated `/ingest` ("Pipeline Overview") and the rail claimed you were on
   * a page you were not. Longest-match keeps the useful half of prefix
   * matching — a detail route like `/geography/places/01KDVDNA00N6BFFK8VF5K8YXPW` still
   * highlights "Places" — while a sibling section index never steals it.
   */
  const activeHref = useMemo(() => {
    let best: string | null = null;
    for (const group of groups) {
      for (const item of group.items) {
        const hit = item.exact
          ? pathname === item.href
          : pathname === item.href || pathname.startsWith(item.href + "/");
        if (hit && (best === null || item.href.length > best.length)) best = item.href;
      }
    }
    return best;
  }, [groups, pathname]);

  const isActive = useCallback((href: string) => href === activeHref, [activeHref]);

  const toggleGroup = (id: string, hasActive: boolean) => {
    if (hasActive && !dismissedPath) {
      setDismissedPath(pathname);
      return;
    }
    setOpen((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  // The mobile drawer always shows the expanded presentation, regardless of
  // the persisted collapse state.
  const isCollapsed = collapsed && !mobileOpen;

  return (
    <>
      {mobileOpen ? (
        <div
          className="gg-sidebar__scrim"
          onClick={onMobileClose}
          aria-hidden
        />
      ) : null}

      <aside
        className={cn("gg-sidebar", isCollapsed && "gg-sidebar--collapsed", mobileOpen && "gg-sidebar--mobile-open")}
        aria-label="Primary navigation"
        data-intensity="balanced"
      >
        <div className="gg-sidebar__brand">
          {brand}
          {mobileOpen ? (
            <button className="gg-button gg-button--ghost gg-button--icon" onClick={onMobileClose} aria-label="Close navigation">
              <X size={18} />
            </button>
          ) : null}
        </div>

        <nav className="gg-sidebar__nav">
          {visible.map((group) => {
            const hasActive = group.items.some((i) => isActive(i.href));
            const forcedOpen = hasActive && dismissedPath !== pathname;
            const expanded = open.includes(group.id) || forcedOpen;
            const rollup = groupBadge(group);

            // A group left with one item renders flat — no header, no accordion.
            if (group.items.length === 1 || group.label === null) {
              return (
                <div key={group.id} className="gg-sidebar__group">
                  {group.items.map((item) => (
                    <NavLink
                      key={item.id} item={item} collapsed={isCollapsed}
                      active={isActive(item.href)} onNavigate={onNavigate}
                    />
                  ))}
                </div>
              );
            }

            return (
              <div key={group.id} className="gg-sidebar__group">
                <button
                  type="button"
                  className={cn("gg-sidebar__group-header", hasActive && "is-active")}
                  aria-expanded={expanded}
                  aria-controls={`nav-group-${group.id}`}
                  onClick={() => toggleGroup(group.id, hasActive)}
                  title={isCollapsed ? group.label : undefined}
                >
                  <group.icon size={16} className="gg-sidebar__group-icon" aria-hidden />
                  {!isCollapsed ? (
                    <>
                      <span className="gg-sidebar__group-label">{group.label}</span>
                      {rollup > 0 ? <span className="gg-sidebar__rollup">{rollup > 99 ? "99+" : rollup}</span> : null}
                      <ChevronRight size={14} className={cn("gg-sidebar__chevron", expanded && "is-open")} aria-hidden />
                    </>
                  ) : rollup > 0 ? (
                    <span className="gg-sidebar__rollup-dot" aria-hidden />
                  ) : null}
                </button>

                <div id={`nav-group-${group.id}`} className={cn("gg-sidebar__items", expanded && "is-open")}>
                  <div className="gg-sidebar__items-inner">
                    {group.items.map((item) => (
                      <NavLink
                        key={item.id} item={item} collapsed={isCollapsed}
                        active={isActive(item.href)} onNavigate={onNavigate}
                      />
                    ))}
                  </div>
                </div>
              </div>
            );
          })}
        </nav>

        {footer ? <div className="gg-sidebar__footer">{footer}</div> : null}
      </aside>
    </>
  );
}

function NavLink({
  item, collapsed, active, onNavigate,
}: {
  item: NavGroup["items"][number];
  collapsed: boolean;
  active: boolean;
  onNavigate?: ((href: string) => void) | undefined;
}) {
  return (
    <a
      href={item.href}
      aria-current={active ? "page" : undefined}
      title={collapsed ? item.label : undefined}
      className={cn("gg-navlink", active && "is-active", collapsed && "gg-navlink--collapsed")}
      onClick={(e) => {
        if (onNavigate) {
          e.preventDefault();
          onNavigate(item.href);
        }
      }}
    >
      <item.icon size={17} className="gg-navlink__icon" aria-hidden />
      {!collapsed ? (
        <>
          <span className="gg-navlink__label">{item.label}</span>
          {item.badge ? <span className="gg-navlink__badge">{item.badge > 99 ? "99+" : item.badge}</span> : null}
        </>
      ) : item.badge ? (
        <span className="gg-navlink__badge-dot" aria-hidden />
      ) : null}
      {collapsed ? <span className="gg-sr-only">{item.label}</span> : null}
    </a>
  );
}
