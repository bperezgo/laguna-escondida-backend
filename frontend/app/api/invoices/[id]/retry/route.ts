import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";

// POST /api/invoices/:id/retry - Manually re-submit a failed/pending invoice
export async function POST(
  _request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;

  return await serverApiRequest<void>(`/invoices/${id}/retry`, {
    method: "POST",
  });
}
