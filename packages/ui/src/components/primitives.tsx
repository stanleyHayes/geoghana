"use client";

import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import {
  forwardRef,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type InputHTMLAttributes,
} from "react";
import { MapPin } from "lucide-react";
import { cn } from "../lib/utils";

/* Every component below reads ONLY semantic and --mat-* tokens. There is no
   `material === "glass"` branch anywhere in this file, and that is the whole
   proof of the architecture. (DESIGN_SYSTEM.md 5.5) */

// ---------------------------------------------------------------- Card

export const Card = forwardRef<
  HTMLDivElement,
  HTMLAttributes<HTMLDivElement> & { interactive?: boolean }
>(({ className, interactive, ...props }, ref) => (
  <div
    ref={ref}
    data-interactive={interactive ? "" : undefined}
    className={cn("gg-card", className)}
    {...props}
  />
));
Card.displayName = "Card";

// ---------------------------------------------------------------- Button

const buttonVariants = cva("gg-button", {
  variants: {
    variant: {
      primary: "gg-button--primary",
      secondary: "gg-button--secondary",
      ghost: "gg-button--ghost",
      danger: "gg-button--danger",
    },
    size: {
      sm: "gg-button--sm",
      md: "gg-button--md",
      lg: "gg-button--lg",
      icon: "gg-button--icon",
    },
  },
  defaultVariants: { variant: "secondary", size: "md" },
});

export interface ButtonProps
  extends
    ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        ref={ref}
        className={cn(buttonVariants({ variant, size }), className)}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";

// ---------------------------------------------------------------- Input

export const Input = forwardRef<
  HTMLInputElement,
  InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => (
  <input ref={ref} className={cn("gg-input", className)} {...props} />
));
Input.displayName = "Input";

// ---------------------------------------------------------------- Badge

const badgeVariants = cva("gg-badge", {
  variants: {
    tone: {
      neutral: "gg-badge--neutral",
      canonical: "gg-badge--canonical",
      reviewed: "gg-badge--reviewed",
      reference: "gg-badge--reference",
      needsRecon: "gg-badge--needs-recon",
      deprecated: "gg-badge--deprecated",
      danger: "gg-badge--danger",
    },
  },
  defaultVariants: { tone: "neutral" },
});

export interface BadgeProps
  extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, tone, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}

/** Maps a verification status to its badge tone, icon glyph and label.
 *  Colour is never the sole carrier: every badge also shows a glyph and text. */
export function verificationTone(status: string): {
  tone: NonNullable<BadgeProps["tone"]>;
  glyph: string;
  label: string;
} {
  switch (status) {
    case "CANONICAL":
      return { tone: "canonical", glyph: "✓", label: "Canonical" };
    case "REVIEWED":
      return { tone: "reviewed", glyph: "◆", label: "Reviewed" };
    case "SEED_NEEDS_CANONICAL_RECONCILIATION":
      return { tone: "needsRecon", glyph: "!", label: "Needs reconciliation" };
    case "REFERENCE":
      return { tone: "reference", glyph: "·", label: "Reference" };
    default:
      return { tone: "deprecated", glyph: "×", label: status };
  }
}

// ---------------------------------------------------------------- Skeleton

/** Skeletons mirror the real layout. Their fill differs per material —
 *  a shimmer sweep is invisible over backdrop-filter and reads as a foreign
 *  object on an extruded surface. (DESIGN_SYSTEM.md 5.7) */
export function Skeleton({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div aria-hidden className={cn("gg-skeleton", className)} {...props} />
  );
}

// ---------------------------------------------------------------- EmptyState

export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="gg-empty" role="status">
      <div className="gg-empty__icon" aria-hidden>
        <span className="gg-empty__orbit">
          <i />
          <i />
          <i />
        </span>
        {icon ?? <MapPin />}
      </div>
      <h2 className="gg-empty__title">{title}</h2>
      {description ? <p className="gg-empty__desc">{description}</p> : null}
      {action ? <div className="gg-empty__action">{action}</div> : null}
    </div>
  );
}

// ---------------------------------------------------------------- SkipLink

/** First focusable element on every page. None of the three reference shells
 *  ships one; that is a gap, not a precedent. */
export function SkipLink({ href = "#main" }: { href?: string }) {
  return (
    <a href={href} className="gg-skip-link">
      Skip to main content
    </a>
  );
}
