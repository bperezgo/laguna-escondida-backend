"use client";

import { useEffect, useRef, useState } from "react";
import type { Reservation } from "@/types/reservation";

export type StreamState = "connecting" | "live" | "offline";

const RETRY_MS = 5000;

/**
 * Follows /api/sse/reservations. `onOpen` runs on every (re)connection so the caller reloads what
 * it may have missed; the stream itself never replays.
 */
export function useReservationStream(
  enabled: boolean,
  onChange: (reservation: Reservation) => void,
  onOpen: () => void
): StreamState {
  const [state, setState] = useState<StreamState>("connecting");
  const onChangeRef = useRef(onChange);
  const onOpenRef = useRef(onOpen);

  useEffect(() => {
    onChangeRef.current = onChange;
    onOpenRef.current = onOpen;
  }, [onChange, onOpen]);

  useEffect(() => {
    if (!enabled) return;
    let source: EventSource | null = null;
    let retry: ReturnType<typeof setTimeout> | null = null;
    let stopped = false;

    const connect = () => {
      setState("connecting");
      source = new EventSource("/api/sse/reservations");
      source.onopen = () => {
        setState("live");
        onOpenRef.current();
      };
      source.addEventListener("reservation", (event) => {
        try {
          onChangeRef.current(JSON.parse((event as MessageEvent<string>).data) as Reservation);
        } catch (error) {
          console.error("Unreadable reservation event:", error);
        }
      });
      source.onerror = () => {
        // The browser retries a dropped stream by itself, but gives up on an error status.
        if (source?.readyState === EventSource.CLOSED && !stopped) {
          setState("offline");
          source.close();
          retry = setTimeout(connect, RETRY_MS);
        } else {
          setState("connecting");
        }
      };
    };

    connect();
    return () => {
      stopped = true;
      if (retry) clearTimeout(retry);
      source?.close();
    };
  }, [enabled]);

  return enabled ? state : "offline";
}
