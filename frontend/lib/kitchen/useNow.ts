"use client";

import { useState, useEffect } from "react";

/**
 * A single shared ticking clock. Returns `Date.now()` refreshed every `intervalMs`.
 *
 * Hoist one `useNow()` to a kitchen view and pass the value down to every line/card,
 * instead of running a `setInterval` per card. All lines derive their own countdown
 * from this one `now` — one heartbeat, many independent countdowns.
 */
export function useNow(intervalMs: number = 1000): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    const interval = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(interval);
  }, [intervalMs]);

  return now;
}
