import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  SupplierCatalogItem,
  UpdateSupplierCatalogRequest,
} from "@/types/supplierCatalog";

// PUT /api/suppliers/:id/products/:productId - Update product pricing in catalog
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; productId: string }> }
) {
  try {
    const { id, productId } = await params;
    const body: UpdateSupplierCatalogRequest = await request.json();
    const response = await serverApiRequest<SupplierCatalogItem>(
      `/suppliers/${id}/products/${productId}`,
      {
        method: "PUT",
        body: JSON.stringify(body),
      }
    );
    
    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to update catalog item" },
        { status: response.status }
      );
    }
    
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error updating supplier catalog item:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to update catalog item",
      },
      { status: 500 }
    );
  }
}

// DELETE /api/suppliers/:id/products/:productId - Remove product from catalog
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; productId: string }> }
) {
  try {
    const { id, productId } = await params;
    const response = await serverApiRequest<void>(
      `/suppliers/${id}/products/${productId}`,
      {
        method: "DELETE",
      }
    );
    
    if (!response.ok && response.status !== 204) {
      return NextResponse.json(
        { error: "Failed to remove product from catalog" },
        { status: response.status }
      );
    }
    
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Error removing product from supplier catalog:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to remove product from catalog",
      },
      { status: 500 }
    );
  }
}
