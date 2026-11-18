import { NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// GET /api/products/categories - Get all unique product categories
export async function GET() {
  const response = await serverApiRequest<string[]>("/products/categories");
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to list categories" },
      { status: response.status }
    );
  }
  const data = await response.json();
  return NextResponse.json(data);
}
