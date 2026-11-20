import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { DeliveryOrderListResponse } from "@/types/delivery";

const FORWARDED_QUERY = ["status", "order_type"];

// GET /api/deliveries/orders?status=&order_type= - The delivery dashboard (cloud only)
export async function GET(request: NextRequest) {
  const query = new URLSearchParams();
  for (const key of FORWARDED_QUERY) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) query.set(key, value);
  }
  const suffix = query.size > 0 ? `?${query}` : "";
  return serverApiRequest<DeliveryOrderListResponse>(`/deliveries/orders${suffix}`);
}
