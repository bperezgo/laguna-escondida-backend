import { serverApiRequest } from '@/lib/api/server';
import { transformStockListResponse, transformProductListResponse } from '@/lib/api/transform';
import type { Stock, StockListResponse } from '@/types/stock';
import type { Product, ProductListResponse } from '@/types/product';
import StockPageClient from '@/components/stock/StockPageClient';

export default async function StockPage() {
  // Fetch stocks and products server-side
  let stocks: Stock[] = [];
  let products: Product[] = [];
  
  try {
    const stockResponse = await serverApiRequest<any>('/stock');
    const transformed = transformStockListResponse(stockResponse);
    stocks = transformed.stocks || [];
  } catch (error) {
    console.error('Error fetching stocks:', error);
    // Stocks will be empty array, error will be handled by client component
  }

  try {
    // Fetch products to show in the product selector
    const productResponse = await serverApiRequest<any>('/products');
    const transformed = transformProductListResponse(productResponse);
    products = transformed.products || [];
  } catch (error) {
    console.error('Error fetching products:', error);
    // Products will be empty array
  }

  return <StockPageClient initialStocks={stocks} products={products} />;
}

