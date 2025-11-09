export interface Product {
  id: string;
  name: string;
  category: string;
  version: number;
  sku: string;
  total_price_with_taxes: number;
  unit_price: number;
  description?: string;
  vat: number;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface CreateProductRequest {
  name: string;
  category: string;
  vat: number;
  ico: number;
  description?: string | null;
  brand?: string | null;
  model?: string | null;
  sku: string;
  total_price_with_taxes: number;
}

export interface UpdateProductRequest {
  name: string;
  category?: string;
  vat?: number;
  ico?: number;
  description?: string | null;
  brand?: string | null;
  model?: string | null;
  sku: string;
  total_price_with_taxes: number;
}

export interface ProductListResponse {
  products: Product[];
  total?: number;
}

