import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { DailyCloseReport } from "@/types/dailyClose";

// GET /api/reports/daily-close - End-of-day money reconciliation for one business day.
// Proxies to the Go backend; ?date=YYYY-MM-DD selects the day (backend defaults to today).
export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const params = new URLSearchParams();

  const date = searchParams.get("date");
  if (date) params.append("date", date);

  const queryString = params.toString();
  const endpoint = `/reports/daily-close${queryString ? `?${queryString}` : ""}`;

  const response = await serverApiRequest<DailyCloseReport>(endpoint);

  if (!response.ok) {
    return NextResponse.json(
      { error: "Failed to fetch daily close" },
      { status: response.status }
    );
  }

  const data = await response.json();
  return NextResponse.json(data);
}
