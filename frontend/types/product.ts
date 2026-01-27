export type ProductType = 'SELLABLE' | 'INGREDIENT' | 'COMPOSITE' | 'BOTH';

export type UnitOfMeasure = 'unit' | 'kg' | 'g' | 'l' | 'ml';

export const PRODUCT_TYPES: { value: ProductType; label: string }[] = [
  { value: 'SELLABLE', label: 'Vendible' },
  { value: 'INGREDIENT', label: 'Ingrediente' },
  { value: 'COMPOSITE', label: 'Compuesto' },
  { value: 'BOTH', label: 'Vendible e Ingrediente' },
];

export const UNITS_OF_MEASURE: { value: UnitOfMeasure; label: string }[] = [
  { value: 'unit', label: 'Unidades' },
  { value: 'kg', label: 'Kilogramos (kg)' },
  { value: 'g', label: 'Gramos (g)' },
  { value: 'l', label: 'Litros (l)' },
  { value: 'ml', label: 'Mililitros (ml)' },
];

export interface Product {
  id: string;
  name: string;
  category: string;
  product_type: ProductType;
  unit_of_measure: UnitOfMeasure;
  ico: string;
  ico_amount?: string;
  version: number;
  sku: string;
  total_price_with_taxes: string;
  unit_price: string;
  description?: string;
  brand?: string | null;
  model?: string | null;
  vat: string;
  vat_amount?: string;
  created_at: string;
  updated_at: string;
  deleted_at?: string | null;
}

export interface CreateProductRequest {
  name: string;
  category: string;
  product_type: ProductType;
  unit_of_measure: UnitOfMeasure;
  sku: string;
  description?: string | null;
  // Price fields - only required for SELLABLE, COMPOSITE, BOTH
  total_price_with_taxes?: string;
  vat?: string;
  ico?: string;
  taxes_format?: string;
}

export interface UpdateProductRequest {
  name: string;
  category: string;
  product_type: ProductType;
  unit_of_measure: UnitOfMeasure;
  sku: string;
  description?: string | null;
  // Price fields - only required for SELLABLE, COMPOSITE, BOTH
  total_price_with_taxes?: string;
  vat?: string;
  ico?: string;
  taxes_format?: string;
}

export interface ProductListResponse {
  products: Product[];
  total?: number;
}

// Helper function to check if a product type requires pricing
export function requiresPricing(productType: ProductType): boolean {
  return productType !== 'INGREDIENT';
}
