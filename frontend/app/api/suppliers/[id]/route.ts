import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { Supplier, UpdateSupplierRequest } from "@/types/supplier";

// GET /api/suppliers/:id - Get supplier by ID
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const response = await serverApiRequest<Supplier>(`/suppliers/${id}`);
  
  if (!response.ok) {
    return NextResponse.json(
      { error: "Supplier not found" },
      { status: response.status }
    );
  }
  
  const data = await response.json();
  return NextResponse.json(data);
}

// PUT /api/suppliers/:id - Update a supplier
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: UpdateSupplierRequest = await request.json();
    const response = await serverApiRequest<Supplier>(`/suppliers/${id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
    
    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to update supplier" },
        { status: response.status }
      );
    }
    
    const data = await response.json();
    return NextResponse.json(data);
  } catch (error) {
    console.error("Error updating supplier:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to update supplier",
      },
      { status: 500 }
    );
  }
}

// DELETE /api/suppliers/:id - Delete a supplier
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const response = await serverApiRequest<void>(`/suppliers/${id}`, {
      method: "DELETE",
    });
    
    if (!response.ok && response.status !== 204) {
      return NextResponse.json(
        { error: "Failed to delete supplier" },
        { status: response.status }
      );
    }
    
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Error deleting supplier:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to delete supplier",
      },
      { status: 500 }
    );
  }
}
