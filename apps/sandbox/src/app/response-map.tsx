"use client";

import { useEffect, useRef } from "react";

type Point = { latitude: number; longitude: number };

function collect(value: unknown, points: Point[], geometries: GeoJSON.GeoJsonObject[]) {
  if (!value || typeof value !== "object") return;
  const record = value as Record<string, unknown>;
  if (typeof record.latitude === "number" && typeof record.longitude === "number") {
    points.push({ latitude: record.latitude, longitude: record.longitude });
  }
  if (typeof record.geojson === "string") {
    try { geometries.push(JSON.parse(record.geojson) as GeoJSON.GeoJsonObject); } catch { /* malformed geometry stays out of the map */ }
  } else if (record.type && (record.coordinates || record.features || record.geometry)) {
    geometries.push(record as unknown as GeoJSON.GeoJsonObject);
  }
  for (const child of Object.values(record)) collect(child, points, geometries);
}

export function ResponseMap({ response }: { response: string }) {
  const element = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!element.current || !response) return;
    let cancelled = false;
    let dispose: (() => void) | undefined;

    void import("leaflet").then((module) => {
      if (cancelled || !element.current) return;
      const L = module.default;
      const points: Point[] = [];
      const geometries: GeoJSON.GeoJsonObject[] = [];
      try { collect(JSON.parse(response), points, geometries); } catch { return; }
      if (points.length === 0 && geometries.length === 0) return;

      const map = L.map(element.current, { scrollWheelZoom: false, zoomControl: true }).setView([7.9465, -1.0232], 7);
      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        maxZoom: 19,
      }).addTo(map);

      const bounds = L.latLngBounds([]);
      for (const point of points) {
        L.circleMarker([point.latitude, point.longitude], {
          radius: 7, color: "#0b6b51", fillColor: "#45b894", fillOpacity: 0.82, weight: 2,
        }).addTo(map);
        bounds.extend([point.latitude, point.longitude]);
      }
      for (const geometry of geometries) {
        const layer = L.geoJSON(geometry, { style: { color: "#159475", weight: 3, fillOpacity: 0.18 } }).addTo(map);
        bounds.extend(layer.getBounds());
      }
      if (bounds.isValid()) map.fitBounds(bounds.pad(0.3), { maxZoom: 14 });
      dispose = () => map.remove();
    });

    return () => { cancelled = true; dispose?.(); };
  }, [response]);

  if (!response) return null;
  return <section className="sandbox-map-panel" aria-label="Map of response locations"><div ref={element} className="sandbox-map" /><p>Map data © OpenStreetMap contributors · response geometry is rendered as GeoJSON.</p></section>;
}
