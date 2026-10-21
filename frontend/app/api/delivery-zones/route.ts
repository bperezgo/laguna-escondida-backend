import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  CreateDeliveryZoneRequest,
  DeliveryZone,
  DeliveryZoneListResponse,
} from "@/types/deliveryZone";

// GET /api/delivery-zones - List zones (active and inactive)
export async function GET() {
  const response =
    await serverApiRequest<DeliveryZoneListResponse>("/delivery-zones");
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch delivery zones" },
      { status: response.status }
    );
  }
  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/delivery-zones - Create a zone
export async function POST(request: NextRequest) {
  try {
    const body: CreateDeliveryZoneRequest = await request.json();
    const response = await serverApiRequest<DeliveryZone>("/delivery-zones", {
      method: "POST",
      body: JSON.stringify(body),
    });

    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        {
          error: errorData.error || "Failed to create delivery zone",
          code: errorData.code,
        },
        { status: response.status }
      );
    }

    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error creating delivery zone:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create delivery zone",
      },
      { status: 500 }
    );
  }
}
