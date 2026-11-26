import { NextRequest, NextResponse } from "next/server";
import { backendSessionResponse } from "@/lib/auth/sessionResponse";

export async function POST(request: NextRequest) {
  const { id_token } = await request.json().catch(() => ({}));
  if (typeof id_token !== "string" || !id_token) {
    return NextResponse.json({ error: "id_token is required" }, { status: 400 });
  }
  return backendSessionResponse("/auth/google", { id_token }, "Google sign-in failed");
}
