"use client";

import { Info, RotateCcw } from "lucide-react";
import { Card } from "../components/primitives";
import { RadioGroup } from "../components/forms";
import { useTheme } from "./provider";
import { BRAND_PRESETS, MATERIALS, type Density, type ModePreference } from "./types";

/**
 * One component, mounted in admin settings, portal settings and the marketing
 * footer, so a visitor can set their preference before ever signing up.
 * (DESIGN_SYSTEM.md 8)
 */
export function ThemePicker() {
  const {
    material, mode, brandH, density, adjustmentNote,
    setMaterial, setMode, setBrand, setDensity, reset,
  } = useTheme();

  return (
    <div className="gg-picker">
      <section className="gg-picker__section">
        <h3 className="gg-picker__legend">Material</h3>
        <div className="gg-picker__materials" role="radiogroup" aria-label="Material">
          {MATERIALS.map((m) => (
            <button
              key={m.id}
              role="radio"
              aria-checked={material === m.id}
              data-material={m.id}
              className={`gg-picker__swatch ${material === m.id ? "is-selected" : ""}`}
              onClick={(e) => setMaterial(m.id, { x: e.clientX, y: e.clientY })}
            >
              {/* Each swatch renders INSIDE its own material — you see the
                  thing itself, not a label describing it. */}
              <span className="gg-picker__swatch-preview" aria-hidden />
              <span className="gg-picker__swatch-label">{m.label}</span>
              <span className="gg-picker__swatch-hint">{m.hint}</span>
            </button>
          ))}
        </div>
      </section>

      <section className="gg-picker__section">
        <h3 className="gg-picker__legend">Mode</h3>
        {/* A real radiogroup rather than buttons pretending to be one: arrow
            keys move between options and only the selected one is tabbable. */}
        <RadioGroup
          ariaLabel="Mode"
          orientation="horizontal"
          value={mode}
          onValueChange={(v) => setMode(v as ModePreference)}
          options={[
            { value: "light", label: "Light" },
            { value: "dark", label: "Dark" },
            { value: "system", label: "Match system" },
          ]}
        />
      </section>

      <section className="gg-picker__section">
        <h3 className="gg-picker__legend">Brand hue</h3>
        <input
          type="range"
          min={0}
          max={359}
          value={brandH}
          aria-label="Brand hue"
          className="gg-picker__hue"
          onChange={(e) => setBrand(Number(e.target.value))}
        />
        <div className="gg-picker__row">
          {BRAND_PRESETS.map((p) => (
            <button
              key={p.id}
              className={`gg-picker__chip ${brandH === p.h ? "is-selected" : ""}`}
              onClick={() => setBrand(p.h, p.c)}
            >
              <span
                className="gg-picker__dot"
                style={{ background: `oklch(0.60 ${p.c} ${p.h})` }}
                aria-hidden
              />
              {p.label}
            </button>
          ))}
        </div>
        {adjustmentNote ? (
          <p className="gg-picker__note">
            <Info size={13} aria-hidden /> {adjustmentNote}
          </p>
        ) : null}
      </section>

      <section className="gg-picker__section">
        <h3 className="gg-picker__legend">Density</h3>
        <RadioGroup
          ariaLabel="Density"
          orientation="horizontal"
          value={density}
          onValueChange={(v) => setDensity(v as Density)}
          options={[
            { value: "comfortable", label: "Comfortable", hint: "44px rows" },
            { value: "compact", label: "Compact", hint: "36px rows" },
          ]}
        />
      </section>

      <section className="gg-picker__section">
        <h3 className="gg-picker__legend">Live preview</h3>
        {/* Real components, including a table row, so the effect on dense
            content is visible before committing. */}
        <Card className="gg-picker__preview">
          <div className="gg-picker__preview-row">
            <strong>Osu</strong>
            <span className="gg-badge gg-badge--reference">· Reference</span>
          </div>
          <p className="gg-picker__preview-meta">Suburb · Accra Metropolitan, Greater Accra</p>
          <div className="gg-picker__preview-actions">
            <button className="gg-button gg-button--primary gg-button--sm">Publish</button>
            <button className="gg-button gg-button--secondary gg-button--sm">Review</button>
          </div>
          <input className="gg-input" placeholder="Search districts…" readOnly />
        </Card>
      </section>

      <button className="gg-button gg-button--ghost gg-button--sm" onClick={reset}>
        <RotateCcw size={14} aria-hidden /> Reset to defaults
      </button>
    </div>
  );
}
