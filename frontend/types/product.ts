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
  Name: string;
  Category: string;
  Price: number;
  VAT: number;
}

export interface UpdateProductRequest {
  Name?: string;
  Category?: string;
  Price?: number;
  VAT?: number;
}

export interface ProductListResponse {
  products: Product[];
  total?: number;
}

