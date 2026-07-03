// Shared kitchen countdown logic.
//
// The countdown for a line is a pure function of the line's own `created_at` and
// `priority` plus the current time `now`. Keeping `now` as a parameter (instead of
// calling Date.now() inside) lets every line derive its own independent countdown
// from a single shared clock — see `useNow` — so we don't run one interval per card.

export const COUNTDOWN_CONSTANT = 30; // 30 minutes base target

/**
 * Remaining milliseconds until this line is "due".
 * Allotted time shrinks as priority rises: p0 -> 30min, p1 -> 15, p2 -> 10, p3 -> 7.5.
 * `created_at` is a timestamptz instant (serialized as ...Z), so parse it directly —
 * no manual UTC offset. `now` is also an absolute instant (Date.now()).
 */
export function calculateRemainingMs(
  priority: number,
  createdAt: string,
  now: number
): number {
  const totalMinutes = COUNTDOWN_CONSTANT / (priority + 1);
  const totalMs = totalMinutes * 60 * 1000;
  const createdTime = new Date(createdAt).getTime();
  const elapsed = now - createdTime;
  return totalMs - elapsed;
}

export function formatCountdown(remainingMs: number): string {
  if (remainingMs <= 0) {
    return "¡URGENTE!";
  }

  const totalSeconds = Math.floor(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

export function getCountdownColor(remainingMs: number): {
  bg: string;
  text: string;
} {
  const minutes = remainingMs / 1000 / 60;

  if (remainingMs <= 0) {
    return { bg: "var(--color-danger)", text: "white" };
  } else if (minutes < 5) {
    return { bg: "var(--color-danger-light)", text: "var(--color-danger)" };
  } else if (minutes < 10) {
    return { bg: "var(--color-warning-light)", text: "var(--color-warning)" };
  } else {
    return { bg: "var(--color-success-light)", text: "var(--color-success)" };
  }
}

/**
 * Ambient row styling for a kitchen line so an urgent/near-due item is spottable
 * without scanning each badge. Returns a faint background tint + a solid left-accent
 * color (a stripe that scans down the column). Only near-due/urgent lines are tinted;
 * healthy lines return `null` so they stay neutral and the eye is drawn to what needs
 * attention. Thresholds mirror getCountdownColor. Escalation: <10min amber, <5min red,
 * overdue red (stronger). Don't call for completed lines — struck lines aren't urgent.
 */
export function getCountdownRowStyle(remainingMs: number): {
  background: string;
  accent: string;
} | null {
  const minutes = remainingMs / 1000 / 60;

  if (remainingMs <= 0) {
    return { background: "var(--color-danger-light)", accent: "var(--color-danger)" };
  } else if (minutes < 5) {
    return { background: "var(--color-danger-tint)", accent: "var(--color-danger)" };
  } else if (minutes < 10) {
    return { background: "var(--color-warning-tint)", accent: "var(--color-warning)" };
  }
  return null;
}
