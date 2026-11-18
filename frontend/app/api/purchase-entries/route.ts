import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  PurchaseEntry,
  CreatePurchaseEntryRequest,
  PurchaseEntryListResponse,
} from "@/types/purchaseEntry";

// GET /api/purchase-entries - Get all purchase entries
export async function GET(request: NextRequest) {
  const { searchParams } = new URL(request.url);
  const supplierId = searchParams.get("supplier_id");
  const startDate = searchParams.get("start_date");
  const endDate = searchParams.get("end_date");

  const params = new URLSearchParams();
  if (supplierId) params.set("supplier_id", supplierId);
  if (startDate) params.set("start_date", startDate);
  if (endDate) params.set("end_date", endDate);

  const query = params.toString() ? `?${params.toString()}` : "";
  const response = await serverApiRequest<PurchaseEntryListResponse>(
    `/purchase-entries${query}`
  );
  
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch purchase entries" },
      { status: response.status }
    );
  }
  
  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/purchase-entries - Create a new purchase entry
export async function POST(request: NextRequest) {
  try {
    const body: CreatePurchaseEntryRequest = await request.json();
    const response = await serverApiRequest<PurchaseEntry>("/purchase-entries", {
      method: "POST",
      body: JSON.stringify(body),
    });
    
    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to create purchase entry" },
        { status: response.status }
      );
    }
    
    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error creating purchase entry:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create purchase entry",
      },
      { status: 500 }
    );
  }
}
