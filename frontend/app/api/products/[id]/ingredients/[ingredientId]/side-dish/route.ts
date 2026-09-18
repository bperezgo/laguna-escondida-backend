import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { ConfigureSideDishRequest } from "@/types/product";

// PUT /api/products/[id]/ingredients/[ingredientId]/side-dish
// Mark an ingredient as a side dish (or update its default/min/max bounds).
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; ingredientId: string }> },
) {
  try {
    const { id, ingredientId } = await params;
    const body: ConfigureSideDishRequest = await request.json();
    // Return the backend response as-is so validation errors (400 invalid bounds,
    // 400 not-composite, etc.) reach the client.
    return await serverApiRequest(
      `/products/${id}/ingredients/${ingredientId}/side-dish`,
      {
        method: "PUT",
        body: JSON.stringify(body),
      },
    );
  } catch (error) {
    console.error("Error configuring side dish:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to configure side dish",
      },
      { status: 500 },
    );
  }
}

// DELETE /api/products/[id]/ingredients/[ingredientId]/side-dish
// Revert an ingredient back to a fixed (non-configurable) recipe row.
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; ingredientId: string }> },
) {
  try {
    const { id, ingredientId } = await params;
    return await serverApiRequest(
      `/products/${id}/ingredients/${ingredientId}/side-dish`,
      {
        method: "DELETE",
      },
    );
  } catch (error) {
    console.error("Error clearing side dish:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to clear side dish",
      },
      { status: 500 },
    );
  }
}
