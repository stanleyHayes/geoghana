export type Material = "neu" | "glass" | "clay";
export type Mode = "light" | "dark";
export type ModePreference = Mode | "system";
export type Density = "comfortable" | "compact";

export interface ThemeState {
  material: Material;
  mode: ModePreference;
  brandH: number;
  brandC: number;
  density: Density;
}

export const STORAGE_KEY = "ghanageo-theme";

export const DEFAULT_THEME: ThemeState = {
  material: "glass", // the only material rated fully viable in both modes
  mode: "system",
  brandH: 168, // "meridian" — 3deg clear of the reserved success arc
  brandC: 0.115,
  density: "comfortable",
};

export interface BrandPreset {
  id: string;
  label: string;
  h: number;
  c: number;
}

/** Every preset sits outside both reserved arcs by construction. */
export const BRAND_PRESETS: readonly BrandPreset[] = [
  { id: "meridian", label: "Meridian", h: 168, c: 0.115 },
  { id: "lagoon", label: "Lagoon", h: 232, c: 0.12 },
  { id: "terracotta", label: "Terracotta", h: 55, c: 0.105 },
  { id: "kente", label: "Kente", h: 118, c: 0.13 },
  { id: "ink", label: "Ink", h: 268, c: 0.09 },
];

export const MATERIALS: ReadonlyArray<{ id: Material; label: string; hint: string }> = [
  { id: "neu", label: "Soft", hint: "Extruded from the page" },
  { id: "glass", label: "Frost", hint: "Frosted panes over depth" },
  { id: "clay", label: "Clay", hint: "Inflated and soft" },
];
