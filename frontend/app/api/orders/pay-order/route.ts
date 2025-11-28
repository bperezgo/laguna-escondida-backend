import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// POST /api/orders/pay-order - Pay and close an order
export async function POST(request: NextRequest) {
  try {
    const body = await request.json();
    const { order_id } = body;

    if (!order_id) {
      return NextResponse.json(
        { error: "order_id is required" },
        { status: 400 }
      );
    }

    const response = await serverApiRequest(`/orders/pay-order`, {
      method: "POST",
      body: JSON.stringify({ order_id }),
    });

    return NextResponse.json(response);
  } catch (error) {
    console.error("Error paying order:", error);
    return NextResponse.json(
      {
        error: error instanceof Error ? error.message : "Failed to pay order",
      },
      { status: 500 }
    );
  }
}
