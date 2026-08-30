"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Badge, Skeleton, verificationTone, resolveApiBase } from "@ghanageo/ui";
import { MapPin, Search } from "lucide-react";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);
type Hit = { id: string; name: string; kind: string; regionName?: string; districtName?: string };

export function HomeSearch() {
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<Hit[]>([]);
  const [loading, setLoading] = useState(false);
  const abort = useRef<AbortController | null>(null);

  const run = useCallback(async (value: string) => {
    if (value.trim().length < 2) { setHits([]); return; }
    abort.current?.abort();
    const controller = new AbortController();
    abort.current = controller;
    setLoading(true);
    try {
      const response = await fetch(`${API}/search?q=${encodeURIComponent(value)}&limit=5`, { signal: controller.signal });
      if (response.ok) setHits((await response.json()).data ?? []);
    } catch {
      // Aborted and unavailable searches keep the page usable without inventing results.
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => run(query), 200);
    return () => clearTimeout(timer);
  }, [query, run]);

  return <div className="home-search-wrap">
    <label className="home-search"><Search size={20}/><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search Osu, Kwabenya, Tema Community 25…" aria-label="Search Ghanaian places"/>{loading ? <Skeleton className="home-search__status"/> : <kbd>⌘ K</kbd>}</label>
    {loading ? <div className="home-results home-results--skeleton" role="status" aria-label="Searching places">{[0, 1, 2].map((item) => <div key={item}><Skeleton className="home-result-skeleton__icon"/><span><Skeleton/><Skeleton/></span><Skeleton className="home-result-skeleton__badge"/></div>)}</div> : hits.length > 0 ? <div className="home-results">{hits.map((hit) => { const verification = verificationTone("REFERENCE"); return <div key={hit.id}><MapPin size={16}/><span><strong>{hit.name}</strong><small>{[hit.districtName, hit.regionName].filter(Boolean).join(" · ") || hit.kind}</small></span><Badge tone={verification.tone}>{hit.kind}</Badge></div>; })}</div> : query.trim().length >= 2 ? <p className="home-no-result">No match yet. Try “kumsai” for typo-tolerant search.</p> : null}
  </div>;
}
