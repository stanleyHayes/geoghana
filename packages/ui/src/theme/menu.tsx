"use client";

import * as Popover from "@radix-ui/react-popover";
import { Check, Monitor, Moon, Palette, Sun } from "lucide-react";
import { useTheme } from "./provider";
import { BRAND_PRESETS, MATERIALS, type ModePreference } from "./types";

/**
 * The compact theme control that lives in a navbar.
 *
 * ThemePicker is the full settings panel — materials with previews, a hue
 * slider, density, a live component preview. That belongs on a settings page.
 * Every other app needs the same three axes reachable in one click from the
 * header, or the tri-morphic system is invisible to anyone who never opens
 * admin: the marketing site is where visitors first meet it.
 *
 * Same axes, same store, same reveal transition — only the chrome differs.
 */

const MODES: ReadonlyArray<{
  id: ModePreference;
  label: string;
  Icon: typeof Sun;
}> = [
  { id: "light", label: "Light", Icon: Sun },
  { id: "dark", label: "Dark", Icon: Moon },
  { id: "system", label: "System", Icon: Monitor },
];

export function ThemeMenu({
  align = "end",
  /* Only apps that actually HAVE a settings page pass this. A hardcoded link
     here is how a dead link gets shipped to four apps at once. */
  settingsHref,
}: {
  align?: "start" | "center" | "end" | undefined;
  settingsHref?: string | undefined;
}) {
  const { material, mode, resolvedMode, brandH, setMaterial, setMode, setBrand } = useTheme();

  return (
    <Popover.Root>
      <Popover.Trigger asChild>
        <button
          className="gg-button gg-button--ghost gg-button--icon"
          aria-label={`Theme — ${resolvedMode} ${material}`}
          title="Theme"
        >
          {resolvedMode === "dark" ? <Moon size={18} /> : <Sun size={18} />}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="gg-thememenu" align={align} sideOffset={8} collisionPadding={12}>
          <section className="gg-thememenu__section">
            <p className="gg-thememenu__legend">Mode</p>
            <div className="gg-thememenu__modes" role="radiogroup" aria-label="Mode">
              {MODES.map(({ id, label, Icon }) => (
                <button
                  key={id}
                  role="radio"
                  aria-checked={mode === id}
                  className={`gg-thememenu__mode ${mode === id ? "is-selected" : ""}`}
                  onClick={(e) => setMode(id, { x: e.clientX, y: e.clientY })}
                >
                  <Icon size={15} aria-hidden />
                  {label}
                </button>
              ))}
            </div>
          </section>

          <section className="gg-thememenu__section">
            <p className="gg-thememenu__legend">Material</p>
            <div className="gg-thememenu__materials" role="radiogroup" aria-label="Material">
              {MATERIALS.map((m) => (
                <button
                  key={m.id}
                  role="radio"
                  aria-checked={material === m.id}
                  /* The swatch renders in its own material, so the choice is
                     shown rather than described. */
                  data-material={m.id}
                  className={`gg-thememenu__material ${material === m.id ? "is-selected" : ""}`}
                  onClick={(e) => setMaterial(m.id, { x: e.clientX, y: e.clientY })}
                  title={m.hint}
                >
                  <span className="gg-thememenu__chip" aria-hidden />
                  {m.label}
                </button>
              ))}
            </div>
          </section>

          <section className="gg-thememenu__section">
            <p className="gg-thememenu__legend">Brand</p>
            <div className="gg-thememenu__brands" role="radiogroup" aria-label="Brand colour">
              {BRAND_PRESETS.map((p) => (
                <button
                  key={p.id}
                  role="radio"
                  aria-checked={brandH === p.h}
                  aria-label={p.label}
                  title={p.label}
                  className={`gg-thememenu__brand ${brandH === p.h ? "is-selected" : ""}`}
                  style={{ background: `oklch(0.62 ${p.c} ${p.h})` }}
                  onClick={() => setBrand(p.h, p.c)}
                >
                  {brandH === p.h ? <Check size={13} strokeWidth={3} aria-hidden /> : null}
                </button>
              ))}
            </div>
          </section>

          {settingsHref ? (
            <a className="gg-thememenu__more" href={settingsHref}>
              <Palette size={14} aria-hidden /> All appearance settings
            </a>
          ) : null}
          <Popover.Arrow className="gg-thememenu__arrow" width={12} height={6} />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
