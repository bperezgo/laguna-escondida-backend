export type EdgeMode = "edge" | "cloud";

/**
 * Connectivity & sync state reported by the edge node's `GET /api/edge/status`.
 *
 * ⚠ Assumed contract — confirm shape with the backend/edge team. The fields are
 * read defensively across the UI so they're easy to adjust.
 */
export interface EdgeStatus {
  /** Whether the app is served from the local edge node or the cloud. */
  mode: EdgeMode;
  /** Whether the node currently has internet/cloud connectivity. */
  online: boolean;
  /** How far behind the cloud the local node is, in seconds. */
  sync_lag_seconds: number;
  /** Electronic invoices queued for submission to the cloud. */
  pending_invoices: number;
  /** Orders created locally that have not yet synced to the cloud. */
  unsynced_orders: number;
}
