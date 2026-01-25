export interface Product {
  id: string;
  name: string;
  category: string;
  ico: string;
  version: number;
  sku: string;
  total_price_with_taxes: string;
  unit_price: string;
  description?: string;
  brand?: string | null;
  model?: string | null;
  vat: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface CreateProductRequest {
  name: string;
  category: string;
  vat: string;
  ico: string;
  taxes_format: string;
  description?: string | null;
  sku: string;
  total_price_with_taxes: string;
}

export interface UpdateProductRequest {
  name: string;
  category: string;
  vat: string;
  ico: string;
  taxes_format: string;
  description?: string | null;
  sku: string;
  total_price_with_taxes: string;
}

export interface ProductListResponse {
  products: Product[];
  total?: number;
}
