export interface Stock {
  id: string;
  product_id: string;
  amount: number;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface HistoricStock {
  id: number;
  product_id: string;
  created_at: string;
  change: number;
}

export interface CreateStockRequest {
  product_id: string;
  amount: number;
}

export interface AddOrDecreaseStockRequest {
  product_id: string;
  change: number;
}

export interface BulkStockItem {
  product_id: string;
  amount: number;
}

export interface BulkStockCreationOrUpdatingRequest {
  items: BulkStockItem[];
}

export interface StockListResponse {
  stocks: Stock[];
  total?: number;
}
