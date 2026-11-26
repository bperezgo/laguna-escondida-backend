import { NextResponse } from "next/server";

// Passes the backend's error, code, field and destination status through. A channel body holds
// its webhook, so these routes never log a request body nor an error raised while reading it.
export async function notificationErrorResponse(response: Response, fallback: string) {
  const errorData = await response.json().catch(() => ({}));
  return NextResponse.json(
    {
      error: errorData.error || fallback,
      code: errorData.code,
      field: errorData.field,
      destination_status: errorData.destination_status,
      required_plan: errorData.required_plan,
    },
    { status: response.status }
  );
}

export function notificationUnreachableResponse(fallback: string) {
  return NextResponse.json({ error: fallback }, { status: 502 });
}
