import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { NotificationChannel, NotificationChannelListResponse } from "@/types/notifications";
import { notificationErrorResponse, notificationUnreachableResponse } from "./errors";

// GET /api/notification-channels - The organization's channels, with webhook hints only
export async function GET() {
  const response = await serverApiRequest<NotificationChannelListResponse>("/notification-channels");
  if (!response.ok) {
    return notificationErrorResponse(response, "Failed to fetch notification channels");
  }
  return NextResponse.json(await response.json());
}

// POST /api/notification-channels
export async function POST(request: NextRequest) {
  const body = await request.text();
  try {
    const response = await serverApiRequest<NotificationChannel>("/notification-channels", { method: "POST", body });
    if (!response.ok) {
      return notificationErrorResponse(response, "Failed to create notification channel");
    }
    return NextResponse.json(await response.json(), { status: 201 });
  } catch {
    return notificationUnreachableResponse("Failed to create notification channel");
  }
}
