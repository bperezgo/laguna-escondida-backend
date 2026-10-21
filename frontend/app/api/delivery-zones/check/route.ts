import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { DeliveryPointCheck } from "@/types/deliveryZone";

// GET /api/delivery-zones/check?lat=&lng= - Is a point deliverable, and where?
export async function GET(request: NextRequest) {
  const lat = request.nextUrl.searchParams.get("lat") ?? "";
  const lng = request.nextUrl.searchParams.get("lng") ?? "";
  const query = new URLSearchParams({ lat, lng });

  const response = await serverApiRequest<DeliveryPointCheck>(
    `/delivery-zones/check?${query}`
  );
  if (!response.ok) {
    const errorData = await response.json();
    return NextResponse.json(
      { error: errorData.error || "Failed to check delivery point" },
      { status: response.status }
    );
  }
  const data = await response.json();
  return NextResponse.json(data);
}
