import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { sessionDecision, type SessionDecision } from "./sessionDecision";

export async function backendSessionResponse(
  endpoint: string,
  body: unknown,
  fallbackError: string,
  decide: (response: Response, fallbackError: string) => Promise<SessionDecision> = sessionDecision
): Promise<NextResponse> {
  try {
    const response = await fetch(`${config.apiUrl}${endpoint}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const decision = await decide(response, fallbackError);
    const res = NextResponse.json(decision.body, { status: decision.status });
    if (decision.token) {
      res.cookies.set("access_token", decision.token, {
        httpOnly: true,
        secure: process.env.NODE_ENV === "production",
        sameSite: "strict",
        maxAge: 60 * 60 * 24 * 7,
        path: "/",
      });
    }
    return res;
  } catch (error) {
    console.error(`Session request to ${endpoint} failed:`, error);
    return NextResponse.json({ error: "Internal server error" }, { status: 500 });
  }
}
