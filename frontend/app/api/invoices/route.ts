import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  ElectronicInvoice,
  CreateElectronicInvoiceRequest,
} from "@/types/invoice";

// GET /api/invoices - Get all invoices with optional filters
export async function GET(request: NextRequest) {
  try {
    const searchParams = request.nextUrl.searchParams;
    const page = searchParams.get("page") || "1";
    const page_size = searchParams.get("page_size") || "20";
    const created_at_start = searchParams.get("created_at_start");
    const created_at_end = searchParams.get("created_at_end");
    const national_identification = searchParams.get("national_identification");

    // Build query string
    const queryParams = new URLSearchParams({
      page,
      page_size,
    });

    if (created_at_start) {
      queryParams.set("created_at_start", created_at_start);
    }
    if (created_at_end) {
      queryParams.set("created_at_end", created_at_end);
    }
    if (national_identification) {
      queryParams.set("national_identification", national_identification);
    }

    return await serverApiRequest<any>(`/invoices?${queryParams.toString()}`);
  } catch (error) {
    console.error("Error fetching invoices:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to fetch invoices",
      },
      { status: 500 }
    );
  }
}

// POST /api/invoices - Create a new electronic invoice
export async function POST(request: NextRequest) {
  try {
    const body: CreateElectronicInvoiceRequest = await request.json();
    return await serverApiRequest<void>("/invoices", {
      method: "POST",
      body: JSON.stringify(body),
    });
  } catch (error) {
    console.error("Error creating invoice:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to create invoice",
      },
      { status: 500 }
    );
  }
}
