import { cardLabel } from "./card.ts";
import { formatLongDate } from "./setup.ts";
import { isPeriod, isPlan, type Period, type Plan } from "../plans.ts";

// GET /api/billing (backend/docs/api/billing.md).

export type ChargeStatus = "pending" | "approved" | "declined" | "voided" | "error";

export interface SubscriptionCharge {
  id: string;
  date: string;
  amount: number;
  status: ChargeStatus;
  coversFrom: string | null;
  coversUntil: string | null;
}

export interface SavedCard {
  brand: string;
  lastFour: string;
}

export interface Subscription {
  plan: Plan;
  period: Period;
  status: string;
  trialEndsAt: string | null;
  paidUntil: string | null;
  dueDate: string | null;
  nextChargeAmount: number | null;
  cancelled: boolean;
  paymentFailed: boolean;
  accessEndsAt: string | null;
  card: SavedCard | null;
  latestCharge: SubscriptionCharge | null;
  charges: SubscriptionCharge[];
  cardEntry: boolean;
}

const CHARGE_STATUSES: readonly string[] = ["pending", "approved", "declined", "voided", "error"];

const text = (value: unknown): string | null => (typeof value === "string" && value ? value : null);

export function parseCharge(value: unknown): SubscriptionCharge | null {
  if (typeof value !== "object" || value === null) return null;
  const c = value as Record<string, unknown>;
  if (typeof c.id !== "string" || typeof c.date !== "string" || typeof c.amount !== "number") return null;
  if (typeof c.status !== "string" || !CHARGE_STATUSES.includes(c.status)) return null;
  return {
    id: c.id,
    date: c.date,
    amount: c.amount,
    status: c.status as ChargeStatus,
    coversFrom: text(c.covers_from),
    coversUntil: text(c.covers_until),
  };
}

export function parseCard(value: unknown): SavedCard | null {
  if (typeof value !== "object" || value === null) return null;
  const c = value as Record<string, unknown>;
  const lastFour = typeof c.last_four === "string" && /^\d{4}$/.test(c.last_four) ? c.last_four : "";
  const brand = typeof c.brand === "string" ? c.brand : "";
  return { brand, lastFour };
}

export function parseSubscription(body: unknown): Subscription | null {
  if (typeof body !== "object" || body === null) return null;
  const b = body as Record<string, unknown>;
  if (!isPlan(b.plan) || !isPeriod(b.period) || typeof b.status !== "string") return null;
  const charges = Array.isArray(b.charges) ? b.charges.map(parseCharge).filter((c): c is SubscriptionCharge => c !== null) : [];
  return {
    plan: b.plan,
    period: b.period,
    status: b.status,
    trialEndsAt: text(b.trial_ends_at),
    paidUntil: text(b.paid_until),
    dueDate: text(b.due_date),
    nextChargeAmount: typeof b.next_charge_amount === "number" ? b.next_charge_amount : null,
    cancelled: b.cancelled === true,
    paymentFailed: b.payment_failed === true,
    accessEndsAt: text(b.access_ends_at),
    card: parseCard(b.card),
    latestCharge: parseCharge(b.latest_charge) ?? charges[0] ?? null,
    charges,
    cardEntry: b.card_entry === true,
  };
}

// "Visa terminada en 4242"; "Tarjeta terminada en 4242" for a brand we don't name.
export function savedCardLabel(card: SavedCard): string {
  return cardLabel(card.brand, card.lastFour) ?? "Tarjeta guardada";
}

export const CHARGE_STATUS_LABELS: Record<ChargeStatus, string> = {
  pending: "En proceso",
  approved: "Aprobado",
  declined: "Rechazado",
  voided: "Anulado",
  error: "No se pudo cobrar",
};

export function longDate(value: string | null): string | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : formatLongDate(new Date(parsed));
}

export type SubscriptionStateKind = "trial" | "paid" | "cancelled" | "failed" | "no-card";

export interface SubscriptionState {
  kind: SubscriptionStateKind;
  /** The state in words, e.g. "En prueba hasta el 15 de diciembre de 2026". */
  label: string;
  /** What happens next, when there is something to say. */
  detail: string | null;
}

// The state the Suscripción page shows. A failed payment comes before everything: it is what the
// admin has to act on. Without a card there is nothing to charge and billing is handled with support.
export function subscriptionState(sub: Subscription): SubscriptionState {
  const due = longDate(sub.dueDate);
  const ends = longDate(sub.accessEndsAt);

  if (!sub.card) {
    const trialEnd = sub.status === "trial" ? longDate(sub.trialEndsAt) : null;
    return {
      kind: "no-card",
      label: trialEnd ? `En prueba hasta el ${trialEnd}` : "Activa",
      detail: "No hay una tarjeta guardada. La facturación de tu restaurante se maneja con soporte.",
    };
  }
  if (sub.paymentFailed) {
    return {
      kind: "failed",
      label: "El pago falló",
      detail: ends ? `No pudimos cobrar tu suscripción. Cambia la tarjeta antes del ${ends} para no perder el acceso.` : "No pudimos cobrar tu suscripción. Cambia la tarjeta para no perder el acceso.",
    };
  }
  if (sub.cancelled) {
    return {
      kind: "cancelled",
      label: due ? `Cancelada. Tu último día es el ${due}` : "Cancelada",
      detail: "No se hará ningún cobro más. Puedes reanudarla antes de esa fecha.",
    };
  }
  if (sub.status === "trial") {
    return {
      kind: "trial",
      label: due ? `En prueba hasta el ${due}` : "En prueba",
      detail: due ? `El primer cobro se hace ese día.` : null,
    };
  }
  if (sub.paidUntil) {
    return { kind: "paid", label: due ? `Pagada hasta el ${due}` : "Pagada", detail: "Se renueva sola ese día." };
  }
  // A card on file and no paid period: the operator activated it by hand.
  return { kind: "no-card", label: "Activa", detail: "La facturación de tu restaurante se maneja con soporte." };
}
