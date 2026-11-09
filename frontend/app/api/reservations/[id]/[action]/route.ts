import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { Reservation, ReservationAction } from "@/types/reservation";

const ACTIONS: readonly ReservationAction[] = ["confirm", "decline", "cancel", "no-show"];

function isAction(value: string): value is ReservationAction {
  return (ACTIONS as readonly string[]).includes(value);
}

// POST /api/reservations/:id/{confirm|decline|cancel|no-show} - Move a reservation on, as staff
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string; action: string }> }
) {
  const { id, action } = await params;
  if (!isAction(action)) {
    return NextResponse.json({ error: "Unknown action" }, { status: 404 });
  }
  const body = await request.text();
  return serverApiRequest<Reservation>(
    `/reservations/${encodeURIComponent(id)}/${action}`,
    { method: "POST", body: body || undefined }
  );
}
