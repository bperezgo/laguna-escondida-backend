// What a sign-in route handler answers for a backend response that carries a session token
// (sign-in, Google sign-in, signup, invitation acceptance). Errors keep the backend's status,
// `code` and `field` so the page can tell trial_expired, invalid_or_expired_link, etc. apart.
export interface SessionDecision {
  status: number;
  body: Record<string, unknown>;
  /** Set only on success: the route stores it in the access_token cookie. */
  token?: string;
}

export async function sessionDecision(response: Response, fallbackError: string): Promise<SessionDecision> {
  const data = await response.json().catch(() => ({}));

  if (!response.ok) {
    const body: Record<string, unknown> = { error: data.error || data.message || fallbackError };
    if (data.code) body.code = data.code;
    if (data.field) body.field = data.field;
    return { status: response.status, body };
  }

  if (typeof data.token !== "string" || !data.token) {
    return { status: 500, body: { error: "No token received from server" } };
  }

  return { status: response.status, body: { success: true }, token: data.token };
}

// A new signup answers 201 waiting for approval, with no token: no session is set for it.
export async function signupDecision(response: Response, fallbackError: string): Promise<SessionDecision> {
  if (response.status !== 201) return sessionDecision(response, fallbackError);
  const data = await response.json().catch(() => ({}));
  const body: Record<string, unknown> = {};
  for (const key of ["status", "organization_name", "plan"]) {
    if (typeof data[key] === "string") body[key] = data[key];
  }
  return { status: 201, body };
}
