import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OpenBillWithProducts, UpdateOrderRequest } from "@/types/order";

// GET /api/orders/:id - Get a specific order with products
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;

  return await serverApiRequest<OpenBillWithProducts>(`/orders/${id}`);
}

// PUT /api/orders/:id - Update an order
export async function PUT(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: UpdateOrderRequest = await request.json();

    return await serverApiRequest<OpenBillWithProducts>(`/orders/${id}`, {
      method: "PUT",
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error("Error updating order:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to update order",
      },
      { status: 500 }
    );
  }
}

// DELETE /api/orders/:id - Remove an open bill
export async function DELETE(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    return await serverApiRequest<void>(`/orders/${id}`, {
      method: "DELETE",
    });
  } catch (error) {
    console.error("Error removing order:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to remove order",
      },
      { status: 500 }
    );
  }
}
