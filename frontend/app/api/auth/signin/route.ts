import { NextRequest, NextResponse } from "next/server";
import { signInBody } from "@/lib/signIn";
import { backendSessionResponse } from "@/lib/auth/sessionResponse";

export async function POST(request: NextRequest) {
  const backendBody = signInBody(await request.json().catch(() => ({})));
  if (!backendBody) {
    return NextResponse.json(
      { error: "Email or username, and password, are required" },
      { status: 400 }
    );
  }
  return backendSessionResponse("/auth/signin", backendBody, "Authentication failed");
}
