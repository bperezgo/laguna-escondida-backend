import { CARD_REFUSED_MESSAGE, parseSetupState, PROVIDER_DOWN_MESSAGE, TOO_MANY_ATTEMPTS_MESSAGE, type SetupState } from "./setup.ts";
import { parseCard, parseCharge, type SavedCard, type SubscriptionCharge } from "./subscription.ts";

// GET /api/billing/recovery (backend/docs/api/billing.md).

export const RECOVERY_PATH = "/recuperar";

export type StoppedReason = "trial_expired" | "payment_required";

export interface RecoveryState {
  organizationName: string;
  reason: StoppedReason;
  plan: SetupState["plan"];
  period: SetupState["period"];
  amountDue: number;
  card: SavedCard | null;
  latestCharge: SubscriptionCharge | null;
  cardEntry: boolean;
  wompi: SetupState["wompi"];
}

export function parseRecoveryState(body: unknown): RecoveryState | null {
  // The plan, the period and what card entry needs come in the setup state's shape.
  const setup = parseSetupState(body);
  if (!setup) return null;
  const b = body as Record<string, unknown>;
  if (b.reason !== "trial_expired" && b.reason !== "payment_required") return null;
  return {
    organizationName: setup.organizationName,
    reason: b.reason,
    plan: setup.plan,
    period: setup.period,
    amountDue: typeof b.amount_due === "number" ? b.amount_due : 0,
    card: parseCard(b.card),
    latestCharge: parseCharge(b.latest_charge),
    cardEntry: setup.cardEntry,
    wompi: setup.wompi,
  };
}

export const STOPPED_REASONS: Record<StoppedReason, string> = {
  trial_expired: "Tu prueba gratis terminó. Paga tu plan para seguir usando Kuenta.",
  payment_required: "No pudimos cobrar tu suscripción y tu cuenta se detuvo. Paga para volver a usarla.",
};

export const CHARGE_PENDING_MESSAGE = "Estamos confirmando tu pago con el banco. Esto puede tardar unos segundos; no cierres esta página.";
export const CHARGE_DECLINED_MESSAGE = "El banco rechazó el pago. Prueba con otra tarjeta.";
export const RECOVERY_POLL_MS = 4000;

// What the page shows for the state it read. `home`: the payment was approved and the route
// handler stored the session. `signin`: the setup session is over, or the account already works.
export type RecoveryView =
  | { next: "home" }
  | { next: "signin"; expired: boolean }
  | { next: "pending"; state: RecoveryState }
  | { next: "declined"; state: RecoveryState }
  | { next: "pay"; state: RecoveryState }
  | { next: "retry" };

export function recoveryView(status: number, body: unknown): RecoveryView {
  const b = (typeof body === "object" && body !== null ? body : {}) as Record<string, unknown>;
  if (status === 401) return { next: "signin", expired: true };
  if (status === 409 && b.code === "not_stopped") return { next: "signin", expired: false };
  if (status < 200 || status >= 300) return { next: "retry" };
  if (b.signed_in === true) return { next: "home" };
  const state = parseRecoveryState(b);
  if (!state) return { next: "retry" };
  const charge = state.latestCharge?.status;
  if (charge === "pending") return { next: "pending", state };
  if (charge === "declined" || charge === "voided" || charge === "error") return { next: "declined", state };
  return { next: "pay", state };
}

// What the page does with the answer of a payment.
export type PayOutcome =
  | { next: "poll" }
  | { next: "signin"; expired: boolean }
  | { next: "retry"; message: string; clearCard: boolean };

export function payOutcome(status: number, body: { code?: string; field?: string }): PayOutcome {
  // A charge that is already waiting is the one to follow.
  if (status === 202 || (status === 409 && body.code === "charge_pending")) return { next: "poll" };
  if (status === 401) return { next: "signin", expired: true };
  if (status === 409 && body.code === "not_stopped") return { next: "signin", expired: false };
  if (status === 402 || body.code === "card_refused") return { next: "retry", message: CARD_REFUSED_MESSAGE, clearCard: true };
  if (status === 429) return { next: "retry", message: TOO_MANY_ATTEMPTS_MESSAGE, clearCard: false };
  if (status === 502 || body.code === "payment_provider_unavailable") return { next: "retry", message: PROVIDER_DOWN_MESSAGE, clearCard: false };
  if (status === 400 && body.field === "card_token") return { next: "retry", message: "Ingresa una tarjeta para pagar.", clearCard: false };
  return { next: "retry", message: "No pudimos hacer el cobro. Intenta de nuevo.", clearCard: false };
}
