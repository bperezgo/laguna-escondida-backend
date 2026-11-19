import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import { notificationErrorResponse, notificationUnreachableResponse } from "../../errors";

type Params = { params: Promise<{ id: string }> };

// POST /api/notification-channels/:id/test - Sends a sample message right away
export async function POST(_request: NextRequest, { params }: Params) {
  const { id } = await params;
  try {
    const response = await serverApiRequest(`/notification-channels/${encodeURIComponent(id)}/test`, {
      method: "POST",
    });
    if (response.status === 204) {
      return new NextResponse(null, { status: 204 });
    }
    return notificationErrorResponse(response, "Failed to test notification channel");
  } catch {
    return notificationUnreachableResponse("Failed to test notification channel");
  }
}
