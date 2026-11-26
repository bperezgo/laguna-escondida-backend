export interface Notice {
  message: string;
  /** Null when no support email is configured: the message shows without a link. */
  mailto: string | null;
  email: string | null;
}

export function supportEmail(raw: string | undefined = process.env.NEXT_PUBLIC_SUPPORT_EMAIL): string | null {
  const email = raw?.trim();
  return email ? email : null;
}

export function supportMailto(email: string | null, subject?: string): string | null {
  if (!email) return null;
  return subject ? `mailto:${email}?subject=${encodeURIComponent(subject)}` : `mailto:${email}`;
}

export const TRIAL_EXPIRED_MESSAGE = "Tu prueba terminó. Escríbenos para continuar";

export function trialExpiredNotice(email: string | null): Notice {
  return { message: TRIAL_EXPIRED_MESSAGE, mailto: supportMailto(email, "Continuar después de la prueba"), email };
}
