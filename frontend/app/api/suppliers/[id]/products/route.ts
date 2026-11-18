import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  SupplierCatalogItem,
  AddProductToSupplierRequest,
  SupplierCatalogListResponse,
} from "@/types/supplierCatalog";

// GET /api/suppliers/:id/products - Get supplier's product catalog
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const response = await serverApiRequest<SupplierCatalogListResponse>(
    `/suppliers/${id}/products`
  );
  
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch supplier catalog" },
      { status: response.status }
    );
  }
  
  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/suppliers/:id/products - Add product to supplier's catalog
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: AddProductToSupplierRequest = await request.json();
    const response = await serverApiRequest<SupplierCatalogItem>(
      `/suppliers/${id}/products`,
      {
        method: "POST",
        body: JSON.stringify(body),
      }
    );
    
    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to add product to catalog" },
        { status: response.status }
      );
    }
    
    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error adding product to supplier catalog:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to add product to catalog",
      },
      { status: 500 }
    );
  }
}
