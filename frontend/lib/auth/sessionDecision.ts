// What a sign-in route handler answers for a backend response that carries a session token
// (sign-in, Google sign-in, signup, invitation acceptance). Errors keep the backend's status,
// `code` and `field` so the page can tell trial_expired, invalid_or_expired_link, etc. apart.
export interface SessionDecision {
  status: number;
  body: Record<string, unknown>;
  /** Set only on success: the route stores it in the access_token cookie. */
  token?: string;
  /** A pending or stopped organization's admin's setup token: the route stores it in the setup_token cookie. Never in `body`. */
  setupToken?: string;
}

export const SETUP_COOKIE = "setup_token";

// Refusals that give an admin a setup session: to finish signup, or to pay a stopped account's way back.
const SETUP_SESSION_CODES: readonly unknown[] = ["pending_approval", "trial_expired", "payment_required"];
export const SETUP_COOKIE_MAX_AGE = 60 * 60;

function setupTokenOf(data: Record<string, unknown>): string | undefined {
  return typeof data.setup_token === "string" && data.setup_token ? data.setup_token : undefined;
}

export async function sessionDecision(response: Response, fallbackError: string): Promise<SessionDecision> {
  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    const body: Record<string, unknown> = { error: data.error || data.message || fallbackError };
    if (data.code) body.code = data.code;
    if (data.field) body.field = data.field;
    const setupToken = SETUP_SESSION_CODES.includes(data.code) ? setupTokenOf(data) : undefined;
    if (!setupToken) return { status: response.status, body };
    // The page only learns that a setup session exists; the token stays in its httpOnly cookie.
    return { status: response.status, body: { ...body, setup: true }, setupToken };
  }

  if (typeof data.token !== "string" || !data.token) {
    return { status: 500, body: { error: "No token received from server" } };
  }

  return { status: response.status, body: { success: true }, token: data.token };
}

// A new signup answers 201 pending, with a setup token and no sign-in token: no session is set for it.
export async function signupDecision(response: Response, fallbackError: string): Promise<SessionDecision> {
  if (response.status !== 201) return sessionDecision(response, fallbackError);
  const data = await response.json().catch(() => ({}));
  const body: Record<string, unknown> = {};
  for (const key of ["status", "organization_name", "plan"]) {
    if (typeof data[key] === "string") body[key] = data[key];
  }
  const setupToken = setupTokenOf(data);
  if (!setupToken) return { status: 201, body };
  return { status: 201, body: { ...body, setup: true }, setupToken };
}
