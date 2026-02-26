import { NextRequest, NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { getAccessToken } from "@/lib/auth";
import { cookies } from "next/headers";

// GET /api/purchase-entries/export - Export purchase entries as CSV
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const supplier_id = searchParams.get("supplier_id");
  const start_date = searchParams.get("start_date");
  const end_date = searchParams.get("end_date");

  const queryParams = new URLSearchParams();

  if (supplier_id) queryParams.set("supplier_id", supplier_id);
  if (start_date) queryParams.set("start_date", start_date);
  if (end_date) queryParams.set("end_date", end_date);

  const queryString = queryParams.toString();
  const endpoint = queryString
    ? `/purchase-entries/export?${queryString}`
    : "/purchase-entries/export";

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
        { error: "Failed to export purchase entries" },
        { status: response.status }
      );
    }

    const csvContent = await response.text();

    const contentDisposition = response.headers.get("Content-Disposition");
    let filename = `entradas_compra_${new Date().toISOString().split("T")[0]}.csv`;

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
    console.error("Error exporting purchase entries:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error
            ? error.message
            : "Failed to export purchase entries",
      },
      { status: 500 }
    );
  }
}
