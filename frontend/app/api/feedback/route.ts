import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { FeedbackListResponse } from "@/types/feedback";

const FORWARDED_QUERY = ["from", "to", "needs_attention"];

// GET /api/feedback?from=YYYY-MM-DD&to=YYYY-MM-DD - Guests' answers, newest first
export async function GET(request: NextRequest) {
  const query = new URLSearchParams();
  for (const key of FORWARDED_QUERY) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) query.set(key, value);
  }
  const suffix = query.size > 0 ? `?${query}` : "";
  return serverApiRequest<FeedbackListResponse>(`/feedback${suffix}`);
}
