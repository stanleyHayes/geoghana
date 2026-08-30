"use client";

import { useEffect, useRef, useState } from "react";
import type { Map as LeafletMap } from "leaflet";
import type { BoundaryFeature, Coordinate } from "@/lib/api";

export interface ExplorerMapItem {
  id: string;
  name: string;
  kind: "region" | "district" | "place";
  centroid: Coordinate;
}

export interface ExplorerViewport {
  latitude: number;
  longitude: number;
  zoom: number;
}

interface ExplorerMapProps {
  items: ExplorerMapItem[];
  boundary: BoundaryFeature | null;
  selectedId?: string | undefined;
  viewport: ExplorerViewport;
  onSelect: (id: string) => void;
  onViewportChange: (viewport: ExplorerViewport) => void;
}

const KIND_LABEL = { region: "region", district: "district", place: "place" } as const;

/**
 * Groups markers in a geographic grid whose cell size shrinks as the operator
 * zooms in. This keeps Leaflet responsive without asking the public API for an
 * unbounded national place collection or adding a second clustering runtime.
 */
function grouped(items: ExplorerMapItem[], zoom: number) {
  const cell = Math.max(0.018, 8 / 2 ** Math.max(zoom - 3, 0));
  const buckets = new Map<string, ExplorerMapItem[]>();
  for (const item of items) {
    const key = `${Math.round(item.centroid.latitude / cell)}:${Math.round(item.centroid.longitude / cell)}`;
    buckets.set(key, [...(buckets.get(key) ?? []), item]);
  }
  return [...buckets.values()];
}

export function ExplorerMap({
  items,
  boundary,
  selectedId,
  viewport,
  onSelect,
  onViewportChange,
}: ExplorerMapProps) {
  const element = useRef<HTMLDivElement>(null);
  const [ready, setReady] = useState(false);
  const mapRef = useRef<LeafletMap | null>(null);
  const itemLayerRef = useRef<import("leaflet").LayerGroup | null>(null);
  const boundaryLayerRef = useRef<import("leaflet").GeoJSON | null>(null);
  const callbacks = useRef({ onSelect, onViewportChange });

  useEffect(() => {
    callbacks.current = { onSelect, onViewportChange };
  }, [onSelect, onViewportChange]);

  useEffect(() => {
    if (!element.current || mapRef.current) return;
    let disposed = false;
    let map: LeafletMap | undefined;

    void import("leaflet").then((module) => {
      if (disposed || !element.current) return;
      const L = module.default;
      map = L.map(element.current, {
        zoomControl: false,
        minZoom: 6,
        maxZoom: 18,
        preferCanvas: true,
      }).setView([viewport.latitude, viewport.longitude], viewport.zoom);
      L.control.zoom({ position: "bottomright" }).addTo(map);
      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>',
        maxZoom: 19,
      }).addTo(map);
      itemLayerRef.current = L.layerGroup().addTo(map);
      map.on("moveend", () => {
        if (!map) return;
        const center = map.getCenter();
        callbacks.current.onViewportChange({ latitude: center.lat, longitude: center.lng, zoom: map.getZoom() });
      });
      mapRef.current = map;
      setReady(true);
    });

    return () => {
      disposed = true;
      map?.remove();
      mapRef.current = null;
      itemLayerRef.current = null;
      boundaryLayerRef.current = null;
    };
    // The initial viewport is deliberately read once; later map movement is
    // owned by Leaflet and reflected back into the URL.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    const center = map.getCenter();
    if (Math.abs(center.lat - viewport.latitude) > 0.0001 || Math.abs(center.lng - viewport.longitude) > 0.0001 || map.getZoom() !== viewport.zoom) {
      map.setView([viewport.latitude, viewport.longitude], viewport.zoom, { animate: false });
    }
  }, [ready, viewport]);

  useEffect(() => {
    let cancelled = false;
    void import("leaflet").then((module) => {
      const map = mapRef.current;
      const layer = itemLayerRef.current;
      if (cancelled || !map || !layer) return;
      const L = module.default;
      layer.clearLayers();
      for (const bucket of grouped(items, map.getZoom())) {
        const selected = bucket.some((item) => item.id === selectedId);
        const latitude = bucket.reduce((sum, item) => sum + item.centroid.latitude, 0) / bucket.length;
        const longitude = bucket.reduce((sum, item) => sum + item.centroid.longitude, 0) / bucket.length;
        const marker = L.circleMarker([latitude, longitude], {
          radius: bucket.length > 1 ? Math.min(19, 9 + Math.log2(bucket.length) * 2.5) : selected ? 9 : 7,
          color: selected ? "#effff9" : "#0c5d48",
          fillColor: selected ? "#0b6b51" : "#45b894",
          fillOpacity: selected ? 1 : 0.88,
          weight: selected ? 4 : 2,
        }).addTo(layer);
        const first = bucket[0]!;
        marker.bindTooltip(
          bucket.length > 1
            ? `${bucket.length} nearby ${KIND_LABEL[first.kind]}s — zoom in to separate`
            : `${first.name} · ${KIND_LABEL[first.kind]}`,
          { direction: "top", offset: [0, -7] },
        );
        if (bucket.length === 1) marker.on("click", () => callbacks.current.onSelect(first.id));
        else marker.on("click", () => map.setView([latitude, longitude], Math.min(map.getZoom() + 2, 18)));
      }
    });
    return () => { cancelled = true; };
  }, [items, ready, selectedId, viewport.zoom]);

  useEffect(() => {
    let cancelled = false;
    void import("leaflet").then((module) => {
      const map = mapRef.current;
      if (cancelled || !map) return;
      boundaryLayerRef.current?.remove();
      boundaryLayerRef.current = null;
      if (!boundary) return;
      const L = module.default;
      const layer = L.geoJSON(boundary as unknown as GeoJSON.GeoJsonObject, {
        style: { color: "#18a27f", weight: 3, fillColor: "#45b894", fillOpacity: 0.12 },
      }).addTo(map);
      boundaryLayerRef.current = layer;
      const bounds = layer.getBounds();
      if (bounds.isValid()) map.fitBounds(bounds.pad(0.12), { maxZoom: 13, animate: false });
    });
    return () => { cancelled = true; };
  }, [boundary, ready]);

  return <div ref={element} className="admin-explorer__map" aria-label="Interactive map of the selected geography" />;
}
