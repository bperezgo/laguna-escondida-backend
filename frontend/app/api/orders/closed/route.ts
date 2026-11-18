import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OpenBillListResponse } from "@/types/order";

// GET /api/orders/closed - Orders closed today (soft-deleted open bills)
export async function GET(_request: NextRequest) {
  return await serverApiRequest<OpenBillListResponse>("/orders/closed");
}
