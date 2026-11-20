import { serverApiRequest } from "@/lib/api/server";
import type { CourierQueue } from "@/types/delivery";

// GET /api/deliveries/mine - What the signed-in courier can claim and carries
export async function GET() {
  return serverApiRequest<CourierQueue>("/deliveries/mine");
}
