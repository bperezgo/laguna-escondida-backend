import { cookies } from "next/headers";
import { NextResponse } from "next/server";
import { config } from "@/lib/config/config";
import { SETUP_COOKIE } from "@/lib/auth/sessionDecision";
import { clearSetupCookie, setAccessCookie } from "@/lib/auth/sessionResponse";
import { NO_SETUP_SESSION, setupDecision, type SetupDecision } from "./setupDecision";

function respond(decision: SetupDecision): NextResponse {
  const headers: Record<string, string> = { "Cache-Control": "no-store" };
  if (decision.retryAfter) headers["Retry-After"] = decision.retryAfter;
  const res = NextResponse.json(decision.body, { status: decision.status, headers });
  if (decision.token) setAccessCookie(res, decision.token);
  if (decision.clearSetup) clearSetupCookie(res);
  return res;
}

// Forwards a billing setup request with the setup token from its httpOnly cookie as the bearer.
export async function backendSetupResponse(
  method: "GET" | "PUT" | "POST",
  endpoint: string,
  body?: unknown,
  decide: (response: Response) => Promise<SetupDecision> = setupDecision
): Promise<NextResponse> {
  const setupToken = (await cookies()).get(SETUP_COOKIE)?.value;
  if (!setupToken) return respond(NO_SETUP_SESSION);
  try {
    const response = await fetch(`${config.apiUrl}${endpoint}`, {
      method,
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${setupToken}` },
      body: body === undefined ? undefined : JSON.stringify(body),
      cache: "no-store",
    });
    return respond(await decide(response));
  } catch (error) {
    console.error(`Billing setup request to ${endpoint} failed:`, error);
    return NextResponse.json({ error: "Internal server error" }, { status: 500 });
  }
}
