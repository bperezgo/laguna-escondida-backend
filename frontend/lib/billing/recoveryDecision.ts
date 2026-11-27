import { cardRequestBody, type SetupDecision } from "./setupDecision.ts";

// What the app/api/billing/recovery route handlers answer for a backend response. As in signup,
// the setup token lives in an httpOnly cookie and a sign-in token never reaches the page.

function failure(response: Response, data: Record<string, unknown>): SetupDecision {
  const body: Record<string, unknown> = { error: typeof data.error === "string" && data.error ? data.error : "Recovery failed" };
  if (typeof data.code === "string") body.code = data.code;
  if (typeof data.field === "string") body.field = data.field;
  const decision: SetupDecision = { status: response.status, body };
  // The session is over, or the account works again: either way there is nothing left to recover.
  if (response.status === 401 || (response.status === 409 && data.code === "not_stopped")) decision.clearSetup = true;
  const retryAfter = response.headers.get("retry-after");
  if (retryAfter) decision.retryAfter = retryAfter;
  return decision;
}

// Once the payment is approved the backend answers a sign-in instead of the state: the token goes
// to its cookie, the setup session ends, and the page only learns it is signed in.
export async function recoveryStateDecision(response: Response): Promise<SetupDecision> {
  const data = await response.json().catch(() => ({}));
  if (!response.ok) return failure(response, data);
  if (typeof data.token === "string" && data.token) {
    return { status: 200, body: { signed_in: true }, token: data.token, clearSetup: true };
  }
  return { status: response.status, body: data };
}

// Selection and pay: the backend's body is what the page needs, and it holds no secret.
export async function recoveryDecision(response: Response): Promise<SetupDecision> {
  const data = await response.json().catch(() => ({}));
  if (!response.ok) return failure(response, data);
  return { status: response.status, body: data };
}

// Nothing, to pay with the saved card; or Wompi's token and the two acceptances of a new one.
// Null for half a card, which would otherwise charge the saved one by surprise.
export function payRequestBody(input: unknown): Record<string, string> | null {
  if (input === null || input === undefined) return {};
  if (typeof input !== "object") return null;
  const sent = ["card_token", "acceptance_token", "personal_data_acceptance_token"].filter((key) => {
    const value = (input as Record<string, unknown>)[key];
    return value !== undefined && value !== null && value !== "";
  });
  if (sent.length === 0) return {};
  return cardRequestBody(input);
}
