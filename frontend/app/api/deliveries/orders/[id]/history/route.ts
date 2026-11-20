import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { DeliveryHistory } from "@/types/delivery";

// GET /api/deliveries/orders/:id/history - An order's delivery changes, oldest first
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  return serverApiRequest<DeliveryHistory>(`/deliveries/orders/${encodeURIComponent(id)}/history`);
}
