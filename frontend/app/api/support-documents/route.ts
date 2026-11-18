import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { CreateSupportDocumentRequest } from "@/types/support-document";

// GET /api/support-documents - Get all support documents with optional filters
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const page = searchParams.get("page") || "1";
  const page_size = searchParams.get("page_size") || "20";
  const created_at_start = searchParams.get("created_at_start");
  const created_at_end = searchParams.get("created_at_end");
  const provider_document_number = searchParams.get(
    "provider_document_number"
  );

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
  if (provider_document_number) {
    queryParams.set("provider_document_number", provider_document_number);
  }

  const response = await serverApiRequest<any>(
    `/support-documents?${queryParams.toString()}`
  );
  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch support documents" },
      { status: response.status }
    );
  }
  const data = await response.json();
  return NextResponse.json(data);
}

// POST /api/support-documents - Create a new support document
export async function POST(request: NextRequest) {
  try {
    const body: CreateSupportDocumentRequest = await request.json();
    const response = await serverApiRequest<void>("/support-documents", {
      method: "POST",
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      return NextResponse.json(
        { error: "Failed to create support document" },
        { status: response.status }
      );
    }
    return response;
  } catch (error) {
    console.error("Error creating support document:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to create support document",
      },
      { status: 500 }
    );
  }
}
