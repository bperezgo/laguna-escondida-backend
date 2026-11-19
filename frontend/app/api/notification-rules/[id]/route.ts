import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { NotificationRule } from "@/types/notifications";
import { notificationErrorResponse, notificationUnreachableResponse } from "../../notification-channels/errors";

type Params = { params: Promise<{ id: string }> };

function backendPath(id: string) {
  return `/notification-rules/${encodeURIComponent(id)}`;
}

// GET /api/notification-rules/:id
export async function GET(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  const response = await serverApiRequest<NotificationRule>(backendPath(id));
  if (!response.ok) {
    return notificationErrorResponse(response, "Failed to fetch notification rule");
  }
  return NextResponse.json(await response.json());
}

// PUT /api/notification-rules/:id - The event can't change
export async function PUT(request: NextRequest, { params }: Params) {
  const { id } = await params;
  const body = await request.text();
  try {
    const response = await serverApiRequest<NotificationRule>(backendPath(id), { method: "PUT", body });
    if (!response.ok) {
      return notificationErrorResponse(response, "Failed to update notification rule");
    }
    return NextResponse.json(await response.json());
  } catch {
    return notificationUnreachableResponse("Failed to update notification rule");
  }
}

// DELETE /api/notification-rules/:id
export async function DELETE(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  const response = await serverApiRequest(backendPath(id), { method: "DELETE" });
  if (!response.ok) {
    return notificationErrorResponse(response, "Failed to delete notification rule");
  }
  return NextResponse.json(await response.json());
}
