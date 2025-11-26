import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OpenBillWithProducts } from "@/types/order";

// GET /api/orders/:id - Get a specific order with products
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    const response = await serverApiRequest<OpenBillWithProducts>(
      `/orders/${id}`
    );
    return NextResponse.json(response);
  } catch (error) {
    console.error("Error fetching order:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to fetch order",
      },
      { status: 500 }
    );
  }
}
