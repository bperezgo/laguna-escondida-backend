export type OpenBillProductStatus =
  | "created"
  | "completed"
  | "cancelled"
  | "in_progress";

// A resolved side dish on a kitchen line — the backend resolves the name (BSP-33).
export interface SideDishSSE {
  name: string;
  quantity: number;
}

export interface OpenBillProductFromSSE {
  open_bill_product_id: string;
  open_bill_id: string;
  product_name: string;
  quantity: number;
  notes?: string | null;
  area: string;
  status: OpenBillProductStatus;
  temporal_identifier: string;
  priority: number;
  created_at: string;
  created_by_name: string;
  // Only present on the completed/ready feed (GET .../completed).
  completed_at?: string | null;
  // Resolved side-dish selections for this line, with names pre-resolved by the backend.
  side_dishes?: SideDishSSE[];
}

/**
 * @deprecated Use OpenBillProductFromSSE instead
 */
export type CommandItemFromSSE = OpenBillProductFromSSE;
