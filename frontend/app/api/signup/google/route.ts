import { NextRequest, NextResponse } from "next/server";
import { signupDecision } from "@/lib/auth/sessionDecision";
import { backendSessionResponse } from "@/lib/auth/sessionResponse";

export async function POST(request: NextRequest) {
  const { id_token, restaurant_name, plan } = await request.json().catch(() => ({}));
  if (typeof id_token !== "string" || !id_token) {
    return NextResponse.json({ error: "id_token is required" }, { status: 400 });
  }
  return backendSessionResponse("/signup/google", { id_token, restaurant_name, plan }, "Signup failed", signupDecision);
}
