import type { PaymentSettings, PaymentSettingsRequest, PaymentStrategy } from "../../types/paymentSettings.ts";

export const MIN_MANUAL_WINDOW = 10;
export const MAX_MANUAL_WINDOW = 1440;
export const MAX_INSTRUCTIONS = 1000;

export type PaymentSettingsCardState =
  | { kind: "error"; message: string }
  | { kind: "loaded"; settings: PaymentSettings };

export interface PaymentSettingsLoad {
  status: number;
  settings?: PaymentSettings;
}

export function paymentSettingsCardState(load: PaymentSettingsLoad): PaymentSettingsCardState {
  if (load.status === 403) return { kind: "error", message: "No tienes permiso para ver cómo se pagan los pedidos." };
  if (load.status === 404) return { kind: "error", message: "El pago de los pedidos solo se configura en la nube." };
  if (load.status < 200 || load.status >= 300 || !load.settings) {
    return { kind: "error", message: "Error al cargar cómo se pagan los pedidos." };
  }
  return { kind: "loaded", settings: load.settings };
}

export interface PaymentSettingsDraft {
  strategy: PaymentStrategy;
  manual_instructions: string;
  manual_window_minutes: string;
}

export interface PaymentSettingsErrors {
  manual_instructions?: string;
  manual_window_minutes?: string;
  form?: string;
}

export function draftFrom(settings: PaymentSettings): PaymentSettingsDraft {
  return {
    strategy: settings.strategy,
    manual_instructions: settings.manual_instructions,
    manual_window_minutes: String(settings.manual_window_minutes),
  };
}

/** Caught before sending so the reason is shown at once; the backend repeats the checks. */
export function validatePaymentDraft(draft: PaymentSettingsDraft): PaymentSettingsErrors {
  const errors: PaymentSettingsErrors = {};
  if (draft.strategy === "manual" && draft.manual_instructions.trim() === "") {
    errors.manual_instructions = "Escribe cómo debe hacer la transferencia el cliente.";
  }
  if (draft.manual_instructions.length > MAX_INSTRUCTIONS) {
    errors.manual_instructions = `Máximo ${MAX_INSTRUCTIONS} caracteres.`;
  }
  const window = Number(draft.manual_window_minutes);
  if (!Number.isInteger(window) || window < MIN_MANUAL_WINDOW || window > MAX_MANUAL_WINDOW) {
    errors.manual_window_minutes = `Un número entero de minutos, de ${MIN_MANUAL_WINDOW} a ${MAX_MANUAL_WINDOW}.`;
  }
  return errors;
}

export function hasPaymentErrors(errors: PaymentSettingsErrors): boolean {
  return Boolean(errors.manual_instructions || errors.manual_window_minutes || errors.form);
}

export function paymentRequest(draft: PaymentSettingsDraft): PaymentSettingsRequest {
  return {
    strategy: draft.strategy,
    manual_instructions: draft.manual_instructions.trim(),
    manual_window_minutes: Number(draft.manual_window_minutes),
  };
}

export function isPaymentDraftChanged(draft: PaymentSettingsDraft, saved: PaymentSettings): boolean {
  const request = paymentRequest(draft);
  return (
    request.strategy !== saved.strategy ||
    request.manual_instructions !== saved.manual_instructions ||
    request.manual_window_minutes !== saved.manual_window_minutes
  );
}

/** Puts a refused save next to the field the backend names, or on the form when it names none. */
export function paymentSaveErrors(status: number, message: string, field?: string): PaymentSettingsErrors {
  if (status === 400 && field === "manual_instructions") {
    return { manual_instructions: "Escribe las instrucciones de pago (máximo 1000 caracteres)." };
  }
  if (status === 400 && field === "manual_window_minutes") {
    return { manual_window_minutes: `Un número entero de minutos, de ${MIN_MANUAL_WINDOW} a ${MAX_MANUAL_WINDOW}.` };
  }
  if (status === 403) return { form: "No tienes permiso para cambiar cómo se pagan los pedidos." };
  return { form: message || "Error al guardar cómo se pagan los pedidos." };
}

export function strategyLabel(strategy: PaymentStrategy): string {
  return strategy === "manual" ? "Transferencia confirmada por el personal" : "Wompi (QR y link de pago)";
}

/** Under wompi without an integration, agent orders are refused: say so before the first customer finds out. */
export function wompiMissingNotice(strategy: PaymentStrategy, wompiConfigured: boolean): string | null {
  if (strategy !== "wompi" || wompiConfigured) return null;
  return "Wompi no está configurado: los pedidos de WhatsApp se rechazan hasta que pongas las llaves de Wompi, o elijas la transferencia.";
}
