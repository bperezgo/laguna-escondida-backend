import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OpenBillWithProducts } from "@/types/order";

// GET /api/orders/closed/:id - A single closed order with products (for reprint)
export async function GET(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;

  return await serverApiRequest<OpenBillWithProducts>(`/orders/closed/${id}`);
}
