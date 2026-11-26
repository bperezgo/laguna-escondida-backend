import { isPlan, type Plan } from "./plans.ts";
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
