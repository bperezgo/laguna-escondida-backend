import { trialExpiredNotice } from "./support.ts";

export const NOT_ACTIVE_YET_MESSAGE = "Tu cuenta todavía no está activa. Pídele a quien administra el restaurante que termine el registro.";

// The cloud signs in by email; an edge box signs its own restaurant's users in by username.
// GET /api/auth/context says which, so the form never asks for the restaurant.

export type SignInMode = "cloud" | "edge";

export interface SignInContext {
  mode: SignInMode;
  organization?: { name: string };
}

// An edge box older than /api/auth/context answers 404; the cloud is always deployed first.
export const LEGACY_EDGE_CONTEXT: SignInContext = { mode: "edge" };

export function parseSignInContext(body: unknown): SignInContext {
  if (typeof body !== "object" || body === null) return LEGACY_EDGE_CONTEXT;
  const { mode, organization } = body as { mode?: unknown; organization?: { name?: unknown } };
  if (mode === "cloud") return { mode: "cloud" };
  if (mode === "edge" && typeof organization?.name === "string" && organization.name) {
    return { mode: "edge", organization: { name: organization.name } };
  }
  return LEGACY_EDGE_CONTEXT;
}

export interface SignInCredentials {
  email?: string;
  username?: string;
  password?: string;
}

// The backend sign-in body: the email when one is given, otherwise the username. Null when a
// credential or the password is missing.
export function signInBody({ email, username, password }: SignInCredentials) {
  if (!password) return null;
  const trimmedEmail = email?.trim();
  if (trimmedEmail) return { email: trimmedEmail, password };
  if (username) return { username, password };
  return null;
}

const INVALID_CREDENTIALS: Record<string, string> = {
  "Invalid email or password": "Correo o contraseña incorrectos",
  "Invalid username or password": "Usuario o contraseña incorrectos",
};

export function signInErrorMessage(message: string): string {
  return INVALID_CREDENTIALS[message] ?? message;
}

export interface SignInFailure {
  message: string;
  /** Set for an ended trial when a support email is configured. */
  mailto: string | null;
}

export function signInFailure(
  body: { error?: string; code?: string },
  email: string | null
): SignInFailure {
  if (body.code === "trial_expired") {
    const notice = trialExpiredNotice(email);
    return { message: notice.message, mailto: notice.mailto };
  }
  if (body.error === "Google sign-in failed") {
    return { message: "No pudimos iniciar sesión con esa cuenta de Google.", mailto: null };
  }
  return { message: signInErrorMessage(body.error || "No se pudo iniciar sesión"), mailto: null };
}

export type SignInOutcome = { next: "home" } | { next: "payment" } | { next: "failure"; failure: SignInFailure };

// A pending admin continues signup at step 2 (`setup`: the route handler stored a setup session);
// anyone else of a pending organization can only be told to wait.
export function signInOutcome(
  ok: boolean,
  body: { error?: string; code?: string; setup?: boolean },
  email: string | null
): SignInOutcome {
  if (ok) return { next: "home" };
  if (body.code === "pending_approval") {
    return body.setup ? { next: "payment" } : { next: "failure", failure: { message: NOT_ACTIVE_YET_MESSAGE, mailto: null } };
  }
  return { next: "failure", failure: signInFailure(body, email) };
}

// /signin?aviso=sesion: the setup session ran out in the middle of signup.
export function signInNotice(aviso: unknown): string | null {
  return aviso === "sesion" ? "Inicia sesión de nuevo para continuar con tu registro." : null;
}
