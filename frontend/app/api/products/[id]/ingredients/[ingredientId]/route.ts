import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { UpdateIngredientRequest } from "@/types/product";

// PUT /api/products/[id]/ingredients/[ingredientId] - change an ingredient's quantity
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; ingredientId: string }> },
) {
  try {
    const { id, ingredientId } = await params;
    const body: UpdateIngredientRequest = await request.json();
    return await serverApiRequest(
      `/products/${id}/ingredients/${ingredientId}`,
      {
        method: "PUT",
        body: JSON.stringify(body),
      },
    );
  } catch (error) {
    console.error("Error updating ingredient:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to update ingredient",
      },
      { status: 500 },
    );
  }
}

// DELETE /api/products/[id]/ingredients/[ingredientId] - remove an ingredient
export async function DELETE(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; ingredientId: string }> },
) {
  try {
    const { id, ingredientId } = await params;
    // Backend returns 204 No Content; serverApiRequest forwards it as an empty response.
    return await serverApiRequest(
      `/products/${id}/ingredients/${ingredientId}`,
      {
        method: "DELETE",
      },
    );
  } catch (error) {
    console.error("Error removing ingredient:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to remove ingredient",
      },
      { status: 500 },
    );
  }
}
