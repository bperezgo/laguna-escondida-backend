import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import {
  transformProduct,
  transformProductListResponse,
} from "@/lib/api/transform";
import type {
  Product,
  CreateProductRequest,
  UpdateProductRequest,
  ProductListResponse,
} from "@/types/product";

// GET /api/products - Get all products
export async function GET() {
  const response = await serverApiRequest<ProductListResponse>("/products");
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch products" },
      { status: response.status }
    );
  }
  const data = await response.json();
  const transformed = transformProductListResponse(data);
  return NextResponse.json(transformed);
}

// POST /api/products - Create a new product
export async function POST(request: NextRequest) {
  try {
    const body: CreateProductRequest = await request.json();
    const product = await serverApiRequest<any>("/products", {
      method: "POST",
      body: JSON.stringify(body),
    });
    const transformed = transformProduct(product);
    return NextResponse.json(transformed);
  } catch (error) {
    console.error("Error creating product:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to create product",
      },
      { status: 500 }
    );
  }
}
