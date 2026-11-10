import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { ReplacePlateOptionSlotsRequest } from "@/types/product";

// GET /api/products/[id]/option-slots - list a composite plate's option slots
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    return await serverApiRequest(`/products/${id}/option-slots`);
  } catch (error) {
    console.error("Error fetching option slots:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch option slots",
      },
      { status: 500 },
    );
  }
}

// PUT /api/products/[id]/option-slots - replace the plate's whole slot list (cloud only)
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const body: ReplacePlateOptionSlotsRequest = await request.json();
    // Passed through as-is so 400/404/409 messages naming the slot reach the editor.
    return await serverApiRequest(`/products/${id}/option-slots`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error("Error replacing option slots:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to save option slots",
      },
      { status: 500 },
    );
  }
}
