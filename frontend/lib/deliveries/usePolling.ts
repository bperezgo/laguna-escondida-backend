"use client";

import { useCallback, useEffect, useRef } from "react";

export const DELIVERY_POLL_MS = 10_000;

/** Runs `load` now, every `intervalMs` while the page is visible, and on coming back to it. */
export function usePolling(load: () => Promise<void>, intervalMs: number = DELIVERY_POLL_MS): () => Promise<void> {
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  }, [load]);
  const refresh = useCallback(() => loadRef.current(), []);

  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | null = null;
    const start = () => {
      if (timer === null) timer = setInterval(() => void loadRef.current(), intervalMs);
    };
    const stop = () => {
      if (timer !== null) clearInterval(timer);
      timer = null;
    };
    const onVisibility = () => {
      if (document.visibilityState === "visible") {
        void loadRef.current();
        start();
      } else {
        stop();
      }
    };
    void loadRef.current();
    if (document.visibilityState === "visible") start();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stop();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [intervalMs]);

  return refresh;
}
