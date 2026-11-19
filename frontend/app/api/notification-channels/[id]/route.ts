import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { NotificationChannel } from "@/types/notifications";
import { notificationErrorResponse, notificationUnreachableResponse } from "../errors";

type Params = { params: Promise<{ id: string }> };

function backendPath(id: string) {
  return `/notification-channels/${encodeURIComponent(id)}`;
}

// GET /api/notification-channels/:id
export async function GET(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  const response = await serverApiRequest<NotificationChannel>(backendPath(id));
  if (!response.ok) {
    return notificationErrorResponse(response, "Failed to fetch notification channel");
  }
  return NextResponse.json(await response.json());
}

// PUT /api/notification-channels/:id - Rename, enable/disable or replace the webhook
export async function PUT(request: NextRequest, { params }: Params) {
  const { id } = await params;
  const body = await request.text();
  try {
    const response = await serverApiRequest<NotificationChannel>(backendPath(id), { method: "PUT", body });
    if (!response.ok) {
      return notificationErrorResponse(response, "Failed to update notification channel");
    }
    return NextResponse.json(await response.json());
  } catch {
    return notificationUnreachableResponse("Failed to update notification channel");
  }
}

// DELETE /api/notification-channels/:id - Deletes the channel and its rules
export async function DELETE(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  const response = await serverApiRequest(backendPath(id), { method: "DELETE" });
  if (!response.ok) {
    return notificationErrorResponse(response, "Failed to delete notification channel");
  }
  return NextResponse.json(await response.json());
}
