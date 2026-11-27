import { isPeriod, isPlan, type Period, type Plan } from "../plans.ts";

// GET /api/billing/setup (backend/docs/api/billing.md).
export interface SetupState {
  organizationName: string;
  plan: Plan;
  period: Period;
  cardEntry: boolean;
  wompi: {
    publicKey: string;
    apiUrl: string;
    termsUrl: string;
    personalDataUrl: string;
    acceptanceToken: string;
    personalDataAcceptanceToken: string;
  } | null;
}

export function parseSetupState(body: unknown): SetupState | null {
  if (typeof body !== "object" || body === null) return null;
  const b = body as Record<string, unknown>;
  if (typeof b.organization_name !== "string" || !isPlan(b.plan) || !isPeriod(b.period)) return null;
  const w = (b.wompi ?? null) as Record<string, unknown> | null;
  const fields = ["public_key", "api_url", "terms_url", "personal_data_url", "acceptance_token", "personal_data_acceptance_token"];
  const complete = b.card_entry === true && w !== null && fields.every((f) => typeof w[f] === "string" && w[f]);
  return {
    organizationName: b.organization_name,
    plan: b.plan,
    period: b.period,
    cardEntry: complete,
    wompi: complete && w
      ? {
          publicKey: w.public_key as string,
          apiUrl: w.api_url as string,
          termsUrl: w.terms_url as string,
          personalDataUrl: w.personal_data_url as string,
          acceptanceToken: w.acceptance_token as string,
          personalDataAcceptanceToken: w.personal_data_acceptance_token as string,
        }
      : null,
  };
}

const DAY_MS = 24 * 60 * 60 * 1000;

// The first charge falls when the trial ends: trialDays after the card is saved.
export function firstChargeDate(now: Date, trialDays: number): Date {
  return new Date(now.getTime() + trialDays * DAY_MS);
}

const MONTHS = ["enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"];

// "10 de diciembre de 2026", in Colombia's time whatever the browser's.
export function formatLongDate(date: Date): string {
  const bogota = new Date(date.getTime() - 5 * 60 * 60 * 1000);
  return `${bogota.getUTCDate()} de ${MONTHS[bogota.getUTCMonth()]} de ${bogota.getUTCFullYear()}`;
}

export const PERIOD_LABELS: Record<Period, string> = { monthly: "Mensual", annual: "Anual" };
export const PERIOD_UNITS: Record<Period, string> = { monthly: "mes", annual: "año" };

export const CARD_REFUSED_MESSAGE = "No pudimos guardar tu tarjeta. Revisa los datos o prueba con otra.";
export const TOO_MANY_ATTEMPTS_MESSAGE = "Hiciste demasiados intentos. Espera una hora antes de probar de nuevo.";
export const PROVIDER_DOWN_MESSAGE = "No pudimos conectar con Wompi. Intenta de nuevo en unos minutos.";

// What step 2 does with the answer of a setup request (state, selection or card).
export type SetupOutcome =
  | { next: "done" }
  | { next: "signin" }
  | { next: "home" }
  | { next: "retry"; message: string; clearCard: boolean };

export function setupOutcome(status: number, body: { code?: string }): SetupOutcome {
  if (status >= 200 && status < 300) return { next: "done" };
  // The setup session ran out: the person signs in again and comes back to this step.
  if (status === 401) return { next: "signin" };
  // The trial already started, here or by the operator: there is nothing left to set up.
  if (status === 409 && body.code === "not_pending") return { next: "home" };
  if (status === 402 || body.code === "card_refused") return { next: "retry", message: CARD_REFUSED_MESSAGE, clearCard: true };
  if (status === 429) return { next: "retry", message: TOO_MANY_ATTEMPTS_MESSAGE, clearCard: false };
  if (status === 502 || body.code === "payment_provider_unavailable") return { next: "retry", message: PROVIDER_DOWN_MESSAGE, clearCard: false };
  return { next: "retry", message: "No pudimos guardar los cambios. Intenta de nuevo.", clearCard: false };
}

// What step 2 hands to step 3, kept for the tab: never the card, only what a receipt may show.
export interface SignupSummary {
  plan: Plan;
  period: Period;
  amount: number | null;
  cardBrand: string | null;
  cardLastFour: string | null;
}

export const SUMMARY_KEY = "kuenta:signup-summary";

export function parseSignupSummary(raw: string | null): SignupSummary | null {
  if (!raw) return null;
  try {
    const s = JSON.parse(raw) as Record<string, unknown>;
    if (!isPlan(s.plan) || !isPeriod(s.period)) return null;
    return {
      plan: s.plan,
      period: s.period,
      amount: typeof s.amount === "number" ? s.amount : null,
      cardBrand: typeof s.cardBrand === "string" ? s.cardBrand : null,
      cardLastFour: typeof s.cardLastFour === "string" && /^\d{4}$/.test(s.cardLastFour) ? s.cardLastFour : null,
    };
  } catch {
    return null;
  }
}
