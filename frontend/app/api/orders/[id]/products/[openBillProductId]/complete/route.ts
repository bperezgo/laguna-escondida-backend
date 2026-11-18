import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// PATCH /api/orders/:id/products/:openBillProductId/complete
export async function PATCH(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string; openBillProductId: string }> }
) {
  const { id, openBillProductId } = await params;

  return await serverApiRequest<void>(
    `/orders/${id}/products/${openBillProductId}/complete`,
    {
      method: "PATCH",
    }
  );
}
