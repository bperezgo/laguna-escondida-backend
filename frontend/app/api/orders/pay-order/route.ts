import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { PayOrderRequest } from "@/types/billOwner";

// POST /api/orders/pay-order - Pay and close an order
export async function POST(request: NextRequest) {
  try {
    const body: PayOrderRequest = await request.json();
    const { order_id, payment_type } = body;

    if (!order_id) {
      return NextResponse.json(
        { error: "order_id is required" },
        { status: 400 }
      );
    }

    if (!payment_type) {
      return NextResponse.json(
        { error: "payment_type is required" },
        { status: 400 }
      );
    }

    const response = await serverApiRequest(`/orders/pay-order`, {
      method: "POST",
      body: JSON.stringify(body),
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
