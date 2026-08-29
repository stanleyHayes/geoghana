"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { flushSync } from "react-dom";
import { clampBrand } from "./clamp";
import { applyThemeWithReveal, type RevealOrigin } from "./reveal";
import {
  DEFAULT_THEME,
  STORAGE_KEY,
  type Density,
  type Material,
  type Mode,
  type ModePreference,
  type ThemeState,
} from "./types";

interface ThemeContextValue extends ThemeState {
  /** The mode actually in effect once "system" is resolved. */
  resolvedMode: Mode;
  /** Set when the last brand change had to be corrected for readability. */
  adjustmentNote: string | null;
  setMaterial: (m: Material, origin?: RevealOrigin) => void;
  setMode: (m: ModePreference, origin?: RevealOrigin) => void;
  toggleMode: (origin?: RevealOrigin) => void;
  setBrand: (h: number, c?: number) => void;
  setDensity: (d: Density) => void;
  reset: () => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

function readStored(): Partial<ThemeState> {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Partial<ThemeState>) : {};
  } catch {
    // Corrupted or blocked storage must never white-screen the app.
    return {};
  }
}

function systemMode(): Mode {
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  // The inline script (theme/script.ts) has already painted the correct theme;
  // this only mirrors it into React state.
  const [state, setState] = useState<ThemeState>(DEFAULT_THEME);
  const [systemPref, setSystemPref] = useState<Mode>("light");
  const [adjustmentNote, setAdjustmentNote] = useState<string | null>(null);
  const [hydrated, setHydrated] = useState(false);

  useEffect(() => {
    setState({ ...DEFAULT_THEME, ...readStored() });
    setSystemPref(systemMode());
    setHydrated(true);
  }, []);

  // Follow the OS while the preference is "system".
  useEffect(() => {
    if (typeof window === "undefined") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setSystemPref(mq.matches ? "dark" : "light");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const resolvedMode: Mode = state.mode === "system" ? systemPref : state.mode;

  // Reflect state onto <html> and persist. One write, one JSON blob.
  useEffect(() => {
    if (!hydrated) return;
    const d = document.documentElement;
    d.dataset.material = state.material;
    d.dataset.mode = resolvedMode;
    d.dataset.density = state.density;
    d.style.colorScheme = resolvedMode;
    d.style.setProperty("--brand-h", String(state.brandH));
    d.style.setProperty("--brand-c", String(state.brandC));

    // Apply the readability clamp for the mode now in effect.
    const clamped = clampBrand(state.brandH, state.brandC, resolvedMode);
    d.style.setProperty(
      "--brand",
      `oklch(${clamped.lightness} ${clamped.chroma} ${clamped.hue})`,
    );
    d.style.setProperty(
      "--brand-fg",
      clamped.foreground === "light" ? "oklch(0.985 0 0)" : "oklch(0.145 0 0)",
    );

    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      /* storage blocked — the theme still applies for this session */
    }
    window.dispatchEvent(new CustomEvent("ghanageo-theme-change", { detail: state }));
  }, [state, resolvedMode, hydrated]);

  // Cross-tab sync.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key !== STORAGE_KEY || !e.newValue) return;
      try {
        setState({ ...DEFAULT_THEME, ...(JSON.parse(e.newValue) as Partial<ThemeState>) });
      } catch {
        /* ignore a malformed write from another tab */
      }
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const setMaterial = useCallback((material: Material, origin?: RevealOrigin) => {
    void applyThemeWithReveal(() => {
      flushSync(() => setState((s) => ({ ...s, material })));
    }, origin);
  }, []);

  const setMode = useCallback((mode: ModePreference, origin?: RevealOrigin) => {
    void applyThemeWithReveal(() => {
      flushSync(() => setState((s) => ({ ...s, mode })));
    }, origin);
  }, []);

  const toggleMode = useCallback(
    (origin?: RevealOrigin) => {
      setMode(resolvedMode === "dark" ? "light" : "dark", origin);
    },
    [resolvedMode, setMode],
  );

  const setBrand = useCallback(
    (h: number, c?: number) => {
      const chroma = c ?? state.brandC;
      const clamped = clampBrand(h, chroma, resolvedMode);
      setAdjustmentNote(clamped.adjusted ? (clamped.reason ?? "Adjusted for readability.") : null);
      setState((s) => ({ ...s, brandH: clamped.hue, brandC: chroma }));
    },
    [state.brandC, resolvedMode],
  );

  const setDensity = useCallback((density: Density) => {
    setState((s) => ({ ...s, density }));
  }, []);

  const reset = useCallback(() => {
    setAdjustmentNote(null);
    setState(DEFAULT_THEME);
  }, []);

  const value = useMemo<ThemeContextValue>(
    () => ({
      ...state,
      resolvedMode,
      adjustmentNote,
      setMaterial,
      setMode,
      toggleMode,
      setBrand,
      setDensity,
      reset,
    }),
    [state, resolvedMode, adjustmentNote, setMaterial, setMode, toggleMode, setBrand, setDensity, reset],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used inside <ThemeProvider>");
  return ctx;
}
