import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { BillOwnerResponse } from "@/types/billOwner";

// GET /api/bill-owners/:id - Get a specific bill owner
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;

  const response = await serverApiRequest<BillOwnerResponse>(
    `/bill-owners/${id}`
  );
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch bill owner" },
      { status: response.status }
    );
  }
  const data = await response.json();
  return NextResponse.json(data);
}
