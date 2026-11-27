// What the app/api/billing/setup route handlers answer for a backend response. The setup token
// lives in an httpOnly cookie and is never in a body; a sign-in token never is either.

export interface SetupDecision {
  status: number;
  body: Record<string, unknown>;
  /** Set when the card was saved: the route stores it as access_token. */
  token?: string;
  /** The setup session is over (saved, expired or no longer pending): the route clears its cookie. */
  clearSetup?: boolean;
  retryAfter?: string;
}

export const NO_SETUP_SESSION: SetupDecision = { status: 401, body: { error: "Setup session expired" }, clearSetup: true };

function errorBody(data: Record<string, unknown>, fallback: string): Record<string, unknown> {
  const body: Record<string, unknown> = { error: typeof data.error === "string" && data.error ? data.error : fallback };
  if (typeof data.code === "string") body.code = data.code;
  if (typeof data.field === "string") body.field = data.field;
  return body;
}

function failure(response: Response, data: Record<string, unknown>): SetupDecision {
  const decision: SetupDecision = { status: response.status, body: errorBody(data, "Billing setup failed") };
  if (response.status === 401 || (response.status === 409 && data.code === "not_pending")) decision.clearSetup = true;
  const retryAfter = response.headers.get("retry-after");
  if (retryAfter) decision.retryAfter = retryAfter;
  return decision;
}

// State and selection: the backend's body is what the page needs, and it holds no secret.
export async function setupDecision(response: Response): Promise<SetupDecision> {
  const data = await response.json().catch(() => ({}));
  if (!response.ok) return failure(response, data);
  return { status: response.status, body: data };
}

// Saving the card answers a sign-in: the token goes to its cookie and the page only learns it worked.
export async function cardDecision(response: Response): Promise<SetupDecision> {
  const data = await response.json().catch(() => ({}));
  if (!response.ok) return failure(response, data);
  if (typeof data.token !== "string" || !data.token) {
    return { status: 500, body: { error: "No token received from server" } };
  }
  return { status: 200, body: { success: true }, token: data.token, clearSetup: true };
}

// Only Wompi's token and the two acceptances go on to the backend, whatever the page sent.
export function cardRequestBody(input: unknown): Record<string, string> | null {
  if (typeof input !== "object" || input === null) return null;
  const body: Record<string, string> = {};
  for (const key of ["card_token", "acceptance_token", "personal_data_acceptance_token"]) {
    const value = (input as Record<string, unknown>)[key];
    if (typeof value !== "string" || !value) return null;
    body[key] = value;
  }
  return body;
}
