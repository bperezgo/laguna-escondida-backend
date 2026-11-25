import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { OpenBillWithProducts } from "@/types/order";

type Params = { params: Promise<{ id: string }> };

// PUT /api/orders/:id/payment-review - Staff start checking a manual order's payment
export async function PUT(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  return await serverApiRequest<OpenBillWithProducts>(`/orders/${encodeURIComponent(id)}/payment-review`, { method: "PUT" });
}

// DELETE /api/orders/:id/payment-review - Stop checking it
export async function DELETE(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  return await serverApiRequest<OpenBillWithProducts>(`/orders/${encodeURIComponent(id)}/payment-review`, { method: "DELETE" });
}
