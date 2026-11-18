import { NextRequest, NextResponse } from "next/server";
import { serverApiRequest } from "@/lib/api/server";
import type { ResetPasswordRequest } from "@/types/user";

// POST /api/admin/users/:id/reset-password - Set a new admin-defined password
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const body: ResetPasswordRequest = await request.json();
    const response = await serverApiRequest(
      `/admin/users/${id}/reset-password`,
      {
        method: "POST",
        body: JSON.stringify(body),
      }
    );

    if (!response.ok && response.status !== 204) {
      const errorData = await response.json();
      return NextResponse.json(
        { error: errorData.error || "Failed to reset password" },
        { status: response.status }
      );
    }

    return new NextResponse(null, { status: 204 });
  } catch (error) {
    console.error("Error resetting password:", error);
    return NextResponse.json(
      {
        error:
          error instanceof Error ? error.message : "Failed to reset password",
      },
      { status: 500 }
    );
  }
}
