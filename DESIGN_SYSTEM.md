# GhanaGeo — Design System & UX Specification

**Version:** 1.0
**Last updated:** 2026-08-29
**Status:** **Mandatory** for all frontend work — lanes L7–L11 (EP-14 Design System, EP-15 Sandbox, EP-16 Developer Portal, EP-17 Admin Portal, EP-18 Marketing & Docs).
**Companion:** [`agent_plan.md`](agent_plan.md) — the delivery plan. This document is the *how it looks and moves*; that one is *what gets built when*.

> **Why this document exists.** GhanaGeo ships five frontends — marketing, docs, sandbox, developer portal and admin — that must read as one product. Every pattern below is either **distilled from three shipping codebases** (`RentOS`, `Xtiitch`, `AuraEDU`) where they independently converged on the same solution, or is a **deliberate correction** of a failure all three share. Where a pattern is a correction, it says so and names the failure. Provenance, not taste, is the argument.

---

## Table of Contents

1. [Principles](#1-principles)
2. [Stack](#2-stack)
3. [The Three-Axis Theming Architecture](#3-the-three-axis-theming-architecture)
4. [Design Tokens](#4-design-tokens)
5. [The Three Materials](#5-the-three-materials)
6. [The Accessibility Contract](#6-the-accessibility-contract)
7. [Theme Resolution, Persistence & FOUC](#7-theme-resolution-persistence--fouc)
8. [The Theme Picker](#8-the-theme-picker)
9. [Motion System](#9-motion-system)
10. [Text Transitions](#10-text-transitions)
11. [In-Page Transitions](#11-in-page-transitions)
12. [Layout & Route Transitions](#12-layout--route-transitions)
13. [Theme Toggle — The Circular Reveal](#13-theme-toggle--the-circular-reveal)
14. [The App Shell — Sidebar](#14-the-app-shell--sidebar)
15. [The App Shell — Navbar](#15-the-app-shell--navbar)
16. [Menus & Dropdowns](#16-menus--dropdowns)
17. [Command Palette & Global Search](#17-command-palette--global-search)
18. [Map Surfaces](#18-map-surfaces)
19. [Data-Dense Surfaces](#19-data-dense-surfaces)
20. [Marketing Surfaces](#20-marketing-surfaces)
21. [Component Inventory → Plan Mapping](#21-component-inventory--plan-mapping)
22. [Design-QA Checklist](#22-design-qa-checklist)
23. [Appendix — Concrete Values (copy these)](#appendix--concrete-values-copy-these)

---

## 1. Principles

1. **Tokens, never hex.** A component references semantic CSS custom properties (`--surface`, `--fg`, `--ring`, `--border`). No component hardcodes a colour, a shadow, a blur or a radius. This is what makes three visual styles possible without three component libraries.
2. **The material is a token swap, not a code branch.** There is exactly **one** `Card`, **one** `Button`. Neumorphic, glass and clay are produced by changing the value of ~14 CSS variables. A component containing `if (style === 'glass')` is a bug. *(§5.5 names the 14.)*
3. **One pattern, one component.** `AppShell`, `Sidebar`, `Navbar`, `CommandPalette`, `ThemePicker`, `PageHeader`, `EmptyState`, `DataTable`, `Skeleton` live in `packages/ui` and are never re-implemented per app.
4. **Legibility outranks material.** When a material's aesthetic fights readability, readability wins — always, without a discussion. This is not negotiable for a product whose admin screens hold national reference data. *(See §6, and the Intensity Tiers in §3.3.)*
5. **Motion is purposeful, cheap and always optional.** Default 150–250 ms. Transform and opacity only. Four independent layers of `prefers-reduced-motion` defence (§9.4).
6. **Skeletons over spinners.** Loading states mirror the real layout. No bare circular spinners in app surfaces, no "Loading…" text.
7. **Accessibility is a gate, not a goal.** WCAG 2.2 AA. Full keyboard operability, a focus ring visible **in every material**, contrast ≥ 4.5:1 for text and ≥ 3:1 for UI boundaries, correct ARIA, `aria-hidden` on decoration. A frontend PR failing §22 is not Done.
8. **Every state, not just the happy one.** Loading, empty, error, permission-denied and offline are designed before a screen is considered complete.

---

## 2. Stack

| Concern | Choice | Note |
|---|---|---|
| Framework | Next.js 16.3.3 App Router + React 19.2.8 | |
| Language | TypeScript 7.0.2 strict | Native-port compiler; typecheck via `tsgo` |
| Styling | Tailwind CSS 4.3.3, CSS-first `@theme` | **No `tailwind.config.js`.** Tokens live in `packages/ui/src/styles/tokens.css`. |
| Colour space | **OKLCH** | Perceptually uniform — essential, because the custom-theme axis derives shades from a user-chosen hue and must not produce muddy or blown-out results. |
| Primitives | Radix UI (`@radix-ui/react-*` 1.1.x) | Unstyled + accessible. Mandatory: the material swap only works if behaviour and appearance are already separated. |
| Icons | `lucide-react` 1.37.0 | |
| Class utils | `class-variance-authority` 0.7.1 + `tailwind-merge` 3.6.0 | `cn()` in `packages/ui/src/lib/utils.ts` |
| Motion — app | **Pure CSS + the View Transitions API. Zero animation-library JS in `apps/admin`, `apps/portal`, `apps/sandbox`.** | This is the house rule, independently arrived at by all three reference codebases: RentOS ships zero animation dependencies, Xtiitch admin ships zero, and AuraEDU's spec states outright that Framer Motion is allowed on marketing *only*. Admin bundles stay small. |
| Motion — marketing | `motion` 13.1.1 + GSAP 3.15.0 with ScrollTrigger, **`apps/web` only** | Never imported by an authenticated app. Enforced by an ESLint `no-restricted-imports` rule per workspace. |
| Maps | Leaflet 1.9.4 + react-leaflet 5.0.0 + `leaflet.markercluster` 1.5.3 | No API key required; OSM raster tiles with visible ODbL attribution. |
| Charts | Recharts 3.10.1 | Themed from CSS variables, never props-hardcoded. |
| Fonts | Display **Bricolage Grotesque** · UI/body **Outfit** · mono **JetBrains Mono**, via `next/font` | Outfit is the house UI face across all three reference products. |
| Theme state | Hand-rolled `ThemeProvider` + blocking inline script | **Explicitly not `next-themes`** — it models one axis (mode). We have three (§3). |

---

## 3. The Three-Axis Theming Architecture

The user's theme is a point in a **three-dimensional space**. This is the central idea of the whole system.

```
             ┌─────────────────────────────────────────────────┐
   AXIS 1    │  MATERIAL   neu  │  glass  │  clay               │  data-material
   (style)   │  How surfaces are made of "stuff"                │  on <html>
             ├─────────────────────────────────────────────────┤
   AXIS 2    │  MODE       light │ dark    │ (system)           │  data-mode
   (light)   │  Where the light comes from                      │  on <html>
             ├─────────────────────────────────────────────────┤
   AXIS 3    │  BRAND      hue 0–360 + chroma                   │  --brand-h / --brand-c
   (custom)  │  The user's or tenant's accent                   │  inline style
             └─────────────────────────────────────────────────┘
                     3  ×  2  ×  ∞   =  every component must survive all of it
```

### 3.1 Why three axes and not one theme list

A flat list ("Neumorphic Light", "Glass Dark", …) multiplies: 3 × 2 × N brand colours is unbounded and unmaintainable. Treating them as **independent axes** means a component reads semantic tokens and never knows which point in the space it is at. Adding a fourth material later costs one token block, not a component rewrite.

### 3.2 Axis independence rule

**Materials must not own colours, and colours must not own materials.**

- The **material axis** may only change: shadows, borders, radii, blur, surface alpha, and elevation. It may **never** change hue, foreground colour, or spacing.
- The **mode axis** may only change: lightness values and shadow colour derivation.
- The **brand axis** may only change: `--brand-h`, `--brand-c`, and everything derived from them.

Violating this is what makes theme systems collapse into unmaintainable special cases. A CI lint asserts that no `[data-material]` block sets a `--fg-*` or `--space-*` token.

### 3.3 Intensity tiers — the honest constraint

Not every surface can carry full material expression, and pretending otherwise would ship an inaccessible admin. Each app declares an **intensity tier** that scales the material down:

| Tier | Where | Material expression |
|---|---|---|
| `expressive` | `apps/web` (marketing) — hero, feature bands, pricing | Full. Large radii, deep shadows, strong blur, decorative blobs. |
| `balanced` | `apps/portal`, `apps/sandbox`, admin dashboards and detail pages | Material visible on cards, panels, buttons and chrome. Reduced shadow spread and blur. |
| `restrained` | **Admin data surfaces** — tables, diff views, geometry editor, audit log, JSON viewers, log tables | Material reduced to a 1px border, a flat surface tint and a 2px focus ring. **Shadows and blur off.** |

Implemented as `data-intensity="expressive|balanced|restrained"` on a container, which scales the material tokens:

```css
[data-intensity="balanced"]  { --mat-shadow-scale: 0.6; --mat-blur-scale: 0.7; }
[data-intensity="restrained"]{ --mat-shadow-scale: 0;   --mat-blur-scale: 0;
                               --mat-border-width: 1px; --mat-border-color: var(--border-strong); }
```

> **Why this exists.** The UI style research is unambiguous: neumorphism is rated *"Do not use for: data-heavy dashboards, high-contrast required"* with *"⚠ Low contrast"* accessibility and only *partial* dark-mode viability; claymorphism is rated *"Do not use for: professional services, data-critical"*. GhanaGeo's admin is precisely a data-critical, data-heavy dashboard. Rather than refuse the brief or ship something unusable, the system **keeps all three materials available everywhere the user chooses them, and reduces their intensity exactly where density demands it**. The user still picks their material; the material simply expresses itself through border and tint instead of shadow when it's sitting under a 500-row table.

---

## 4. Design Tokens

Three layers — primitive → semantic → component — plus a **material layer** that sits between primitive and semantic. That fourth layer is the mechanism of the whole system.

```
┌──────────────────────────────────────────────┐
│ COMPONENT   --btn-bg, --card-pad             │  per-component
├──────────────────────────────────────────────┤
│ SEMANTIC    --surface, --fg, --border, --ring│  purpose
├──────────────────────────────────────────────┤
│ MATERIAL    --mat-shadow-raised, --mat-blur  │  ◄── the style axis lives HERE
├──────────────────────────────────────────────┤
│ PRIMITIVE   --ink-950, --space-4, --dur-base │  raw
└──────────────────────────────────────────────┘
```

### 4.1 Primitive tokens

`packages/ui/src/styles/tokens.css`, inside `@theme`.

```css
@theme {
  /* Type */
  --font-display: var(--font-bricolage), ui-sans-serif, system-ui, sans-serif;
  --font-sans:    var(--font-outfit), ui-sans-serif, system-ui, sans-serif;
  --font-mono:    var(--font-jetbrains), ui-monospace, SFMono-Regular, Menlo, monospace;

  --text-2xs: 0.6875rem;  /* 11px — nav eyebrows, table meta */
  --text-xs:  0.75rem;
  --text-sm:  0.8125rem;  /* 13px — the admin default */
  --text-base:0.9375rem;  /* 15px — portal/marketing body */
  --text-lg:  1.0625rem;
  --text-xl:  1.25rem;
  --text-2xl: 1.5rem;
  --text-3xl: 2rem;
  --text-4xl: 2.75rem;
  --text-5xl: 3.75rem;    /* marketing hero only */

  /* Space — 4px base, 8pt rhythm */
  --space-1: 0.25rem;  --space-2: 0.5rem;   --space-3: 0.75rem;
  --space-4: 1rem;     --space-5: 1.25rem;  --space-6: 1.5rem;
  --space-8: 2rem;     --space-10: 2.5rem;  --space-12: 3rem;  --space-16: 4rem;

  /* Neutral ramp — cool, mineral, map-friendly */
  --ink-50:  oklch(0.985 0.003 250);  --ink-100: oklch(0.965 0.005 250);
  --ink-200: oklch(0.925 0.008 250);  --ink-300: oklch(0.865 0.012 250);
  --ink-400: oklch(0.715 0.016 250);  --ink-500: oklch(0.585 0.018 250);
  --ink-600: oklch(0.475 0.020 250);  --ink-700: oklch(0.385 0.022 252);
  --ink-800: oklch(0.285 0.024 254);  --ink-900: oklch(0.205 0.026 256);
  --ink-950: oklch(0.145 0.024 258);

  /* Brand default — GhanaGeo "meridian" green-teal.
     Ghana's flag greens without the literal flag palette. */
  --brand-h: 168;  --brand-c: 0.115;
  --brand:   oklch(0.545 var(--brand-c) var(--brand-h));

  /* Status — deliberately OUTSIDE the brand axis, so a user's custom hue
     can never make "published" and "rolled back" look alike. */
  --success: oklch(0.560 0.130 152);
  --warning: oklch(0.700 0.150  75);
  --danger:  oklch(0.560 0.195  27);
  --info:    oklch(0.580 0.130 248);

  /* Verification-status colours — GhanaGeo-specific, used by admin + docs */
  --vs-canonical:  var(--success);
  --vs-reviewed:   var(--info);
  --vs-reference:  var(--ink-500);
  --vs-needs-recon:var(--warning);
  --vs-deprecated: var(--ink-400);

  /* Motion */
  --ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1);
  --ease-in-out:    cubic-bezier(0.4, 0, 0.2, 1);
  --ease-spring:    cubic-bezier(0.34, 1.56, 0.64, 1);
  --dur-instant: 90ms;  --dur-fast: 150ms;  --dur-base: 220ms;
  --dur-slow: 360ms;    --dur-slower: 520ms;

  /* Shell geometry */
  --shell-sidebar-w: 288px;
  --shell-sidebar-collapsed-w: 80px;
  --shell-navbar-h: 64px;
  --shell-content-max: 1480px;
}
```

### 4.2 Semantic tokens

These are the only names a component may use.

| Token | Meaning |
|---|---|
| `--bg` | Page ground |
| `--bg-subtle` | Recessed regions (table headers, code blocks) |
| `--surface` | Card / panel fill |
| `--surface-raised` | Popover, dropdown, dialog, tooltip |
| `--surface-sunken` | Inputs, wells, pressed states |
| `--fg` | Primary text |
| `--fg-muted` | Secondary text |
| `--fg-subtle` | Tertiary text, placeholders |
| `--border` | Default boundary |
| `--border-strong` | Emphasised boundary; the fallback boundary in `restrained` |
| `--primary` / `--primary-fg` | Brand action |
| `--accent` / `--accent-fg` | Brand tint surface (active nav, selection) |
| `--ring` | Focus ring colour |
| `--overlay` | Modal scrim |

### 4.3 Brand derivation (axis 3)

Everything derives from two numbers, so a user picking a hue re-skins the product without producing an unreadable result.

```css
:root {
  --brand:        oklch(0.545 var(--brand-c) var(--brand-h));
  --brand-hover:  oklch(0.485 var(--brand-c) var(--brand-h));
  --brand-active: oklch(0.425 var(--brand-c) var(--brand-h));
  --brand-tint:   oklch(0.960 calc(var(--brand-c) * 0.22) var(--brand-h));
  --brand-fg:     oklch(0.985 0 0);
}
[data-mode="dark"] {
  /* Lift lightness and drop chroma: a mid-lightness hue that reads correctly
     on paper goes muddy on ink. Xtiitch does the same thing by hand,
     lightening its wine #800020 → #b82a4b for dark mode. */
  --brand:       oklch(0.680 calc(var(--brand-c) * 0.88) var(--brand-h));
  --brand-hover: oklch(0.735 calc(var(--brand-c) * 0.88) var(--brand-h));
  --brand-tint:  oklch(0.280 calc(var(--brand-c) * 0.55) var(--brand-h));
  --brand-fg:    oklch(0.145 0 0);
}
```

Fixing **L** per mode and varying only **H** and **C** is what guarantees that any user-chosen hue lands at a usable lightness. See §6.3 for the contrast clamp that handles the remaining edge cases.

---

## 5. The Three Materials

Each material is one CSS block. Nothing else in the system changes.

### 5.1 Neumorphism — *extruded from the page*

The surface is **the same colour as its parent**, and shape comes entirely from a paired light/dark shadow. This is why it needs a mid-tone ground: pure white cannot cast a lighter highlight.

```css
[data-material="neu"] {
  --mat-surface-alpha: 1;
  --mat-radius:        1rem;      /* 16px */
  --mat-radius-sm:     0.75rem;
  --mat-blur:          0px;
  --mat-border-width:  0px;
  --mat-border-color:  transparent;

  --bg:            oklch(0.935 0.006 250);   /* mid-tone ground, NOT white */
  --surface:       var(--bg);                /* identical to ground — the rule */
  --surface-raised:oklch(0.955 0.006 250);
  --surface-sunken:oklch(0.905 0.008 250);

  --mat-light: oklch(1 0 0 / 0.90);
  --mat-dark:  oklch(0.72 0.020 250 / 0.42);

  --mat-shadow-flat:   none;
  --mat-shadow-raised:
      calc(6px  * var(--mat-shadow-scale)) calc(6px  * var(--mat-shadow-scale))
      calc(14px * var(--mat-shadow-scale)) var(--mat-dark),
      calc(-6px * var(--mat-shadow-scale)) calc(-6px * var(--mat-shadow-scale))
      calc(14px * var(--mat-shadow-scale)) var(--mat-light);
  --mat-shadow-lifted:
      calc(10px * var(--mat-shadow-scale)) calc(10px * var(--mat-shadow-scale))
      calc(24px * var(--mat-shadow-scale)) var(--mat-dark),
      calc(-10px* var(--mat-shadow-scale)) calc(-10px* var(--mat-shadow-scale))
      calc(24px * var(--mat-shadow-scale)) var(--mat-light);
  --mat-shadow-inset:
      inset 3px 3px 7px var(--mat-dark),
      inset -3px -3px 7px var(--mat-light);
}
[data-material="neu"][data-mode="dark"] {
  --bg:             oklch(0.255 0.018 256);
  --surface:        var(--bg);
  --surface-raised: oklch(0.295 0.018 256);
  --surface-sunken: oklch(0.215 0.020 258);
  --mat-light: oklch(0.40 0.020 256 / 0.55);   /* NOT white — a lifted neutral */
  --mat-dark:  oklch(0.10 0.020 260 / 0.72);
}
```

**Rules.** Inputs and pressed states use `--mat-shadow-inset` (concave); resting cards and buttons use `--mat-shadow-raised` (convex) — the convex/concave distinction is the entire language, and inverting it makes the UI unreadable. Nesting goes raised → inset → raised, never raised → raised. RentOS ships exactly this vocabulary (`--rentos-shadow-soft` / `-lift` / `-inset`).

**Known weakness, addressed:** borderless low-contrast edges. In `restrained` tier, `--mat-shadow-scale: 0` collapses the shadows and `--mat-border-width: 1px` restores a real boundary. Focus rings are never shadow-based (§6.2).

### 5.2 Glassmorphism — *frosted panes over depth*

```css
[data-material="glass"] {
  --mat-surface-alpha: 0.62;
  --mat-radius:        1.125rem;
  --mat-radius-sm:     0.75rem;
  --mat-blur:          calc(16px * var(--mat-blur-scale));
  --mat-border-width:  1px;
  --mat-border-color:  oklch(1 0 0 / 0.28);

  --bg:      oklch(0.955 0.010 250);
  --surface: oklch(1 0 0 / var(--mat-surface-alpha));
  --surface-raised: oklch(1 0 0 / 0.80);
  --surface-sunken: oklch(0.94 0.010 250 / 0.70);

  --mat-backdrop: blur(var(--mat-blur)) saturate(150%);
  --mat-shadow-raised:
      0 calc(4px * var(--mat-shadow-scale)) calc(14px * var(--mat-shadow-scale))
        oklch(0.30 0.02 255 / 0.10),
      inset 0 1px 0 0 oklch(1 0 0 / 0.45);      /* the top-edge shine */
  --mat-shadow-lifted:
      0 calc(12px * var(--mat-shadow-scale)) calc(34px * var(--mat-shadow-scale))
        oklch(0.30 0.02 255 / 0.16),
      inset 0 1px 0 0 oklch(1 0 0 / 0.55);
  --mat-shadow-inset: inset 0 1px 3px oklch(0.30 0.02 255 / 0.14);
}
[data-material="glass"][data-mode="dark"] {
  --bg:      oklch(0.185 0.022 258);
  --surface: oklch(0.42 0.020 256 / 0.34);
  --surface-raised: oklch(0.46 0.020 256 / 0.55);
  --mat-border-color: oklch(1 0 0 / 0.12);
}
```

**Glass requires something behind it.** A glass panel on a flat ground looks like a slightly grey card. `apps/web` and auth pages therefore render a fixed, `aria-hidden` gradient-mesh layer behind the shell. In admin, the map *is* the layer — which is exactly why §18 exists.

**Performance rules (non-negotiable).** `backdrop-filter` rasterises the region behind the element on every frame.
- Apply blur to **fixed-size chrome only** — navbar, sidebar, dialogs, popovers, map panels.
- **Never** on a scrolling container, a virtualised list, a table row, or any element that appears more than ~12 times on screen.
- Never animate `backdrop-filter`; animate `opacity` on a pre-blurred layer instead.
- Every blurred surface declares an **opaque fallback**:
  ```css
  @supports not (backdrop-filter: blur(1px)) {
    [data-material="glass"] { --mat-surface-alpha: 0.96; }
  }
  ```

### 5.3 Claymorphism — *inflated, soft, floating*

Clay differs from neumorphism in one structural way: **clay floats on a contrasting ground; neumorphism extrudes from an identical one.** That single fact drives every value below.

```css
[data-material="clay"] {
  --mat-surface-alpha: 1;
  --mat-radius:        1.75rem;    /* 28px — the defining trait */
  --mat-radius-sm:     1.125rem;
  --mat-blur:          0px;
  --mat-border-width:  0px;

  --bg:      oklch(0.960 0.014 var(--brand-h));   /* tinted, never white */
  --surface: oklch(0.995 0.006 var(--brand-h));   /* CONTRASTS with --bg */
  --surface-raised: oklch(1 0 0);
  --surface-sunken: oklch(0.945 0.016 var(--brand-h));

  --mat-shadow-raised:
      calc(8px  * var(--mat-shadow-scale)) calc(12px * var(--mat-shadow-scale))
      calc(24px * var(--mat-shadow-scale)) oklch(0.55 0.06 var(--brand-h) / 0.20),
      inset -4px -6px 12px oklch(0.62 0.07 var(--brand-h) / 0.16),
      inset  4px  5px 12px oklch(1 0 0 / 0.92);
  --mat-shadow-lifted:
      calc(12px * var(--mat-shadow-scale)) calc(20px * var(--mat-shadow-scale))
      calc(38px * var(--mat-shadow-scale)) oklch(0.55 0.06 var(--brand-h) / 0.26),
      inset -4px -6px 12px oklch(0.62 0.07 var(--brand-h) / 0.16),
      inset  4px  5px 12px oklch(1 0 0 / 0.92);
  --mat-shadow-inset:
      inset 5px 6px 12px oklch(0.55 0.06 var(--brand-h) / 0.22),
      inset -3px -4px 10px oklch(1 0 0 / 0.75);
}
[data-material="clay"][data-mode="dark"] {
  --bg:      oklch(0.225 0.030 var(--brand-h));
  --surface: oklch(0.310 0.032 var(--brand-h));
  --surface-raised: oklch(0.355 0.032 var(--brand-h));
  --mat-shadow-raised:
      calc(8px * var(--mat-shadow-scale)) calc(12px * var(--mat-shadow-scale))
      calc(24px * var(--mat-shadow-scale)) oklch(0.08 0.02 260 / 0.55),
      inset -4px -6px 12px oklch(0.16 0.02 260 / 0.55),
      inset  4px  5px 12px oklch(0.50 0.04 var(--brand-h) / 0.35);
}
```

**Clay's press is a squish**, not a lift: `transform: scale(0.97)` over `--dur-fast` with `--ease-spring`. It is the only material that overshoots, and the overshoot is **disabled in `restrained`** — bounce on a data table reads as sloppy.

### 5.4 The nesting rule (all materials)

Nested radii must decrease or the inner element looks wrong: `inner-radius = outer-radius − padding`. Card at `--mat-radius` with `--space-4` padding gives an inner control `calc(var(--mat-radius) - var(--space-4))`, floored at `--mat-radius-sm`.

### 5.5 The complete material contract — exactly 14 tokens

**These, and only these, differ between materials.** Any component reading only from these plus the semantic layer works in all three, forever.

```
--mat-surface-alpha   --mat-radius          --mat-radius-sm      --mat-blur
--mat-backdrop        --mat-border-width    --mat-border-color   --mat-shadow-flat
--mat-shadow-raised   --mat-shadow-lifted   --mat-shadow-inset   --mat-press-transform
--mat-shadow-scale    --mat-blur-scale
```

A `Card` in every material, in full:

```css
.card {
  background: var(--surface);
  backdrop-filter: var(--mat-backdrop, none);
  border: var(--mat-border-width) solid var(--mat-border-color);
  border-radius: var(--mat-radius);
  box-shadow: var(--mat-shadow-raised);
  padding: var(--space-5);
  color: var(--fg);
  transition: box-shadow var(--dur-base) var(--ease-out-quart),
              transform  var(--dur-fast) var(--ease-out-quart);
}
.card[data-interactive]:hover { box-shadow: var(--mat-shadow-lifted); transform: translateY(-2px); }
.card:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
```

That is the whole proof of the architecture: no material name appears anywhere in it.

---

## 6. The Accessibility Contract

The three materials fail accessibility in three different ways. Each failure has a named, tested remedy. This section is the reason the system is shippable.

| Material | The failure | The remedy |
|---|---|---|
| **Neumorphism** | Boundaries are shadows, not edges. Under Windows High Contrast the shadows vanish and every card merges into the page. Low-vision users lose all structure. | `--mat-border-width` becomes 1px in `restrained` and under `forced-colors`; focus is an `outline`, never a shadow (§6.2). |
| **Glassmorphism** | Text contrast depends on whatever happens to be behind the panel — it is *not statically determinable*. Over a map it changes as the user pans. | Minimum `--mat-surface-alpha` floors per tier; over map, an opaque scrim layer (§18.2). |
| **Claymorphism** | Pastel-on-pastel fills drift under 4.5:1; the large radii swallow dense content. | Fills are derived at fixed L (§4.3) rather than authored; radii shrink in `restrained`. |

### 6.1 Contrast floors (enforced, not aspirational)

| Pair | Minimum |
|---|---|
| Body text on `--surface` | **4.5:1** |
| Large text (≥ 18.66px bold / 24px) | **3:1** |
| `--border-strong` vs adjacent surface | **3:1** |
| Focus ring vs both the surface *and* the page | **3:1** |
| Status colours vs their own surface | **4.5:1** |
| Text over any glass panel over a map | **4.5:1 against the worst-case tile** — measured, not assumed |

### 6.2 The focus ring — the single most important accessibility rule here

**Focus is never expressed as a shadow.** A neumorphic focus glow disappears into the extrusion; a clay one disappears into the inset; a glass one disappears into the blur. Focus is always a real `outline`, in every material, always two-toned so it survives on both light and dark surfaces:

```css
:where(a, button, input, select, textarea, [tabindex]):focus-visible {
  outline: 2px solid var(--ring);
  outline-offset: 2px;
  /* second ring in the page ground guarantees visibility on any surface */
  box-shadow: 0 0 0 4px var(--bg);
}
```
`box-shadow` here is a *ring*, not depth — it is the one permitted exception, and it uses `--bg` so it reads as a halo on every material.

### 6.3 The custom-hue contrast clamp

A user picking a pale yellow brand must not be able to make primary buttons unreadable. On every hue change the provider runs a clamp before committing:

1. Compute the APCA/WCAG contrast of `--brand-fg` against `--brand`.
2. If below 4.5:1, step **L** of `--brand` away from `--brand-fg` in 0.02 increments (down in light mode, up in dark) until it passes, to a maximum of 12 steps.
3. If it still fails, flip `--brand-fg` between near-white and near-black and retry.
4. Persist the **clamped** value, and surface a quiet inline note in the picker: *"Adjusted for readability."*

The user's hue is always honoured; only its lightness is corrected. This runs in `packages/ui/src/theme/clamp.ts` and is unit-tested across the full 0–360 hue circle at both modes — a test that asserts **no hue in either mode can produce a failing pair**.

### 6.4 Forced colours / Windows High Contrast

```css
@media (forced-colors: active) {
  :root { --mat-shadow-raised: none; --mat-shadow-lifted: none;
          --mat-shadow-inset: none; --mat-blur: 0px; --mat-border-width: 1px; }
  .card, .panel, .popover { border: 1px solid CanvasText; background: Canvas; }
  :focus-visible { outline: 2px solid Highlight; }
}
```
All three materials degrade to the same honest bordered-box system. This is correct: in forced-colours mode the user has explicitly asked for the OS palette, and decoration is noise.

### 6.5 Non-negotiables

- Every interactive target ≥ 44 × 44 px (24 × 24 px minimum for inline table controls, per WCAG 2.2 *Target Size (Minimum)*).
- Colour is never the sole carrier of meaning — verification status shows an **icon + label**, not just `--vs-*`.
- Decorative layers (watermarks, gradient mesh, blobs) are `aria-hidden="true"` and never receive pointer events.
- Every icon-only control has an `aria-label` **and** a `title`.
- `--fg-subtle` is never used for content, only for placeholders and disabled affordances.

---

## 7. Theme Resolution, Persistence & FOUC

### 7.1 Resolution order

```
1. Explicit user choice, server-side profile   (signed-in, cross-device)
2. Explicit user choice, localStorage          (anonymous or pre-hydration)
3. System preference                            (prefers-color-scheme → mode only)
4. Product default                              material=glass, mode=system, brand=meridian
```

Default material is **glass** because it is the only one of the three rated fully viable in both light and dark and suited to dashboards; the other two are opt-in.

### 7.2 Storage keys

| Key | Holds |
|---|---|
| `ghanageo-theme` | `{ material, mode, brandH, brandC, version }` — one JSON blob, one write |
| `ghanageo-sidebar` | `{ collapsed, openGroups[] }` |
| `ghanageo-density` | `comfortable \| compact` |

Cross-tab and same-tab sync via a `storage` listener plus a `ghanageo-theme-change` `CustomEvent`. Reads are wrapped in `try/catch` — **corrupted or blocked storage must never white-screen the app** (a lesson RentOS learned and commented in its own store).

### 7.3 FOUC prevention — mandatory

> **This is a correction, not a copy.** RentOS has **no** blocking script: its theme class is applied only once the JS module graph loads, so a dark-mode user sees a light flash on every cold load. GhanaGeo must not ship that.

A blocking inline script in the root layout `<head>`, before any painted content, with `<html suppressHydrationWarning>`:

```html
<script dangerouslySetInnerHTML={{ __html: `(function(){try{
  var d=document.documentElement, s=localStorage.getItem('ghanageo-theme'), t=s?JSON.parse(s):{};
  var mode=t.mode&&t.mode!=='system'?t.mode
    :(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');
  d.dataset.mode=mode;
  d.dataset.material=t.material||'glass';
  d.style.colorScheme=mode;
  if(t.brandH!=null){d.style.setProperty('--brand-h',t.brandH);
                     d.style.setProperty('--brand-c',t.brandC);}
}catch(e){}})();` }} />
```

Three requirements: it is **synchronous and inline** (a deferred or external script is too late); it is wrapped in `try/catch`; and it sets `color-scheme` so form controls, scrollbars and the browser's own canvas paint correctly on the first frame.

---

## 8. The Theme Picker

**One component, `<ThemePicker />`, mounted in three places** — this is an explicit product requirement:

| Location | Surface | Form |
|---|---|---|
| `apps/admin` → Settings → Appearance | Full picker | Three-column layout with live preview |
| `apps/portal` → Settings → Appearance | Full picker | Same component |
| `apps/web` (marketing) → footer **and** navbar | Full picker in footer; compact material+mode toggle in navbar | So a visitor sets their preference **before** signing up, and it carries into the portal on account creation |

### 8.1 Anatomy

```
┌──────────────────────────────────────────────────────────────┐
│  Appearance                                                  │
│                                                              │
│  MATERIAL                                                    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                     │
│  │  ▢ soft  │ │  ▤ frost │ │  ▣ clay  │   ← live mini-cards, │
│  │ Neumorph │ │  Glass   │ │  Clay    │     each rendered IN │
│  └──────────┘ └──────────┘ └──────────┘     its own material │
│                                                              │
│  MODE      ( ) Light   ( ) Dark   (•) Match system           │
│                                                              │
│  BRAND HUE                                                   │
│  ├──────────────●───────────────────────────┤  168°          │
│  [meridian] [lagoon] [clay] [kente] [ink] [custom…]          │
│  ⓘ Adjusted for readability.        ← only when clamped      │
│                                                              │
│  DENSITY   (•) Comfortable   ( ) Compact                     │
│                                                              │
│  ┌── Live preview ───────────────────────────────────────┐   │
│  │  a real Card + Button + Input + Table row + Badge     │   │
│  └───────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

### 8.2 Behaviour

- Each material swatch renders **inside its own material** — you see the thing itself, not a label describing it.
- The preview pane is real components, not a picture, and includes a **table row** so the user sees what their choice does to dense content.
- Changing material or mode routes through the circular reveal (§13).
- Changing hue applies live via `--brand-h` with **no** view transition (it would be constant during a slider drag).
- Signed-in: debounced 400 ms `PATCH /me/preferences`. Anonymous: `localStorage` only.
- Full keyboard operation: arrow keys move within a radiogroup, slider responds to arrows/Home/End, every swatch has an accessible name.
- A **Reset to defaults** action, always present.

---

## 9. Motion System

### 9.1 Duration & easing scale

| Token | Value | Use |
|---|---|---|
| `--dur-instant` | 90 ms | Colour-only hover, checkbox tick |
| `--dur-fast` | 150 ms | Hover lift, press, tooltip |
| `--dur-base` | 220 ms | Dropdown, popover, accordion, sidebar collapse |
| `--dur-slow` | 360 ms | Page enter, dialog, sheet |
| `--dur-slower` | 520 ms | Shell entrance, theme reveal |

| Easing | Value | Use |
|---|---|---|
| `--ease-out-quart` | `cubic-bezier(0.25, 1, 0.5, 1)` | **The default.** Everything entering. |
| `--ease-in-out` | `cubic-bezier(0.4, 0, 0.2, 1)` | Things that move and settle: sidebar width, theme reveal |
| `--ease-spring` | `cubic-bezier(0.34, 1.56, 0.64, 1)` | Clay press only. **Never** in `restrained`. |

Exit is always faster than entry — cap exits at 150 ms so the UI never feels like it is holding the user up.

### 9.2 What may be animated

**Only `transform` and `opacity`** — compositor-safe, no layout, no paint.

Explicitly forbidden: animating `width`, `height`, `top`, `left`, `margin`, `padding`, `box-shadow` geometry, `backdrop-filter`, or `filter` on anything that repeats. Two sanctioned exceptions, both cheap and both used:
- Sidebar `width` + content `margin-left`, because it is one element pair, once, at 220 ms.
- Accordion `grid-template-rows: 0fr → 1fr`, which is the only correct way to animate to intrinsic height.

`will-change` is applied on interaction start and **removed on completion** — leaving it on permanently costs GPU memory and makes things slower.

### 9.3 Keyframe catalogue

Shipped in `tokens.css`; nothing else may declare a keyframe.

```css
@keyframes fade-in    { from { opacity: 0 } to { opacity: 1 } }
@keyframes slide-up   { from { opacity: 0; transform: translateY(0.5rem) }  to { opacity: 1; transform: none } }
@keyframes slide-down { from { opacity: 0; transform: translateY(-0.5rem) } to { opacity: 1; transform: none } }
@keyframes scale-in   { from { opacity: 0; transform: scale(0.96) }         to { opacity: 1; transform: none } }
@keyframes page-enter { from { opacity: 0; transform: translateY(0.75rem) } to { opacity: 1; transform: none } }
@keyframes shimmer    { from { background-position: -200% 0 } to { background-position: 200% 0 } }
@keyframes pulse-dot  { 0%,100% { opacity: 1 } 50% { opacity: 0.35 } }
@keyframes blur-in    { from { opacity: 0; filter: blur(6px); transform: translateY(0.4rem) }
                        to   { opacity: 1; filter: blur(0);   transform: none } }
```

### 9.4 Reduced motion — four independent layers of defence

Any one layer failing must not let motion through.

**Layer 1 — global kill switch**, declared near the top of `tokens.css` so it wins by cascade:
```css
@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after {
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
    scroll-behavior: auto !important;
  }
}
```
**Layer 2 — `motion-safe:`** on every discretionary animation (page-enter, scroll reveals, blob drift).
**Layer 3 — component `@media` blocks** setting `animation: none` on decorative loops.
**Layer 4 — JS guards:** `matchMedia('(prefers-reduced-motion: reduce)').matches` checked before `startViewTransition`, before smooth scrolling, and before any GSAP timeline is created.

**What survives reduced motion:** opacity crossfades under 100 ms, colour changes, and *all* state feedback. Reduced motion means less movement, not less feedback — a user who cannot see a spinner spin still needs to know something is loading.

---

## 10. Text Transitions

Type is the product's voice; these are how it arrives. **Marketing only for split-text effects** — admin text never animates per character.

### 10.1 The accessibility rule that governs all of them

Splitting text into per-character spans destroys it for screen readers and breaks text selection and search. Every split element therefore:

```html
<h1 class="split" aria-label="Ghana's location data, as infrastructure">
  <span aria-hidden="true"><span class="ch">G</span><span class="ch">h</span>…</span>
</h1>
```
The accessible name lives on the parent; the spans are `aria-hidden`. Under `prefers-reduced-motion`, **do not split at all** — render the plain text node. GSAP `SplitText` instances are `revert()`ed on unmount so assistive tech and copy/paste see real text again.

### 10.2 The catalogue

| Name | Where | Recipe |
|---|---|---|
| **Blur-in headline** | Marketing hero (`apps/web`) | Per-word `blur-in`, `--dur-slow`, `--ease-out-quart`, stagger **60 ms**. Max ~8 words. |
| **Char cascade** | Marketing section headings | Per-char `y: 20px → 0`, `rotateX(-40deg) → 0`, 600 ms `expo.out`, stagger **15 ms**. |
| **Line mask** | Marketing pull-quotes | Lines in `overflow:hidden` wrappers, `translateY(100%) → 0`, 500 ms, stagger 80 ms. Best-behaved: no per-char DOM. |
| **Counting number** | Coverage stats ("**261** districts") | `requestAnimationFrame` count-up over 900 ms `ease-out-quart`, `font-variant-numeric: tabular-nums` so width never jitters. Reduced motion → final value immediately. |
| **Decode / scramble** | Sandbox: text→coordinates resolution | Cycle glyphs 40 ms per frame, resolving left→right over ≤ 700 ms. **Sandbox only** — one use, as a delight moment when a query resolves. |
| **Gradient sweep** | Marketing hero keyword | `background-clip: text` + animated `background-position`, 3 s linear infinite. **Never** on body copy. |
| **Type-in** | Sandbox sample-query demo | 45 ms per char with a blinking caret. Skips to full text on any user input. |

### 10.3 Text transitions in the app

Admin and portal get **exactly three**, all cheap:
1. **Value crossfade** — a number changing (queue depth, quota) fades old → new over `--dur-fast`, with `tabular-nums`. No sliding digits.
2. **Truncation reveal** — a truncated cell expands on hover/focus over `--dur-fast`.
3. **Status change** — a badge changing state animates only its colour, over `--dur-base`.

Nothing else. Split text in a data table is noise.

---

## 11. In-Page Transitions

| Pattern | Spec |
|---|---|
| **Scroll reveal** (marketing) | `IntersectionObserver` at `rootMargin: '0px 0px -12% 0px'`, `slide-up` `--dur-slow`. **Fires once**, then unobserves. Content is visible by default in CSS and only hidden once JS confirms observer support — never `opacity: 0` with no fallback, which hides content from crawlers and no-JS users. |
| **Staggered list** | 30 ms per item, **capped at 8 items**; item 9+ appears with the 8th. Long staggers feel broken. |
| **Accordion** | `grid-template-rows: 0fr → 1fr` + `opacity`, `--dur-base`, inner wrapper `min-height: 0; overflow: hidden`. |
| **Dialog** | Overlay `fade-in` 160 ms; content `scale-in` 200 ms `--ease-out-quart`. Exit 120 ms. |
| **Sheet / drawer** | `translateX(-100%) → 0` (left) over `--dur-base`; scrim `fade-in` 160 ms. |
| **Popover / dropdown** | `slide-up` 180 ms, `transform-origin` set from Radix's `data-side`. |
| **Tooltip** | `fade-in` 120 ms, 400 ms open delay, 0 ms close. |
| **Toast** | Enter `slide-up`; stack newest-at-bottom, max 3 visible; older collapse to a "+N" pill. |
| **Skeleton** | `shimmer` 1.4 s `sine.inOut` infinite, one shared timeline per group so the wave reads as a single sweep. Skeletons **mirror the real layout**, never a grey block. |
| **Tab underline** | Shared `layoutId`-style morph — an absolutely-positioned indicator translated/scaled to the active tab over `--dur-base`. Pure CSS transform, no library. |
| **Optimistic row** | On mutation, the row shows at `opacity: 0.6` with a pulsing left border until the server confirms, then settles to 1 over `--dur-fast`. On failure it flashes `--danger` once and reverts. |
| **Map ↔ list sync** | Selecting in either pane highlights the other over `--dur-fast`; the map eases (not jumps) to the selection over `--dur-slow`, and **does not animate at all** under reduced motion. |

---

## 12. Layout & Route Transitions

### 12.1 Route transitions — the View Transitions API

Same-document view transitions are the primary mechanism. Support is real in Chromium and Safari and still absent in Firefox at time of writing, so **the API is always feature-detected and the fallback is an instant, correct navigation** — never a broken one.

```ts
export function navigateWithTransition(run: () => void) {
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (typeof document.startViewTransition !== 'function' || reduced) { run(); return; }
  document.startViewTransition(run).finished.catch(() => {});
}
```
`.catch(() => {})` is required: a rapid second navigation aborts the first and rejects.

```css
::view-transition-old(root) { animation: fade-out 120ms var(--ease-out-quart) both; }
::view-transition-new(root) { animation: page-enter 260ms var(--ease-out-quart) both; }
```

### 12.2 The `template.tsx` rule

Route-enter animation is declared in **`template.tsx`, not `layout.tsx`**. Next.js re-instantiates a template on every navigation but preserves a layout, so an animation in `layout.tsx` plays once and never again:

```tsx
// app/(admin)/template.tsx
export default function Template({ children }: { children: React.ReactNode }) {
  return <div className="motion-safe:animate-[page-enter_360ms_var(--ease-out-quart)]">{children}</div>;
}
```

### 12.3 Shared elements

`view-transition-name` on genuinely continuous elements only — **one pair per navigation**. Names must be unique per document, so they are assigned dynamically to the active item only:

| From → To | Shared element |
|---|---|
| District list row → district detail | The district name heading |
| Map marker → place inspector | The place title |
| Dataset list row → release detail | The version badge |

### 12.4 Shell layout transitions

Sidebar collapse animates **two properties in lockstep** — the rail's `width` and the content's `margin-left` — with **identical duration and easing** so it reads as one motion rather than two racing elements:

```css
.shell-sidebar { transition: width var(--dur-base) var(--ease-in-out); }
.shell-content { transition: margin-left var(--dur-base) var(--ease-in-out); }
```
*(Directly adopted from Xtiitch, which pairs `width 220ms ease` with `margin-left 220ms ease` for exactly this reason.)*

Sidebar and navbar entrance play **once per session**, not per navigation — re-animating chrome on every route change is the most common way an app starts to feel slow.

---

## 13. Theme Toggle — The Circular Reveal

**The signature interaction.** RentOS and Xtiitch arrived at this independently, with near-identical maths; that convergence is why it is mandatory here.

The new theme wipes in as a circle expanding from the exact point the user clicked.

```ts
export async function applyThemeWithReveal(
  commit: () => void,
  origin?: { x: number; y: number },
) {
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
  if (typeof document.startViewTransition !== 'function' || reduced || !origin) {
    commit();
    return;
  }
  const { x, y } = origin;
  const endRadius = Math.hypot(
    Math.max(x, innerWidth  - x),
    Math.max(y, innerHeight - y),
  );
  const transition = document.startViewTransition(() => flushSync(commit));
  try {
    await transition.ready;
    document.documentElement.animate(
      { clipPath: [`circle(0px at ${x}px ${y}px)`,
                   `circle(${endRadius}px at ${x}px ${y}px)`] },
      { duration: 520,
        easing: 'cubic-bezier(0.4, 0, 0.2, 1)',
        pseudoElement: '::view-transition-new(root)' },
    );
  } catch { /* aborted by a rapid re-toggle */ }
}
```

Supporting CSS must neutralise the browser's default crossfade, or the wipe is invisible under it:

```css
::view-transition-old(root),
::view-transition-new(root) { animation: none; mix-blend-mode: normal; }
::view-transition-old(root) { z-index: 1; }
::view-transition-new(root) { z-index: 9999; }
```

**Requirements.** `flushSync` is mandatory — React 19 batches by default and the transition would capture the *old* DOM without it. The click coordinates come from `event.clientX/clientY` on the toggle. The same function serves the **material** switch, so changing neu → glass wipes identically. The icon itself crossfades with `rotate-90 scale-0 opacity-0` ↔ `rotate-0 scale-100 opacity-100` over 300 ms.

---

## 14. The App Shell — Sidebar

One `<AppShell>` in `packages/ui` serves `apps/admin`, `apps/portal` and `apps/sandbox`. Navigation is **data, not JSX**.

### 14.1 Geometry

| Property | Value | Provenance |
|---|---|---|
| Expanded width | `288px` | RentOS 256 · Xtiitch 296 · AuraEDU 288 — the median, and an 8pt multiple |
| Collapsed width | `80px` | Fits a 24px icon + 44px touch target with breathing room |
| Header row height | `64px` | **Exactly `--shell-navbar-h`**, so the logo row and the navbar align on one line across the whole shell (RentOS's best structural idea) |
| Position | `fixed`, `inset: 0 auto 0 0`, `height: 100dvh` | `dvh`, not `vh` — mobile browser chrome |
| z-index | `40` | **Above** the navbar's `30`, so the mobile drawer covers the bar rather than sliding under it |
| Breakpoint | Rail ≥ `1024px`; below that, a Sheet drawer | |

### 14.2 Structure

```
┌─ <aside> fixed, flex column, 100dvh ────────┐
│ BRAND ROW              h-64, flex-shrink-0  │  logo + wordmark; mark only when collapsed
├─────────────────────────────────────────────┤
│ CONTEXT CARD           mx-3, collapsible    │  dataset version + environment; hidden when collapsed
├─────────────────────────────────────────────┤
│ <nav> flex-1 overflow-y-auto                │  ◄── the ONLY scroll region
│   ▸ group                                   │
│     · item · item · item                    │
│   ▸ group                                   │
├─────────────────────────────────────────────┤
│ FOOTER                 flex-shrink-0        │  pin toggle · help · user card
└─────────────────────────────────────────────┘
```

Brand row and footer are `flex-shrink-0`; only `<nav>` scrolls. *(AuraEDU scrolls the whole aside, so the brand scrolls away — do not copy that.)* Desktop hides the scrollbar (`scrollbar-width: none`); the mobile drawer shows a thin one. Footer padding uses `calc(var(--space-3) + env(safe-area-inset-bottom))`.

### 14.3 The navigation data model

```ts
export type NavItem = {
  id: string;
  label: string;
  href: string;
  icon: LucideIcon;
  roles: AdminRole[];          // Appendix C of agent_plan.md
  badge?: () => number;        // live count, subscribed not polled
  exact?: boolean;
};
export type NavGroup = {
  id: string;
  label: string | null;        // null = unlabelled pinned group
  icon: LucideIcon;
  roles: AdminRole[];
  defaultOpen: boolean;
  items: NavItem[];
};
```
Owned centrally at `apps/admin/src/config/navigation.ts` (lane L11, §8 of the plan). One file, one owner, no concurrent-edit conflicts.

### 14.4 Behaviours

| Behaviour | Spec |
|---|---|
| **Collapse** | Toggled from the navbar (not from inside the rail). **Persisted** to `ghanageo-sidebar`. *(Correction: Xtiitch's collapse is `useState` only, so every reload reopens expanded.)* |
| **Accordion groups** | Independently toggleable, not exclusive. Open set persisted. |
| **Auto-open active** | A group containing the current route force-opens **even if the user collapsed it**. To let the user still shut it, clicking a force-opened group records `dismissedPath = pathname`; it stays shut for that exact path only and re-opens on any navigation. *(RentOS's cleverest behaviour — adopted verbatim.)* |
| **Single-item groups flatten** | A group whose role filter leaves one item renders as a flat link with no header. *(RentOS.)* |
| **Role gating** | Three layers: filter groups by role → filter items by role → drop groups left empty. **An item a role cannot read is hidden entirely; a route it can read but not mutate renders with write controls disabled and a tooltip naming the required role.** Never show a nav item that dead-ends in a 403. *(Correction: Xtiitch role-gates nothing in the nav — every operator sees all 23 sections.)* |
| **Server enforcement** | Nav filtering is presentation. The API enforces the same policy independently. Hiding is never the only control. |
| **Badges** | Live counts on Review Queue, Conflicts, Import Failures, Security Events, Support. Group headers show a **roll-up badge = sum of children**, as a dot when collapsed and a pill when expanded. *(Xtiitch.)* **Counts are subscribed, never hardcoded.** *(Correction: AuraEDU's notification bell renders a literal `0`.)* |
| **Collapsed mode** | Items become icon-only tiles wrapped in a Radix Tooltip, `side="right"`. Group headers disappear; items flatten. |
| **Active state** | `aria-current="page"`. Visual = `--accent` fill + a 3px `--primary` left indicator + `--fg` text. In `neu` the active item is **inset** (pressed in); in `glass` it is a brighter pane; in `clay` it is a lifted chip. |
| **Hover nudge** | `translateX(1px)` when expanded (leans toward content), `translateY(-1px)` when collapsed (lifts). 180 ms. *(Xtiitch.)* |
| **Keyboard** | `↑`/`↓` roving tabindex within a group, `←`/`→` collapse/expand a group, `Home`/`End` jump. `[` toggles the rail. |
| **Mobile** | Radix Sheet, `side="left"`, width `min(88vw, 320px)`, scrim `--overlay` with a light blur, `keepMounted` for instant reopen. Opening the drawer **forces the expanded presentation** regardless of persisted collapse. |
| **Pin/unpin** | Footer control. Unpinned, the rail overlays content instead of pushing it — valuable on the map explorer where horizontal space is scarce. |

### 14.5 Admin navigation — the complete IA

Roles: **SA** Super Admin · **DA** Data Admin · **DR** Data Reviewer · **DC** Data Contributor · **DS** Developer Support · **SEC** Security/Auditor. Routes are relative to `admin.ghanageo.dev`.

Two global route rules: every list route accepts `?view=<savedViewId>`, and **every route accepts `?v=<datasetVersion>`** so any screen is linkable at a specific dataset version. Entity routes use the internal ULID, never the human code (plan R7).

| # | Group | Icon | Items (route · roles) |
|---|---|---|---|
| 1 | *(unlabelled, pinned)* | — | **Home** `/` all · **My Work** `/my-work` SA DA DR DC DS · **Activity Feed** `/activity` all |
| 2 | **Explore** | `compass` | **Location Explorer** `/explorer` all *(write DA/SA; DC proposes)* · **Geometry Workbench** `/explorer/geometry` SA DA, DR/SEC read · **Coverage Map** `/explorer/coverage` all |
| 3 | **Geography** | `globe` | **Regions** `/geography/regions` · **Districts / MMDAs** `/geography/districts` · **Places / Localities** `/geography/places` · **Aliases & Name Variants** `/geography/aliases` · **Roads** `/geography/roads` · **Points of Interest** `/geography/pois` · **Postal Areas** `/geography/postal-areas` · **Merges & Redirects** `/geography/redirects` · **Deprecations** `/geography/deprecated` — *all read; write DA/SA; DC proposes* |
| 4 | **Ingestion & Sources** | `workflow` | **Pipeline Overview** `/ingest` · **Sources Register** `/ingest/sources` · **Licences & Attribution** `/ingest/licences` · **Connectors & Adapters** `/ingest/connectors` · **Import Runs** `/ingest/runs` · **Raw Landing** `/ingest/raw` · **Normalization Rules** `/ingest/normalization` · **Matching & Dedupe** `/ingest/matching` · **Duplicate Candidates** `/ingest/duplicates` · **Reconciliation Conflicts** `/ingest/conflicts` · **Geometry Validation** `/ingest/geometry` · **Quality Scores** `/ingest/quality` · **Seed Data** `/ingest/seed` |
| 5 | **Review & Change Control** | `clipboard-check` | **Review Queue** `/review` *(badge)* · **Change Requests** `/review/change-requests` · **Community Submissions** `/review/submissions` · **My Drafts** `/review/drafts` · **Evidence Locker** `/review/evidence` · **Approval Policies** `/review/policies` |
| 6 | **Releases & Versioning** | `package` | **Dataset Versions** `/releases/versions` · **Release Pipeline** `/releases/pipeline` · **Validation Gates** `/releases/validation` · **Version Diff** `/releases/diff` · **Changelog Composer** `/releases/changelog` · **Exports & Downloads** `/releases/exports` · **Rollback** `/releases/rollback` |
| 7 | **Search & Relevance** | `search` | **Query Tester** `/search-ops/tester` · **Ranking & Boosts** `/search-ops/ranking` · **Synonyms & Abbreviations** `/search-ops/synonyms` · **Zero-Result Queries** `/search-ops/gaps` · **Geocoder Eval Sets** `/search-ops/eval` · **Index Status** `/search-ops/index` |
| 8 | **Developers & Access** | `key-round` | **Organizations** `/developers/orgs` · **Developer Users** `/developers/users` · **Applications** `/developers/apps` · **API Keys** `/developers/keys` · **Scopes & Permissions** `/developers/scopes` · **Rate-Limit Plans** `/developers/plans` · **Quota & Usage** `/developers/usage` · **Request Logs** `/developers/logs` · **Support Queue** `/developers/support` — *SA, DS, SEC (read)* |
| 9 | **Platform Health** | `heart-pulse` | **Service Health** `/ops/health` · **Queues & Workers** `/ops/queues` · **Outbox & Dead Letters** `/ops/dlq` · **ETL Freshness** `/ops/freshness` · **SLOs & Error Budgets** `/ops/slo` · **Latency & Traffic** `/ops/performance` · **Cache & Redis** `/ops/cache` · **Database** `/ops/database` · **Feature Flags** `/ops/flags` · **Environments** `/ops/environments` |
| 10 | **Security & Audit** | `shield` | **Audit Log** `/security/audit` · **Security Events** `/security/events` · **Access Reviews** `/security/access-reviews` · **Admin Sessions** `/security/sessions` · **MFA & Passkey Policy** `/security/mfa` · **Network & CORS** `/security/network` · **Privacy & Retention** `/security/privacy` · **Compliance Exports** `/security/exports` — *SEC full read, mutation on none* |
| 11 | **Content & Public** | `megaphone` | **Public Changelog** `/content/changelog` · **Documentation Pages** `/content/docs` · **Roadmap** `/content/roadmap` · **Coverage Page Data** `/content/coverage` · **Status Notices** `/content/status` · **Announcements & Banners** `/content/announcements` |

Settings lives in the user menu, not the rail: `/settings/profile`, `/settings/appearance`, `/settings/notifications`, `/settings/team`, `/settings/roles`, `/settings/system`.

---

## 15. The App Shell — Navbar

### 15.1 Geometry

| Property | Value |
|---|---|
| Height | `64px` standard · `48px` in **immersive mode** (explorer, geometry workbench, diff review) |
| Position | `sticky top-0`, z-index `30` — **below** the sidebar's 40 |
| Background | `--surface-raised` + `--mat-backdrop` (blur applies in `glass` only) |
| Bottom edge | 1px `--border`; **in Production, a 2px `--danger` hairline instead** |
| Regions | **Three:** left (context) · centre (search) · right (actions) |

> **The centre region is the correction.** All three reference shells have only left and right regions and **no working global search**: RentOS renders a search field that is pure decoration — no `value`, no `onChange`, no handler bound to its own `/` hint — and Xtiitch admin has none at all (its component is literally named `Search.tsx` but contains only toggles and a title). For a product whose entire purpose is *finding places*, search is the primary interaction and it belongs in the middle of the bar, permanently visible.

### 15.2 Complete action inventory, left to right

**LEFT — context**

| # | Control | Icon | Behaviour |
|---|---|---|---|
| 1 | Mobile menu | `menu` | `<1024px` only. Opens the Sheet. `aria-label="Open navigation"` |
| 2 | Rail toggle | `panel-left-close` / `panel-left-open` | `≥1024px` only. Label and tooltip flip between "Collapse sidebar" / "Expand sidebar". `[` shortcut |
| 3 | App switcher | `grid-3x3` | Menu → Admin · Developer Portal · Sandbox · Docs · Marketing · Status. **Preserves the current environment.** |
| 4 | Breadcrumb trail | — | `Ghana › Greater Accra › Accra Metropolitan › Osu`. **Each segment is a dropdown of its siblings** — so a steward moves laterally between districts without going back to a list. Collapses to a single chip below 1100px. *(All three references have no breadcrumbs at all; for hierarchical geography that is the wrong call.)* |

**CENTRE — search**

| # | Control | Behaviour |
|---|---|---|
| 5 | **Global search** | 320–420px input at ≥1280px, icon button below. Placeholder *"Search places, districts, keys, runs…"*. `⌘K` / `Ctrl K` hint chip. **Fully functional** — opens the command palette (§17). |

**RIGHT — actions**

| # | Control | Icon | Behaviour |
|---|---|---|---|
| 6 | **Create** | `plus` | Primary-styled split button. Role-filtered menu: New Place · New District · New Alias · New Import Run · New Change Request · New Dataset Release. **Hidden entirely for SEC.** |
| 7 | **Dataset version selector** | `package` | Reads *"Dataset 2026.08.1 · published"* or *"Canonical staging · working"*. **The single most consequential control in the product** — it changes what every list, map and detail screen reads. Menu lists published versions + staging + a pinned "compare to…". Selecting rewrites `?v=` on the current route. Non-production versions show a persistent amber chip. |
| 8 | **Environment badge** | `boxes` | Colour-coded pill: Production `--danger` outline · Staging `--warning` · Sandbox violet · Test/CI grey · Local `--success`. Production also tints the navbar hairline (§15.1) so the environment is unmistakable at a glance. |
| 9 | **Pipeline health** | `activity` | Pulse dot (green/amber/red) + the worst-stage name at ≥1440px ("Reconcile · lagging"). Click → `/ingest`. Popover shows per-stage lag. |
| 10 | **Review queue counter** | `clipboard-check` | Badge = unassigned + assigned-to-me, `max={99}`. Click → `/review`. Hidden for DS and SEC. |
| 11 | **Notifications** | `bell` | Radix Popover, **not** a route jump. *(Correction: Xtiitch's bell navigates to a section instead of opening a panel; AuraEDU's count is hardcoded 0.)* See §16.2. |
| 12 | **Help** | `circle-help` | Menu: Section guide (contextual) · Docs · Keyboard shortcuts (`?`) · API status · Report an issue · What's new. |
| 13 | **Theme** | `sun` / `moon` | Circular reveal (§13). Long-press or right-click opens the full picker. |
| 14 | **User menu** | avatar | See §16.1. |

Every icon-only control has an `aria-label` **and** a `title`, and a Radix Tooltip. Below `768px`, controls 3, 9, 12 collapse into an overflow `⋯` menu; 7 and 8 **never** collapse — environment and dataset version must always be visible.

### 15.3 What is *not* in the navbar

No page title and no greeting. Page identity lives in the `PageHeader` inside `<main>`, where it can carry description, status chips, and primary actions with room to breathe. *(All three references agree on this, arriving at it independently.)*

---

## 16. Menus & Dropdowns

All menus are Radix (`DropdownMenu`, `Popover`, `Select`, `Command`) so keyboard, focus trapping, typeahead and dismissal are correct by construction.

### 16.1 Shared anatomy

Every menu uses the same three-zone structure, adopted from AuraEDU and Xtiitch which converged on it:

```
┌────────────────────────────────────┐
│ IDENTITY / HEADER BAND             │  non-interactive context, tinted --accent
├────────────────────────────────────┤
│ items (label + optional helper)    │  grouped, with section labels
│ ─────────────────────────────────  │  divider
│ items                              │
├────────────────────────────────────┤
│ ─────────────────────────────────  │
│ DESTRUCTIVE ACTION                 │  --danger, always last, always after a divider
└────────────────────────────────────┘
```

**Rows are rich:** a label plus a helper line, not a bare word. `min-width: 288px`, `max-width: calc(100vw - 32px)`, `--mat-radius`, `--surface-raised`, `--mat-shadow-lifted`, `slide-up 180 ms` with `transform-origin` from Radix's `data-side`.

**Every item goes somewhere distinct.** *(Correction: Xtiitch's user menu has two items — "Profile settings" and "Platform settings" — that both navigate to the same section with no deep link. Two rows, one destination, is a dead end.)*

### 16.2 The menus, in full

**① User menu** — trigger: 32px avatar of initials.
- *Identity band:* 46px avatar · display name · email · role chip (`"Data Admin access"`) · MFA status dot.
- *Account:* **Profile** `/settings/profile` — "Name, email, avatar" · **Appearance** `/settings/appearance` — "Theme, material, density" · **Notifications** `/settings/notifications` — "What reaches you, and where" · **Security** `/settings/security` — "Passkeys, MFA, sessions".
- *Workspace* (SA only): **Team & roles** `/settings/team` · **System settings** `/settings/system`.
- *Divider.* **Keyboard shortcuts** `?` · **Documentation** ↗ · **API status** ↗.
- *Divider.* **Sign out** — `--danger`. Signs out **this** session; a "Sign out everywhere" link sits inside `/settings/security`.

**② Notifications popover** — trigger: bell + live badge. Width 400px, max-height 480px.
- *Header:* "Notifications" · unread count · **Mark all read** · gear → `/settings/notifications`.
- *Tabs:* All · Mentions · Assignments · System.
- *Rows:* type icon · title · one-line context · relative time · unread dot. Grouped Today / Earlier. Click marks read and deep-links to the exact entity.
- *Empty:* an illustration and "You're all caught up" — never a blank panel.
- *Footer:* **View all activity** → `/activity`.
- Counts arrive over the existing change-feed connection with polling fallback. **Never hardcoded.**

**③ Dataset version selector** — the product's most important menu.
- *Header band:* currently-served production version + its checksum, monospace.
- *Sections:* **Published** (versions, newest first, with published-at and a "serving" marker) · **Working** (Canonical staging · My draft) · *divider* · **Compare two versions…** → `/releases/diff` · **Release pipeline** → `/releases/pipeline`.
- Selecting a non-production version raises a persistent amber chip in the navbar that stays until cleared. Read-only versions disable every write control app-wide with the tooltip *"Editing is disabled while viewing a published version."*

**④ Create menu** — role-filtered; renders nothing (and the button is hidden) if the role can create nothing. Grouped **Geography** / **Data operations** / **Review**, each row carrying its keyboard shortcut.

**⑤ Environment switcher** — SA only. Rows: Production · Staging · Sandbox · Local, each with a health dot and the current dataset version. Switching **to Production requires a typed confirmation** of the word `production`.

**⑥ Breadcrumb segment menus** — clicking `Greater Accra` lists sibling regions with a filter input above 12 items. Lateral movement without a round trip to a list screen.

**⑦ App switcher** — six destinations, each with icon, name and a one-line description; opens in the same tab, preserving environment.

**⑧ Help menu** — Section guide (opens a contextual right-hand drawer for the current screen) · Documentation ↗ · Keyboard shortcuts (`?`) · API status ↗ · Report an issue · What's new (badge when unread).

**⑨ Table row actions** — trigger `more-horizontal`. Grouped **View** / **Edit** / **Data ops**, with the destructive action after a divider. Deprecate and Merge open confirmation dialogs that state the consequence (*"3 aliases and 1 redirect will follow this record"*). Nothing destructive is one click away.

**⑩ Bulk-action bar** — not a dropdown but a menu surface: on selection, a floating bar rises from the bottom (`slide-up`, `--surface-raised`, `--mat-shadow-lifted`) with the count, the actions, an overflow menu, and **Clear selection**. It never covers the last table row — the table gains bottom padding equal to the bar's height.

**⑪ Language switcher** — English · Twi · Ga · Ewe, checkmark on the active row. *(RentOS ships exactly these four.)* Persisted server-side when signed in, not just to `i18n`.

**⑫ Density menu** — Comfortable / Compact, affecting table row height and shell padding only.

---

## 17. Command Palette & Global Search

`⌘K` / `Ctrl K` from anywhere. Built on Radix/`cmdk`. **This is the single largest improvement over all three reference shells, none of which has a working one.**

### 17.1 Modes

The palette switches mode on a leading sigil, so one input serves every search:

| Prefix | Mode | Searches |
|---|---|---|
| *(none)* | Everything | Places, districts, regions, then commands, then recent |
| `>` | Commands | "Publish release", "Run import", "Toggle dark mode" |
| `#` | Navigation | Every route in §14.5, by label and by group |
| `@` | People & orgs | Developers, organizations, stewards |
| `/` | Entities by ID | ULID, key prefix, request ID, import-run ID |
| `?` | Help | Docs pages, error codes, keyboard shortcuts |

### 17.2 Behaviour

- **Debounce 180 ms; minimum 2 characters.** Below the minimum it shows recents and suggested actions rather than firing a request — the same rule the `@ghanageo/react` SDK enforces (plan GEO-13.3).
- **Every keystroke aborts the previous request** via `AbortSignal`.
- Results are grouped and **keyboard-first**: `↑`/`↓` move, `Enter` opens, `⌘Enter` opens in a new tab, `Esc` closes, `Tab` cycles groups.
- **Geographic results carry their hierarchy** — `Osu · Suburb · Accra Metropolitan, Greater Accra` — because "Osu" alone is ambiguous, and Spec §10 requires ambiguity to surface rather than be guessed away.
- Each result shows its **verification status chip** and, where relevant, a **confidence score with the match explanation** ("alias match: *Osu RE*").
- **Zero results is a feature, not a dead end:** it offers *"Create this place"*, *"Propose an alias"*, and *"Log as a zero-result query"* — feeding `/search-ops/gaps`, which the IA identifies as the product's highest-value data-acquisition loop.
- Recent searches and recently-viewed entities persist per user.
- Opening animates `scale-in` 200 ms; the backdrop `fade-in` 160 ms with a light blur in `glass`.

---

## 18. Map Surfaces

Leaflet with OpenStreetMap raster tiles. No API key, ODbL attribution always visible. This section exists because **the materials interact badly with maps if unmanaged.**

### 18.1 Map theming

Raster OSM tiles are fixed-colour, so dark mode uses a CSS filter on the tile pane rather than a second tile set:

```css
[data-mode="dark"] .leaflet-tile-pane {
  filter: invert(1) hue-rotate(180deg) brightness(0.92) contrast(0.88) saturate(0.72);
}
```
Applied to the **tile pane only** — never the whole map, or markers, labels and GeoJSON overlays invert too. Overlay colours come from tokens and are chosen to survive both treatments.

| Layer | Light | Dark |
|---|---|---|
| Region boundary | `--ink-700` 2px | `--ink-200` 2px |
| District boundary | `--ink-500` 1.5px dashed | `--ink-300` 1.5px dashed |
| Selected feature | `--primary` 3px, fill 12% | same |
| Invalid geometry | `--danger` 2px, hatched fill | same |
| Unreviewed | `--warning` dashed | same |
| Cluster bubble | `--surface-raised` + `--mat-shadow-raised` | same |

### 18.2 Panels over maps — the legibility rule

A glass panel over a map is beautiful and, unmanaged, unreadable: the contrast behind it changes as the user pans, so it cannot be verified statically.

**Every panel floating over a map gets an opaque scrim beneath its blur**, in all three materials:

```css
.map-panel {
  background:
    linear-gradient(var(--surface-scrim), var(--surface-scrim)),  /* ← opaque floor */
    var(--surface);
  backdrop-filter: var(--mat-backdrop, none);
  box-shadow: var(--mat-shadow-lifted);
  border-radius: var(--mat-radius);
}
:root                        { --surface-scrim: oklch(1 0 0 / 0.82); }
[data-mode="dark"]           { --surface-scrim: oklch(0.20 0.02 258 / 0.86); }
[data-material="neu"]        { --surface-scrim: var(--surface); }  /* fully opaque */
```
Contrast is then measured against the scrim, which is deterministic — so the 4.5:1 floor holds regardless of what is on the map. Blur remains permitted here because map panels are fixed-size chrome, which is exactly the case §5.2 allows.

### 18.3 The three-pane explorer

```
┌─────────┬───────────────────────────────────┬──────────────┐
│ TREE    │            MAP                    │  INSPECTOR   │
│ 320px   │            flex-1                 │  400px       │
│         │                                   │              │
│ Ghana   │   ┌──────────────────────┐        │  Osu         │
│ ▸Ahafo  │   │ floating filter bar  │        │  SUBURB      │
│ ▾GtAccra│   └──────────────────────┘        │  ─────────   │
│  ·Accra │                                   │  Aliases     │
│   ·Osu  │              [ + ][ − ][ ⛶ ]      │  Geometry    │
│         │   ┌─────────────┐                 │  Provenance  │
│         │   │ legend      │  attribution    │  History     │
└─────────┴───┴─────────────┴─────────────────┴──────────────┘
```
- Both side panes are collapsible; state persisted. The map never re-mounts on collapse — it calls `invalidateSize()` after the 220 ms transition ends.
- Selection is synchronised three ways: tree ↔ map ↔ inspector, each highlighting over `--dur-fast`. The map **eases** to a selection over `--dur-slow`, and does not animate at all under reduced motion.
- The whole screen runs `data-intensity="restrained"` on the tree and inspector, `balanced` on floating map chrome.
- The URL carries selection and viewport, so any view is shareable.
- **Keyboard-complete:** the tree is a full ARIA `tree` widget; the map is reachable and pannable by keyboard; every map action has a non-map equivalent, because a map alone is not an accessible interface.

---

## 19. Data-Dense Surfaces

Where `restrained` intensity is mandatory (§3.3). The rule: **material expresses itself in borders and tint here, never in shadow or blur.**

### 19.1 Tables

- Sticky header (`--bg-subtle`) and, where an identity column exists, a sticky first column.
- Row height 44px comfortable / 36px compact; `--text-sm`; **numeric and ID columns `font-variant-numeric: tabular-nums`** so digits do not jitter between rows.
- Zebra striping **off** — a 1px `--border` row separator instead; stripes fight all three materials.
- Row hover `--accent` at 40%; selected `--accent`; focused row a 2px inset `--ring`.
- Every table has: column visibility, sort, filter, saved views, CSV export, and **keyboard row navigation**.
- Virtualised beyond 100 rows. Skeleton rows mirror the real column widths.
- Horizontal overflow scrolls **inside the table container**; the page body never scrolls sideways.

### 19.2 Code, JSON and diffs

- One syntax theme per mode, derived from the ink ramp and status tokens so it is material-independent and brand-independent. Code never picks up the user's brand hue — a red keyword must stay red.
- Code blocks sit on `--bg-subtle` with a 1px border, `--mat-radius-sm`, **never a shadow and never blur** — blur over monospace is unreadable.
- Diff views: additions `--success` at 12% with a `+` gutter, deletions `--danger` at 12% with `−`; **never colour alone** (§6.5).
- JSON viewer: collapsible nodes, type-coloured values, a copy-path affordance per key, search within.
- The change-request diff screen is the highest-density surface in the product and is `restrained` throughout.

### 19.3 Forms

- One column; label above field; helper text below; error replaces helper and is announced via `aria-live="polite"`.
- Inputs use `--surface-sunken` and `--mat-shadow-inset` — the one place the inset treatment is used in every material, because "you may type here" is exactly what concave means.
- Required marked on the label, not by colour.
- Destructive confirmations require typing the entity name.
- Unsaved-changes guard on navigation.

---

## 20. Marketing Surfaces

`apps/web` runs `expressive` and is the only place `motion` and GSAP are permitted.

- **Hero:** blur-in headline (§10.2), a **live, working** location search box against the public API, and a protocol selector rendering the same query as REST / GraphQL / gRPC — the Spec §16 requirement.
- **Scroll choreography:** GSAP ScrollTrigger; at most **one** pinned section on the whole site; parallax on decorative layers only, never on body copy.
- **Counting stats** for coverage numbers (16 regions · 261 districts) with `tabular-nums`.
- **Material showcase:** the marketing site demonstrates the three materials by rendering its own feature cards in them, with the theme picker in the footer — the design system as a selling point.
- Content is visible by default; reveal animations are applied only after `IntersectionObserver` support is confirmed, so crawlers and no-JS users see everything.
- Core Web Vitals are a release gate: no layout shift from text animation, fonts `display: swap` with metric-matched fallbacks.

---

## 21. Component Inventory → Plan Mapping

| Component | Package path | Plan story |
|---|---|---|
| Token layers, material packs | `packages/ui/src/styles/` | GEO-14.1 |
| `ThemeProvider`, FOUC script, contrast clamp | `packages/ui/src/theme/` | GEO-14.2 |
| Button, Input, Select, Checkbox, Radio, Switch, Card, Badge, Tabs, Dialog, Sheet, Popover, Dropdown, Tooltip, Toast, Skeleton, Pagination, Breadcrumb, Avatar, Progress, Alert, Combobox | `packages/ui/src/components/` | GEO-14.3 |
| Keyframes, transition helpers, `navigateWithTransition` | `packages/ui/src/motion/` | GEO-14.4 |
| `AppShell`, `Sidebar`, `Navbar`, `PageHeader`, `EmptyState` | `packages/ui/src/shell/` | GEO-14.5 |
| `CommandPalette` | `packages/ui/src/shell/command-palette/` | GEO-14.5 |
| `ThemePicker` | `packages/ui/src/theme/picker/` | GEO-14.6 |
| `DataTable`, `JsonViewer`, `DiffView`, `CodeBlock` | `packages/ui/src/data/` | GEO-14.3 |
| `MapCanvas`, `MapPanel`, `GeoJsonLayer`, `HierarchyTree` | `packages/ui/src/map/` | GEO-17.2, GEO-15.5 |
| Design-QA harness (axe-core + Playwright matrix) | `packages/ui/tests/` | GEO-14.7 |

---

## 22. Design-QA Checklist

Every frontend PR runs this. It is a **required CI check** (GEO-14.7), and results append to `design-qa.md` in the house format: evidence → findings (P0–P3) → fix → post-fix evidence → a literal `final result: passed | blocked` line.

### 22.1 The matrix — every surface, every point

Automated Playwright + axe-core across **3 materials × 3 modes (light/dark/custom-hue) × 5 viewports** (320, 390, 768, 1280, 1920).

### 22.2 Gates

**Material**
- [ ] Renders correctly in `neu`, `glass` and `clay`
- [ ] Correct in light, dark, and a non-default brand hue
- [ ] No hardcoded colour, shadow, radius or blur — tokens only
- [ ] No `if (material === …)` branch in any component
- [ ] Correct `data-intensity` for the surface's density
- [ ] Glass surfaces have an opaque `@supports not (backdrop-filter)` fallback
- [ ] No `backdrop-filter` on a scrolling or repeated element

**Accessibility**
- [ ] axe-core: zero violations
- [ ] Text ≥ 4.5:1, UI boundaries ≥ 3:1 — **verified in all three materials**
- [ ] Focus ring visible on every interactive element in every material, via `outline`
- [ ] Full keyboard operation; logical focus order; no traps
- [ ] `forced-colors: active` renders a usable bordered UI
- [ ] Targets ≥ 44px (24px for inline table controls)
- [ ] Colour is never the sole carrier of meaning
- [ ] Split text carries `aria-label` on the parent and `aria-hidden` on spans
- [ ] Decorative layers are `aria-hidden` and non-interactive

**Motion**
- [ ] `prefers-reduced-motion` honoured — verified by emulating it, not by reading the code
- [ ] Only `transform`/`opacity` animated (except the two sanctioned exceptions in §9.2)
- [ ] No animation longer than 520 ms
- [ ] `will-change` removed after use
- [ ] View transitions feature-detected with a working fallback

**Responsive & state**
- [ ] No horizontal body overflow at any of the five viewports
- [ ] Loading, empty, error and permission-denied states implemented
- [ ] Skeletons mirror the real layout
- [ ] Long content, long names and zero rows all handled
- [ ] Production build clean; no console errors or warnings

**Shell**
- [ ] Sidebar collapse persists across reload
- [ ] Role gating verified for all six roles; **no nav item dead-ends in a 403**
- [ ] Badge counts are live, never hardcoded
- [ ] Every menu item has a distinct destination
- [ ] No flash of wrong theme on cold load (throttled 3G, cache disabled)

---

## Appendix — Concrete Values (copy these)

```css
/* Shell */
--shell-sidebar-w: 288px;  --shell-sidebar-collapsed-w: 80px;
--shell-navbar-h: 64px;    --shell-navbar-h-immersive: 48px;
--shell-content-max: 1480px;
z-index: sidebar 40 · navbar 30 · dropdown 50 · dialog 60 · toast 70 · palette 80

/* Motion */
--dur-instant: 90ms   --dur-fast: 150ms   --dur-base: 220ms
--dur-slow: 360ms     --dur-slower: 520ms
--ease-out-quart: cubic-bezier(0.25, 1, 0.5, 1)
--ease-in-out:    cubic-bezier(0.4, 0, 0.2, 1)
--ease-spring:    cubic-bezier(0.34, 1.56, 0.64, 1)

/* Named surface transitions */
tooltip      fade-in  120ms      dropdown/sheet overlay  fade-in  160ms
popover      slide-up 180ms      dialog content          scale-in 200ms
page enter   page-enter 360ms    theme reveal            520ms
sidebar collapse  width + margin-left 220ms --ease-in-out (identical on both)
skeleton shimmer  1400ms sine.inOut infinite

/* Stagger */
list items 30ms, capped at 8 · marketing words 60ms · marketing chars 15ms

/* Radii by material */
neu   --mat-radius: 1rem     (16px)   --mat-radius-sm: 0.75rem
glass --mat-radius: 1.125rem (18px)   --mat-radius-sm: 0.75rem
clay  --mat-radius: 1.75rem  (28px)   --mat-radius-sm: 1.125rem

/* Type */
display Bricolage Grotesque · UI Outfit · mono JetBrains Mono
admin body 13px · portal/marketing body 15px · nav eyebrow 11px
tabular-nums on every numeric and ID column

/* Density */
comfortable row 44px · compact row 36px · min target 44px (24px inline)

/* Brand presets (hue, chroma) */
meridian 168 0.115  ·  lagoon 232 0.120  ·  clay 42 0.105
kente    118 0.130  ·  ink    268 0.090
```

---

*This document is mandatory. If an implementation and this specification disagree, the specification wins — or the specification changes first, in its own PR. Append QA runs to `design-qa.md`.*
