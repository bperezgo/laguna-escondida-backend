import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { FinancialSummary } from "@/types/financialSummary";

// GET /api/financial/summary - Get financial summary for a date range
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const params = new URLSearchParams();

  const startDate = searchParams.get("start_date");
  const endDate = searchParams.get("end_date");

  if (startDate) params.append("start_date", startDate);
  if (endDate) params.append("end_date", endDate);

  const queryString = params.toString();
  const endpoint = `/financial/summary${queryString ? `?${queryString}` : ""}`;

  const response = await serverApiRequest<FinancialSummary>(endpoint);

  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch financial summary" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}
