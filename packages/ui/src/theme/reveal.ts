/**
 * The circular theme reveal: the new theme wipes in as a circle expanding
 * from the exact point the user clicked.
 *
 * Both RentOS and Xtiitch arrived at this independently with near-identical
 * maths; that convergence is why it is the house signature interaction.
 * (DESIGN_SYSTEM.md 13)
 */
export interface RevealOrigin {
  x: number;
  y: number;
}

export function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

export function supportsViewTransition(): boolean {
  return typeof document !== "undefined" && typeof document.startViewTransition === "function";
}

/**
 * `commit` must synchronously apply the theme. Callers wrap their React state
 * update in flushSync — React 19 batches by default, and without it the
 * transition captures the OLD DOM.
 */
export async function applyThemeWithReveal(
  commit: () => void,
  origin?: RevealOrigin,
): Promise<void> {
  if (!supportsViewTransition() || prefersReducedMotion() || !origin) {
    commit();
    return;
  }

  const { x, y } = origin;
  const endRadius = Math.hypot(
    Math.max(x, window.innerWidth - x),
    Math.max(y, window.innerHeight - y),
  );

  const transition = document.startViewTransition(commit);
  try {
    await transition.ready;
    document.documentElement.animate(
      {
        clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${endRadius}px at ${x}px ${y}px)`],
      },
      {
        duration: 520,
        easing: "cubic-bezier(0.4, 0, 0.2, 1)",
        pseudoElement: "::view-transition-new(root)",
      },
    );
  } catch {
    // A rapid re-toggle aborts the previous transition and rejects. Harmless.
  }
}
