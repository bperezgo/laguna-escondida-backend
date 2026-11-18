import { useCallback, useEffect, useRef, useState } from "react";
import { deliveryZonesApi } from "@/lib/api/deliveryZones";
import type { DeliveryPin } from "@/lib/orders/orderTypePayload";

export type PinCheck =
  | { status: "idle" }
  | { status: "checking" }
  | { status: "inside"; zoneName: string }
  | { status: "outside" }
  | { status: "error" };

const DEBOUNCE_MS = 400;

/** Debounced zone check of the latest pin; only the newest answer is kept while dragging. */
export function useDeliveryPinCheck(): [PinCheck, (pin: DeliveryPin) => void] {
  const [check, setCheck] = useState<PinCheck>({ status: "idle" });
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const seqRef = useRef(0);

  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
      seqRef.current++;
    },
    [],
  );

  const schedule = useCallback((pin: DeliveryPin) => {
    setCheck({ status: "checking" });
    if (timerRef.current) clearTimeout(timerRef.current);
    const seq = ++seqRef.current;
    timerRef.current = setTimeout(async () => {
      try {
        const result = await deliveryZonesApi.check(pin.lat, pin.lng);
        if (seq !== seqRef.current) return;
        setCheck(
          result.deliverable
            ? { status: "inside", zoneName: result.zone_name ?? "zona" }
            : { status: "outside" },
        );
      } catch (error) {
        if (seq !== seqRef.current) return;
        console.error("Error checking delivery zone:", error);
        setCheck({ status: "error" });
      }
    }, DEBOUNCE_MS);
  }, []);

  return [check, schedule];
}
