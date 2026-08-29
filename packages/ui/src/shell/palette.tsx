"use client";

import { Command } from "cmdk";
import { CornerDownLeft, MapPin, Search } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Badge, Skeleton, verificationTone } from "../components/primitives";
import { filterNav, type NavGroup, type Role } from "./types";

/** A geographic result. Hierarchy is always shown, because "Osu" alone is
 *  ambiguous and Spec 10 requires ambiguity to surface, not be guessed away. */
export interface PlaceResult {
  id: string;
  name: string;
  type: string;
  regionName?: string;
  districtName?: string;
  verificationStatus: string;
  score?: number;
  matchReason?: string;
}

export interface PaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  groups: readonly NavGroup[];
  role: Role;
  onNavigate: (href: string) => void;
  /** Debounced, abortable search. Returning [] is a valid, useful answer. */
  search: (query: string, signal: AbortSignal) => Promise<PlaceResult[]>;
  onCreatePlace?: (query: string) => void;
}

const MIN_QUERY = 2;
const DEBOUNCE_MS = 180;

export function CommandPalette({
  open, onOpenChange, groups, role, onNavigate, search, onCreatePlace,
}: PaletteProps) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<PlaceResult[]>([]);
  const [loading, setLoading] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const routes = useMemo(
    () => filterNav(groups, role).flatMap((g) => g.items.map((i) => ({ ...i, group: g.label ?? "Overview" }))),
    [groups, role],
  );

  // Cmd-K from anywhere.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        onOpenChange(!open);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onOpenChange]);

  // Debounced search; every keystroke aborts the previous request.
  useEffect(() => {
    const q = query.trim();
    if (q.length < MIN_QUERY) {
      abortRef.current?.abort();
      setResults([]);
      setLoading(false);
      return;
    }
    const t = setTimeout(() => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      setLoading(true);
      search(q, controller.signal)
        .then((r) => {
          if (!controller.signal.aborted) setResults(r);
        })
        .catch(() => {
          /* aborted or failed — an empty result set is the honest answer */
        })
        .finally(() => {
          if (!controller.signal.aborted) setLoading(false);
        });
    }, DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [query, search]);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setResults([]);
    }
  }, [open]);

  if (!open) return null;

  const q = query.trim();
  const showZeroState = q.length >= MIN_QUERY && !loading && results.length === 0;

  return (
    <div className="gg-palette__overlay" onClick={() => onOpenChange(false)} role="presentation">
      <div className="gg-palette" onClick={(e) => e.stopPropagation()}>
        <Command shouldFilter={false} label="Global search">
          <div className="gg-palette__input-row">
            <Search size={17} aria-hidden />
            <Command.Input
              autoFocus
              value={query}
              onValueChange={setQuery}
              placeholder="Search places, districts, keys, runs…"
              className="gg-palette__input"
            />
            <kbd className="gg-kbd">Esc</kbd>
          </div>

          <Command.List className="gg-palette__list">
            {q.length < MIN_QUERY ? (
              <Command.Group heading="Go to" className="gg-palette__group">
                {routes.slice(0, 8).map((r) => (
                  <Command.Item
                    key={r.id}
                    value={r.id}
                    onSelect={() => {
                      onNavigate(r.href);
                      onOpenChange(false);
                    }}
                    className="gg-palette__item"
                  >
                    <r.icon size={16} aria-hidden />
                    <span>{r.label}</span>
                    <span className="gg-palette__hint">{r.group}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            ) : null}

            {loading ? (
              <div className="gg-palette__skeleton" role="status" aria-label="Searching places">
                {[0, 1, 2].map((item) => (
                  <div key={item}>
                    <Skeleton className="gg-palette__skeleton-icon" />
                    <span><Skeleton /><Skeleton /></span>
                    <Skeleton className="gg-palette__skeleton-badge" />
                  </div>
                ))}
              </div>
            ) : null}

            {results.length > 0 ? (
              <Command.Group heading="Places" className="gg-palette__group">
                {results.map((p) => {
                  const v = verificationTone(p.verificationStatus);
                  const hierarchy = [p.type, p.districtName, p.regionName].filter(Boolean).join(" · ");
                  return (
                    <Command.Item
                      key={p.id}
                      value={p.id}
                      onSelect={() => {
                        onNavigate(`/geography/places/${p.id}`);
                        onOpenChange(false);
                      }}
                      className="gg-palette__item"
                    >
                      <MapPin size={16} aria-hidden />
                      <span className="gg-palette__name">{p.name}</span>
                      <span className="gg-palette__hierarchy">{hierarchy}</span>
                      <Badge tone={v.tone}>
                        <span aria-hidden>{v.glyph}</span> {v.label}
                      </Badge>
                      {p.matchReason ? <span className="gg-palette__hint">{p.matchReason}</span> : null}
                    </Command.Item>
                  );
                })}
              </Command.Group>
            ) : null}

            {/* Zero results is a feature, not a dead end: it feeds the
                zero-result queue, the product's best data-acquisition loop. */}
            {showZeroState ? (
              <div className="gg-palette__zero">
                <p className="gg-palette__zero-title">No match for “{q}”</p>
                <div className="gg-palette__zero-actions">
                  {onCreatePlace ? (
                    <button className="gg-button gg-button--secondary gg-button--sm" onClick={() => onCreatePlace(q)}>
                      Create this place
                    </button>
                  ) : null}
                  <button
                    className="gg-button gg-button--ghost gg-button--sm"
                    onClick={() => {
                      onNavigate("/search-ops/gaps");
                      onOpenChange(false);
                    }}
                  >
                    Log as a zero-result query
                  </button>
                </div>
              </div>
            ) : null}
          </Command.List>

          <div className="gg-palette__footer">
            <span><kbd className="gg-kbd">↑</kbd><kbd className="gg-kbd">↓</kbd> navigate</span>
            <span><kbd className="gg-kbd"><CornerDownLeft size={11} /></kbd> open</span>
            <span><kbd className="gg-kbd">Esc</kbd> close</span>
          </div>
        </Command>
      </div>
    </div>
  );
}
