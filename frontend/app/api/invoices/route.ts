import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  ElectronicInvoice,
  CreateElectronicInvoiceRequest,
} from "@/types/invoice";

// POST /api/invoices - Create a new electronic invoice
export async function POST(request: NextRequest) {
  try {
    const body: CreateElectronicInvoiceRequest = await request.json();
    await serverApiRequest<ElectronicInvoice>("/invoices", {
      method: "POST",
      body: JSON.stringify(body),
    });
    return NextResponse.json({ message: "Invoice created successfully" });
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
