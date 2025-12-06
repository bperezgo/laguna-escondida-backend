import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  CreateOrderRequest,
  CreateOrderResponse,
  OrderListResponse,
  OpenBillListResponse,
} from "@/types/order";

// GET /api/orders - Get all open bills/orders
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const tableId = searchParams.get("tableId");

  const endpoint = tableId ? `/orders?tableId=${tableId}` : "/orders";

  return await serverApiRequest<OpenBillListResponse | OrderListResponse>(
    endpoint
  );
}

// POST /api/orders - Create a new order
export async function POST(request: NextRequest) {
  try {
    const body: CreateOrderRequest = await request.json();

    // Validate required fields
    if (!body.temporal_identifier) {
      return NextResponse.json(
        { error: "temporal_identifier is required" },
        { status: 400 }
      );
    }

    if (!body.products || body.products.length === 0) {
      return NextResponse.json(
        { error: "At least one product is required" },
        { status: 400 }
      );
    }

    return await serverApiRequest<CreateOrderResponse>("/orders", {
      method: "POST",
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error("Error creating order:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to create order",
      },
      { status: 500 }
    );
  }
}
