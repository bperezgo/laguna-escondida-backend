import { serverApiRequest } from "@/lib/api/server";
import type { CourierListResponse } from "@/types/delivery";

// GET /api/deliveries/couriers - Active couriers to assign an order to
export async function GET() {
  return serverApiRequest<CourierListResponse>("/deliveries/couriers");
}
