import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  Supplier,
  CreateSupplierRequest,
  SupplierListResponse,
} from "@/types/supplier";

// GET /api/suppliers - Get all suppliers
export async function GET() {
  const response = await serverApiRequest<SupplierListResponse>("/suppliers");
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch suppliers" },
      { status: response.status }
    );
  }
  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/suppliers - Create a new supplier
export async function POST(request: NextRequest) {
  try {
    const body: CreateSupplierRequest = await request.json();
    const response = await serverApiRequest<Supplier>("/suppliers", {
      method: "POST",
      body: JSON.stringify(body),
    });
    
    if (!response.ok) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to create supplier" },
        { status: response.status }
      );
    }
    
    const data = await response.json();
    return NextResponse.json(data, { status: 201 });
  } catch (error) {
    console.error("Error creating supplier:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to create supplier",
      },
      { status: 500 }
    );
  }
}
