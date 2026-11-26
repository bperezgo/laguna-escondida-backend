import { supportMailto, TRIAL_EXPIRED_MESSAGE, type Notice } from "./support.ts";

const DAY_MS = 24 * 60 * 60 * 1000;

// Whole days left, rounded up: 3 hours left is still "1 día". Null for an unreadable date.
export function trialDaysLeft(trialEndsAt: string, now: number): number | null {
  const end = Date.parse(trialEndsAt);
  if (Number.isNaN(end)) return null;
  return Math.max(0, Math.ceil((end - now) / DAY_MS));
}

interface TrialOrganization {
  status?: string;
  trial_ends_at?: string | null;
}

export function trialBanner(
  organization: TrialOrganization | null | undefined,
  isEdge: boolean,
  now: number,
  email: string | null
): Notice | null {
  if (isEdge || organization?.status !== "trial" || !organization.trial_ends_at) return null;
  const days = trialDaysLeft(organization.trial_ends_at, now);
  if (days === null) return null;
  const mailto = supportMailto(email, "Continuar después de la prueba");
  if (days === 0) return { message: TRIAL_EXPIRED_MESSAGE, mailto, email };
  const message = days === 1 ? "Te queda 1 día de prueba gratis." : `Te quedan ${days} días de prueba gratis.`;
  return { message, mailto, email };
}
