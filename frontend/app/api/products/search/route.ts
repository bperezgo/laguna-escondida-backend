import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { ProductSearchResponse } from "@/types/order";

// GET /api/products/search - Search products
export async function GET(request: NextRequest) {
  try {
    const searchParams = request.nextUrl.searchParams;
    const query = searchParams.get("q");

    if (!query) {
      return NextResponse.json(
        { error: 'Query parameter "q" is required' },
        { status: 400 }
      );
    }

    const response = await serverApiRequest<ProductSearchResponse>(
      `/products/search?q=${encodeURIComponent(query)}`
    );
    return response;
  } catch (error) {
    console.error("Error searching products:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to search products",
      },
      { status: 500 }
    );
  }
}
