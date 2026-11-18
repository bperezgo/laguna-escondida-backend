import type { Attribution } from "./attribution.ts";

export const FIELDS = ["name", "restaurant_name", "phone", "consent"] as const;
export type FieldName = (typeof FIELDS)[number];

export interface FormValues {
  name: string;
  restaurant_name: string;
  phone: string;
  consent: boolean;
  /** Honeypot: a person never fills it. */
  website: string;
}

export type FieldErrors = Partial<Record<FieldName, string>>;

export type LeadRequest = {
  name: string;
  restaurant_name: string;
  phone: string;
  consent: boolean;
  website: string;
  event_id: string;
} & Attribution;

export type SubmitResult =
  | { kind: "success" }
  | { kind: "field"; field: FieldName; message: string }
  | { kind: "error" };

const MESSAGES: Record<FieldName, string> = {
  name: "Escribe tu nombre.",
  restaurant_name: "Escribe el nombre de tu restaurante.",
  phone: "Escribe tu número de WhatsApp, con o sin el indicativo del país.",
  consent: "Acepta la política de privacidad para que podamos contactarte.",
};

export function validate(values: FormValues): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.name.trim()) errors.name = MESSAGES.name;
  if (!values.restaurant_name.trim()) errors.restaurant_name = MESSAGES.restaurant_name;
  if (values.phone.replace(/\D/g, "").length < 7) errors.phone = MESSAGES.phone;
  if (!values.consent) errors.consent = MESSAGES.consent;
  return errors;
}

export function buildRequest(values: FormValues, attribution: Attribution, eventId: string): LeadRequest {
  return {
    name: values.name.trim(),
    restaurant_name: values.restaurant_name.trim(),
    phone: values.phone.trim(),
    consent: values.consent,
    website: values.website,
    event_id: eventId,
    ...attribution,
  };
}

function isField(value: unknown): value is FieldName {
  return typeof value === "string" && (FIELDS as readonly string[]).includes(value);
}

/** 201 is success; a 400 naming one of our fields points at that field; everything else, 429 included, is the general error. */
export function mapResponse(status: number, body: unknown): SubmitResult {
  if (status === 201) return { kind: "success" };
  if (status === 400 && typeof body === "object" && body !== null) {
    const field = (body as { field?: unknown }).field;
    if (isField(field)) return { kind: "field", field, message: MESSAGES[field] };
  }
  return { kind: "error" };
}

export async function submitLead(
  apiBaseUrl: string,
  request: LeadRequest,
  fetchImpl: typeof fetch = fetch,
): Promise<SubmitResult> {
  try {
    const response = await fetchImpl(`${apiBaseUrl}/api/public/leads`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(request),
      signal: AbortSignal.timeout(15_000),
    });
    const body: unknown = await response.json().catch(() => undefined);
    return mapResponse(response.status, body);
  } catch {
    return { kind: "error" };
  }
}
