import { NextRequest } from "next/server";
import { backendSessionResponse } from "@/lib/auth/sessionResponse";

export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ token: string }> }
) {
  const { token } = await params;
  const { password, id_token } = await request.json().catch(() => ({}));
  return backendSessionResponse(
    `/invitations/${encodeURIComponent(token)}/accept`,
    id_token ? { id_token } : { password },
    "Failed to accept the invitation"
  );
}
