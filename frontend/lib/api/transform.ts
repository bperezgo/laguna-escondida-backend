import type { Product, ProductListResponse } from '@/types/product';
import type { Stock, StockListResponse } from '@/types/stock';

/**
 * Transform backend response (PascalCase) to frontend format (camelCase)
 */
export function transformProduct(product: any): Product {
  return {
    id: product.id,
    name: product.name,
    category: product.category,
    version: product.version,
    unit_price: product.unit_price,
    vat: product.vat,
    created_at: product.created_at,
    updated_at: product.updated_at,
    deleted_at: product.deleted_at,
    sku: product.sku,
    total_price_with_taxes: product.total_price_with_taxes,
  };
}

export function transformProductListResponse(response: any): ProductListResponse {
  return {
    products: (response.products || []).map(transformProduct),
    total: response.total,
  };
}

export function transformStock(stock: any): Stock {
  return {
    id: stock.id,
    product_id: stock.product_id,
    amount: stock.amount,
    created_at: stock.created_at,
    updated_at: stock.updated_at,
    deleted_at: stock.deleted_at,
  };
}

export function transformStockListResponse(response: any): StockListResponse {
  return {
    stocks: (response.stocks || []).map(transformStock),
    total: response.total,
  };
}

