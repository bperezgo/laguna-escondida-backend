import { isPlan, type Plan } from "./plans.ts";
import { NOT_ACTIVE_YET_MESSAGE } from "./signIn.ts";
import { trialExpiredNotice } from "./support.ts";

export const RESTAURANT_NAME_MAX_LENGTH = 255;

export interface SignupForm {
  restaurantName: string;
  plan: Plan | null;
}

export interface SignupErrors {
  restaurant_name?: string;
  plan?: string;
}

export function validateSignup({ restaurantName, plan }: SignupForm): SignupErrors {
  const errors: SignupErrors = {};
  const name = restaurantName.trim();
  if (!name) errors.restaurant_name = "Escribe el nombre del restaurante.";
  else if (Array.from(name).length > RESTAURANT_NAME_MAX_LENGTH) {
    errors.restaurant_name = `El nombre admite hasta ${RESTAURANT_NAME_MAX_LENGTH} caracteres.`;
  }
  if (!isPlan(plan)) errors.plan = "Elige un plan.";
  return errors;
}

export function signupBody(idToken: string, { restaurantName, plan }: SignupForm) {
  return { id_token: idToken, restaurant_name: restaurantName.trim(), plan };
}

export interface SignupFailure {
  message: string;
  mailto: string | null;
  field?: keyof SignupErrors;
}

export function signupFailure(
  status: number,
  body: { error?: string; code?: string; field?: string },
  email: string | null
): SignupFailure {
  if (body.code === "trial_expired") {
    const notice = trialExpiredNotice(email);
    return { message: notice.message, mailto: notice.mailto };
  }
  if (status === 400 && body.field === "restaurant_name") {
    return { message: "El nombre del restaurante debe tener entre 1 y 255 caracteres.", mailto: null, field: "restaurant_name" };
  }
  if (status === 400 && body.field === "plan") {
    return { message: "Elige un plan válido.", mailto: null, field: "plan" };
  }
  if (status === 401) {
    return { message: "No pudimos verificar tu cuenta de Google. Intenta de nuevo.", mailto: null };
  }
  if (status === 404) {
    return { message: "El registro no está disponible por ahora.", mailto: null };
  }
  if (status === 429) {
    return { message: "Demasiados intentos. Espera un momento e intenta de nuevo.", mailto: null };
  }
  return { message: "No pudimos crear la cuenta. Intenta de nuevo.", mailto: null };
}

export const SIGNUP_PATH = "/registro";
export const PAYMENT_PATH = "/registro/pago";
export const DONE_PATH = "/registro/listo";

export const SIGNUP_STEPS = ["Cuenta", "Pago", "Listo"] as const;

// Where a signup response sends the person: a new account and one still pending both continue at
// step 2, when the response left a setup session (`setup`); an account with access gets a session.
export type SignupOutcome = { next: "home" } | { next: "payment" } | { next: "failure"; failure: SignupFailure };

export function signupOutcome(
  status: number,
  body: { error?: string; code?: string; field?: string; status?: string; setup?: boolean },
  email: string | null
): SignupOutcome {
  if (status === 201 && body.status === "pending") return { next: "payment" };
  if (body.code === "pending_approval") {
    return body.setup ? { next: "payment" } : { next: "failure", failure: { message: NOT_ACTIVE_YET_MESSAGE, mailto: null } };
  }
  if (status >= 200 && status < 300) return { next: "home" };
  return { next: "failure", failure: signupFailure(status, body, email) };
}
