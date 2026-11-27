import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { SETUP_COOKIE, SETUP_COOKIE_MAX_AGE, sessionDecision, type SessionDecision } from "./sessionDecision";

const cookieBase = () => ({
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "strict" as const,
  path: "/",
});

export function setAccessCookie(res: NextResponse, token: string) {
  res.cookies.set("access_token", token, { ...cookieBase(), maxAge: 60 * 60 * 24 * 7 });
}

export function setSetupCookie(res: NextResponse, token: string) {
  res.cookies.set(SETUP_COOKIE, token, { ...cookieBase(), maxAge: SETUP_COOKIE_MAX_AGE });
}

export function clearSetupCookie(res: NextResponse) {
  res.cookies.set(SETUP_COOKIE, "", { ...cookieBase(), maxAge: 0 });
}

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
      setAccessCookie(res, decision.token);
      clearSetupCookie(res);
    }
    if (decision.setupToken) setSetupCookie(res, decision.setupToken);
    return res;
  } catch (error) {
    console.error(`Session request to ${endpoint} failed:`, error);
    return NextResponse.json({ error: "Internal server error" }, { status: 500 });
  }
}
