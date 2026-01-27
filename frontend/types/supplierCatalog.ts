export interface SupplierCatalogItem {
  id: string;
  supplier_id: string;
  supplier_name?: string;
  product_id: string;
  product_name?: string;
  unit_cost: string;
  supplier_sku?: string | null;
  created_at: string;
  updated_at: string;
}

export interface AddProductToSupplierRequest {
  product_id: string;
  unit_cost: string;
  supplier_sku?: string | null;
}

export interface UpdateSupplierCatalogRequest {
  unit_cost: string;
  supplier_sku?: string | null;
}

export interface SupplierCatalogListResponse {
  items: SupplierCatalogItem[];
  total: number;
}
