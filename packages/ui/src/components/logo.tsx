import type { SVGProps } from "react";

/**
 * The GhanaGeo mark: a meridian and a fixed point.
 *
 * Design reasoning — what it is NOT is as deliberate as what it is. The
 * research on Ghanaian brand voice flagged kente patterns, Adinkra-symbol
 * decoration and "gateway to Africa" imagery as clichés that read as tourism
 * rather than infrastructure. A national dataset should look like a instrument,
 * not a souvenir.
 *
 * So the mark is a globe's meridian crossed by a latitude line, with a filled
 * point where they meet: the literal act of fixing a location. It reads as a
 * globe, a coordinate crosshair and a map pin at once, and it survives 16px
 * because it is four primitives with no fine detail.
 *
 * "Meridian" is also the default brand hue, so the mark and the palette share
 * one idea rather than being decided separately.
 */
export function LogoMark({ size = 24, ...props }: { size?: number } & SVGProps<SVGSVGElement>) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 32 32"
      fill="none"
      role="img"
      aria-label="GhanaGeo"
      {...props}
    >
      {/* Globe */}
      <circle cx="16" cy="16" r="12.5" stroke="currentColor" strokeWidth="2" opacity="0.32" />
      {/* Meridian — an ellipse read as a great circle seen at an angle */}
      <ellipse cx="16" cy="16" rx="5.5" ry="12.5" stroke="currentColor" strokeWidth="2" opacity="0.55" />
      {/* Latitude */}
      <path d="M3.5 16h25" stroke="currentColor" strokeWidth="2" opacity="0.55" strokeLinecap="round" />
      {/* The fixed point: Accra sits south of centre and west of the meridian,
          so the dot is placed there rather than dead centre. */}
      <circle cx="13.2" cy="19.4" r="3.4" fill="currentColor" />
    </svg>
  );
}

export function Logo({
  size = 24,
  showWordmark = true,
  suffix,
  ...props
}: {
  size?: number;
  showWordmark?: boolean;
  /** e.g. "Admin", "Sandbox" — set in the UI face, not the display face. */
  suffix?: string;
} & SVGProps<SVGSVGElement>) {
  return (
    <span className="gg-logo">
      <LogoMark size={size} style={{ color: "var(--brand)", flexShrink: 0 }} {...props} />
      {showWordmark ? (
        <span style={{ display: "grid", lineHeight: 1.1, minWidth: 0 }}>
          <strong
            style={{
              fontFamily: "var(--font-display)",
              fontSize: "1.0625rem",
              fontWeight: 600,
              letterSpacing: "-0.015em",
              color: "var(--fg)",
              whiteSpace: "nowrap",
            }}
          >
            GhanaGeo
          </strong>
          {suffix ? (
            <span
              style={{
                fontFamily: "var(--font-sans)",
                fontSize: "10px",
                letterSpacing: "0.16em",
                textTransform: "uppercase",
                fontWeight: 700,
                color: "var(--fg-subtle)",
              }}
            >
              {suffix}
            </span>
          ) : null}
        </span>
      ) : null}
    </span>
  );
}
