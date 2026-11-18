import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { AddIngredientRequest } from "@/types/product";

// GET /api/products/[id]/ingredients - list a composite product's recipe
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    // Pass the backend response through verbatim (status + body), so the client
    // sees the wrapped { ingredients: [...] } payload and any error message.
    return await serverApiRequest(`/products/${id}/ingredients`);
  } catch (error) {
    console.error("Error fetching ingredients:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch ingredients",
      },
      { status: 500 },
    );
  }
}

// POST /api/products/[id]/ingredients - add an ingredient to the recipe
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const body: AddIngredientRequest = await request.json();
    // Return the backend response as-is so validation errors (400 not-composite,
    // 400 self-ingredient, 409 duplicate, 400 quantity<=0, cycle) reach the client.
    return await serverApiRequest(`/products/${id}/ingredients`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error("Error adding ingredient:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to add ingredient",
      },
      { status: 500 },
    );
  }
}
