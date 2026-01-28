export type OpenBillProductStatus =
  | "created"
  | "completed"
  | "cancelled"
  | "in_progress";

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
}

/**
 * @deprecated Use OpenBillProductFromSSE instead
 */
export type CommandItemFromSSE = OpenBillProductFromSSE;
