import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  PurchaseEntry,
  CreatePurchaseEntryRequest,
  PurchaseEntryListResponse,
} from "@/types/purchaseEntry";

// GET /api/purchase-entries - Get all purchase entries
export async function GET() {
  const response = await serverApiRequest<PurchaseEntryListResponse>(
    "/purchase-entries"
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
