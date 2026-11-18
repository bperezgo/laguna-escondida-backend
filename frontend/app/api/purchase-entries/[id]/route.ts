import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { PurchaseEntry } from "@/types/purchaseEntry";

// GET /api/purchase-entries/:id - Get purchase entry by ID (includes items)
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  const response = await serverApiRequest<PurchaseEntry>(
    `/purchase-entries/${id}`
  );
  
  if (!response.ok) {
    return NextResponse.json(
      { error: "Purchase entry not found" },
      { status: response.status }
    );
  }
  
  const data = await response.json();
  return NextResponse.json(data);
}
