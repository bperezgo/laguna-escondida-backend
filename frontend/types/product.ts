export interface Product {
  ID: string;
  Name: string;
  Category: string;
  Version: number;
  Price: number;
  VAT: number;
  CreatedAt: string;
  UpdatedAt: string;
  DeletedAt?: string | null;
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

