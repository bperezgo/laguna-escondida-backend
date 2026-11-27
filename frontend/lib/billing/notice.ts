import { formatLongDate } from "./setup.ts";
import { formatPesos } from "../plans.ts";
import { supportMailto, TRIAL_EXPIRED_MESSAGE } from "../support.ts";
import { trialBanner, trialDaysLeft } from "../trial.ts";

export const SUBSCRIPTION_PATH = "/ajustes/suscripcion";
export const CHANGE_CARD_PATH = `${SUBSCRIPTION_PATH}#tarjeta`;

export interface NoticeBilling {
  has_card: boolean;
  card_entry: boolean;
  cancelled: boolean;
  due_date: string | null;
  next_charge_amount: number | null;
  payment_failed: boolean;
  access_ends_at: string | null;
}

export interface NoticeOrganization {
  status?: string;
  trial_ends_at?: string | null;
}

export interface SubscriptionNoticeLink {
  href: string;
  label: string;
}

export interface SubscriptionNotice {
  tone: "warning" | "danger";
  message: string;
  link: SubscriptionNoticeLink | null;
}

export interface NoticeInput {
  organization: NoticeOrganization | null | undefined;
  /** Null for anyone who does not manage billing: they see only the trial's days left. */
  billing: NoticeBilling | null | undefined;
  isEdge: boolean;
  now: number;
  supportEmail: string | null;
}

function dateOf(value: string | null): string | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : formatLongDate(new Date(parsed));
}

function daysLeftMessage(days: number): string {
  if (days === 0) return "Tu prueba gratis termina hoy.";
  return days === 1 ? "Te queda 1 día de prueba gratis." : `Te quedan ${days} días de prueba gratis.`;
}

// The one notice at the top of every page. A paid organization with nothing wrong gets none, and
// neither does an edge box, where there is no billing.
export function subscriptionNotice({ organization, billing, isEdge, now, supportEmail }: NoticeInput): SubscriptionNotice | null {
  if (isEdge || !organization) return null;

  if (!billing) {
    const banner = trialBanner(organization, isEdge, now, supportEmail);
    if (!banner) return null;
    return {
      tone: "warning",
      message: banner.message,
      link: banner.mailto
        ? { href: banner.mailto, label: banner.message === TRIAL_EXPIRED_MESSAGE ? "Escríbenos" : "Escríbenos para continuar" }
        : null,
    };
  }

  if (billing.payment_failed) {
    const ends = dateOf(billing.access_ends_at);
    return {
      tone: "danger",
      message: ends
        ? `No pudimos cobrar tu suscripción. Tu acceso termina el ${ends}.`
        : "No pudimos cobrar tu suscripción.",
      link: { href: CHANGE_CARD_PATH, label: "Cambiar la tarjeta" },
    };
  }

  const working = organization.status === "trial" || organization.status === "active";
  if (billing.cancelled && working) {
    const last = dateOf(billing.access_ends_at ?? billing.due_date);
    if (!last) return null;
    return {
      tone: "warning",
      message: `Cancelaste tu suscripción. Tu último día de acceso es el ${last}.`,
      link: { href: SUBSCRIPTION_PATH, label: "Reanudar" },
    };
  }

  if (organization.status !== "trial" || !organization.trial_ends_at) return null;
  const days = trialDaysLeft(organization.trial_ends_at, now);
  if (days === null) return null;

  if (billing.has_card) {
    const chargeOn = dateOf(billing.due_date ?? organization.trial_ends_at);
    const charge =
      billing.next_charge_amount !== null && chargeOn
        ? ` El ${chargeOn}, el último día de tu prueba, se cobrarán ${formatPesos(billing.next_charge_amount)}.`
        : "";
    return { tone: "warning", message: `${daysLeftMessage(days)}${charge}`, link: { href: SUBSCRIPTION_PATH, label: "Suscripción" } };
  }

  if (billing.card_entry) {
    return { tone: "warning", message: daysLeftMessage(days), link: { href: SUBSCRIPTION_PATH, label: "Suscripción" } };
  }
  const mailto = supportMailto(supportEmail, "Continuar después de la prueba");
  return { tone: "warning", message: daysLeftMessage(days), link: mailto ? { href: mailto, label: "Escríbenos para continuar" } : null };
}
