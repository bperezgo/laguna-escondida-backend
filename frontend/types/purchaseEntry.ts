export interface PurchaseEntryItem {
  id: string;
  purchase_entry_id: string;
  product_id: string;
  product_name?: string;
  quantity: string;
  unit_cost: string;
  total_cost: string;
}

export interface PurchaseEntry {
  id: string;
  supplier_id: string;
  supplier_name?: string;
  total_amount: string;
  invoice_reference?: string | null;
  entry_date: string;
  notes?: string | null;
  items?: PurchaseEntryItem[];
  pdf_storage_path?: string | null;
  xml_storage_path?: string | null;
  created_at: string;
}

export interface CreatePurchaseEntryItemRequest {
  product_id: string;
  quantity: string;
  unit_cost: string;
}

export interface CreatePurchaseEntryRequest {
  supplier_id: string;
  invoice_reference?: string | null;
  entry_date?: string | null;
  notes?: string | null;
  items: CreatePurchaseEntryItemRequest[];
}

export interface PurchaseEntryListResponse {
  entries: PurchaseEntry[];
  total: number;
}
