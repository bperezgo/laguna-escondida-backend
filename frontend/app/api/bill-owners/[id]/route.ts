import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { BillOwnerResponse } from "@/types/billOwner";

// GET /api/bill-owners/:id - Get a specific bill owner
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;

    const response = await serverApiRequest<BillOwnerResponse>(
      `/bill-owners/${id}`
    );
    return NextResponse.json(response);
  } catch (error) {
    console.error("Error fetching bill owner:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch bill owner",
      },
      { status: 500 }
    );
  }
}
