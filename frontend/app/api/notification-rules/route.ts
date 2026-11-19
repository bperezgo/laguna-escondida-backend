import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { NotificationRule, NotificationRuleListResponse } from "@/types/notifications";
import { notificationErrorResponse, notificationUnreachableResponse } from "../notification-channels/errors";

// GET /api/notification-rules?channel_id= - Every rule of the organization, or one channel's
export async function GET(request: NextRequest) {
  const channelId = request.nextUrl.searchParams.get("channel_id");
  const query = channelId ? `?channel_id=${encodeURIComponent(channelId)}` : "";
  const response = await serverApiRequest<NotificationRuleListResponse>(`/notification-rules${query}`);
  if (!response.ok) {
    return notificationErrorResponse(response, "Failed to fetch notification rules");
  }
  return NextResponse.json(await response.json());
}

// POST /api/notification-rules
export async function POST(request: NextRequest) {
  const body = await request.text();
  try {
    const response = await serverApiRequest<NotificationRule>("/notification-rules", { method: "POST", body });
    if (!response.ok) {
      return notificationErrorResponse(response, "Failed to create notification rule");
    }
    return NextResponse.json(await response.json(), { status: 201 });
  } catch {
    return notificationUnreachableResponse("Failed to create notification rule");
  }
}
