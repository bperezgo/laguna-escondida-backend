import { NextRequest, NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { cookies } from "next/headers";

// GET /api/support-documents/export - Export support documents as CSV
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const created_at_start = searchParams.get("created_at_start");
  const created_at_end = searchParams.get("created_at_end");
  const provider_document_number = searchParams.get(
    "provider_document_number"
  );

  const queryParams = new URLSearchParams();

  if (created_at_start) {
    queryParams.set("created_at_start", created_at_start);
  }
  if (created_at_end) {
    queryParams.set("created_at_end", created_at_end);
  }
  if (provider_document_number) {
    queryParams.set("provider_document_number", provider_document_number);
  }

  const queryString = queryParams.toString();
  const endpoint = queryString
    ? `/support-documents/export?${queryString}`
    : "/support-documents/export";

  const url = `${config.apiUrl}${endpoint}`;

  const token = await getAccessToken();

  const headers: Record<string, string> = {};

  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(url, {
      method: "GET",
      headers,
    });

    if (response.status === 401) {
      const cookieStore = await cookies();
      cookieStore.delete("access_token");
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    if (!response.ok) {
      return NextResponse.json(
        { error: "Failed to export support documents" },
        { status: response.status }
      );
    }

    const csvContent = await response.text();

    const contentDisposition = response.headers.get("Content-Disposition");
    let filename = `documentos_soporte_${new Date().toISOString().split("T")[0]}.csv`;

    if (contentDisposition) {
      const match = contentDisposition.match(/filename=([^;]+)/);
      if (match) {
        filename = match[1].replace(/"/g, "");
      }
    }

    return new NextResponse(csvContent, {
      status: 200,
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename=${filename}`,
      },
    });
  } catch (error) {
    console.error("Error exporting support documents:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to export support documents",
      },
      { status: 500 }
    );
  }
}
