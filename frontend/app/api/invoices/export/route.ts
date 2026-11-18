import { NextRequest, NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { cookies } from "next/headers";

// GET /api/invoices/export - Export invoices as CSV
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const created_at_start = searchParams.get("created_at_start");
  const created_at_end = searchParams.get("created_at_end");
  const national_identification = searchParams.get("national_identification");

  const queryParams = new URLSearchParams();

  if (created_at_start) {
    queryParams.set("created_at_start", created_at_start);
  }
  if (created_at_end) {
    queryParams.set("created_at_end", created_at_end);
  }
  if (national_identification) {
    queryParams.set("national_identification", national_identification);
  }

  const queryString = queryParams.toString();
  const endpoint = queryString
    ? `/invoices/export?${queryString}`
    : "/invoices/export";

  const url = `${config.apiUrl}${endpoint}`;

  // Get the JWT token from cookies
  const token = await getAccessToken();

  const headers: Record<string, string> = {};

  // Add Authorization header if token exists
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(url, {
      method: "GET",
      headers,
    });

    // Handle unauthorized responses by clearing the invalid token
    if (response.status === 401) {
      const cookieStore = await cookies();
      cookieStore.delete("access_token");
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    if (!response.ok) {
      return NextResponse.json(
        { error: "Failed to export invoices" },
        { status: response.status }
      );
    }

    // Get the CSV content as text
    const csvContent = await response.text();

    // Get the filename from the backend response or generate one
    const contentDisposition = response.headers.get("Content-Disposition");
    let filename = `facturas_${new Date().toISOString().split("T")[0]}.csv`;

    if (contentDisposition) {
      const match = contentDisposition.match(/filename=([^;]+)/);
      if (match) {
        filename = match[1].replace(/"/g, "");
      }
    }

    // Return the CSV with proper headers
    return new NextResponse(csvContent, {
      status: 200,
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": `attachment; filename=${filename}`,
      },
    });
  } catch (error) {
    console.error("Error exporting invoices:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to export invoices",
      },
      { status: 500 }
    );
  }
}
