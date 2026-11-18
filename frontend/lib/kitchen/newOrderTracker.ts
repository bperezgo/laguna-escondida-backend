import type { OpenBillProductFromSSE, OrderType } from "@/types/commandItem";

type TrackedLine = Pick<
  OpenBillProductFromSSE,
  "open_bill_id" | "order_type" | "temporal_identifier"
>;

export interface NewOrderAlert {
  open_bill_id: string;
  order_type: Extract<OrderType, "takeaway" | "delivery">;
  temporal_identifier: string;
}

export interface NewOrderTracker {
  seed(lines: TrackedLine[]): void;
  /** The order to alert for this `created` line, or null. */
  observe(line: TrackedLine): NewOrderAlert | null;
}

// The SSE stream replays every pending line as `created` on each connect, so an
// order is "new" only if this page has never seen its open_bill_id.
export function createNewOrderTracker(): NewOrderTracker {
  const seen = new Set<string>();

  return {
    seed(lines) {
      for (const line of lines) seen.add(line.open_bill_id);
    },
    observe(line) {
      if (seen.has(line.open_bill_id)) return null;
      seen.add(line.open_bill_id);
      if (line.order_type !== "takeaway" && line.order_type !== "delivery") {
        return null;
      }
      return {
        open_bill_id: line.open_bill_id,
        order_type: line.order_type,
        temporal_identifier: line.temporal_identifier,
      };
    },
  };
}
