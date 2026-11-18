import { serverApiRequest } from "@/lib/api/server";
import type { Product, ProductListResponse } from "@/types/product";
import ProductsPageClient from "@/components/products/ProductsPageClient";

export default async function ProductsPage() {
  // Fetch products server-side
  let products: Product[] = [];

  try {
    const response = await serverApiRequest<ProductListResponse>("/products");
    const data = await response.json();
    products = data.products || [];
  } catch (error) {
    console.error("Error fetching products:", error);
    // Products will be empty array, error will be handled by client component
  }

  return <ProductsPageClient initialProducts={products} />;
}
