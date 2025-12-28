export interface CommandItemFromSSE {
  open_bill_product_id: string;
  open_bill_id: string;
  product_name: string;
  quantity: number;
  notes?: string | null;
  temporal_identifier: string;
  priority: number;
  created_at: string;
  name: string; // user name who created the order
}
