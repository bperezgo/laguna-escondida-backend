import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";

// Public: no session is forwarded.
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ token: string }> }
) {
  const { token } = await params;
  try {
    const response = await fetch(`${config.apiUrl}/invitations/${encodeURIComponent(token)}`);
    const body = await response.json().catch(() => ({ error: "Failed to look up the invitation" }));
    return NextResponse.json(body, { status: response.status, headers: { "Cache-Control": "no-store" } });
  } catch (error) {
    console.error("Invitation lookup failed:", error);
    return NextResponse.json({ error: "Failed to look up the invitation" }, { status: 502 });
  }
}
