import { NextRequest } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type {
  CreateStaffReservationRequest,
  Reservation,
  ReservationListResponse,
} from "@/types/reservation";

const FORWARDED_QUERY = ["date", "status", "phone", "from"];

// GET /api/reservations?date=YYYY-MM-DD - A day's reservations in the organization's timezone
export async function GET(request: NextRequest) {
  const query = new URLSearchParams();
  for (const key of FORWARDED_QUERY) {
    const value = request.nextUrl.searchParams.get(key);
    if (value) query.set(key, value);
  }
  const suffix = query.size > 0 ? `?${query}` : "";
  return serverApiRequest<ReservationListResponse>(`/reservations${suffix}`);
}

// POST /api/reservations - A staff booking (phone or other), stored as confirmed
export async function POST(request: NextRequest) {
  const body: CreateStaffReservationRequest = await request.json();
  return serverApiRequest<Reservation>("/reservations", {
    method: "POST",
    body: JSON.stringify(body),
  });
}
