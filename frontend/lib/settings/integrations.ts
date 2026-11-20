import type {
  GeneratedMarketingAppSecret,
  IntegrationEnvironment,
  MarketingAppIntegration,
  OrganizationIntegration,
  WompiCredentialsRequest,
  WompiIntegration,
} from "../../types/organizationIntegrations.ts";

export const INTEGRATIONS_UNAVAILABLE = "INTEGRATIONS_UNAVAILABLE";

export type WompiCardState =
  | { kind: "unavailable" }
  | { kind: "error"; message: string }
  | { kind: "not_configured" }
  | { kind: "configured"; integration: WompiIntegration };

export interface IntegrationsLoad {
  status: number;
  code?: string;
  integrations?: OrganizationIntegration[];
}

type LoadFailure = { kind: "unavailable" } | { kind: "error"; message: string };

function loadFailure(load: IntegrationsLoad): LoadFailure | null {
  if (load.status === 503 && load.code === INTEGRATIONS_UNAVAILABLE) return { kind: "unavailable" };
  if (load.status === 403) return { kind: "error", message: "No tienes permiso para ver las integraciones." };
  if (load.status === 404) return { kind: "error", message: "Las integraciones solo se administran en la nube." };
  if (load.status < 200 || load.status >= 300 || !load.integrations) {
    return { kind: "error", message: "Error al cargar las integraciones." };
  }
  return null;
}

export function wompiCardState(load: IntegrationsLoad): WompiCardState {
  const failure = loadFailure(load);
  if (failure) return failure;
  const wompi = load.integrations?.find((i): i is WompiIntegration => i.provider === "wompi");
  return wompi ? { kind: "configured", integration: wompi } : { kind: "not_configured" };
}

/** The secret lives only in "generated": never in the URL, storage or logs, and gone once dismissed. */
export type MarketingAppCardState =
  | { kind: "unavailable" }
  | { kind: "error"; message: string }
  | { kind: "not_configured" }
  | { kind: "generated"; integration: MarketingAppIntegration; secret: string }
  | { kind: "configured"; integration: MarketingAppIntegration };

export function marketingAppCardState(load: IntegrationsLoad): MarketingAppCardState {
  const failure = loadFailure(load);
  if (failure) return failure;
  const integration = load.integrations?.find((i): i is MarketingAppIntegration => i.provider === "marketing_app");
  return integration ? { kind: "configured", integration } : { kind: "not_configured" };
}

export function marketingAppGenerated(response: GeneratedMarketingAppSecret): MarketingAppCardState {
  const { signing_secret: secret, ...integration } = response;
  return { kind: "generated", integration, secret };
}

/** "Listo" drops the secret and leaves only the hint. */
export function marketingAppDismissed(state: MarketingAppCardState): MarketingAppCardState {
  return state.kind === "generated" ? { kind: "configured", integration: state.integration } : state;
}

export type MarketingAppAction = "generate" | "regenerate" | "remove";

export function marketingAppActions(state: MarketingAppCardState, canManage: boolean): MarketingAppAction[] {
  if (!canManage) return [];
  if (state.kind === "not_configured") return ["generate"];
  if (state.kind === "configured") return ["regenerate", "remove"];
  return [];
}

export const MARKETING_APP_PASTE_NOTE =
  "Pégalo en cada canal del restaurante en marketing-app, en Canal → Ajustes → «Secreto de eventos del sistema de pedidos». No se volverá a mostrar.";

export const MARKETING_APP_REGENERATE_WARNING =
  "Al generar uno nuevo, el anterior deja de servir: los eventos fallarán hasta que pegues el nuevo secreto en todos los canales. Se reintentan cada hora durante 72 horas.";

export function marketingAppErrorMessage(status: number, message: string): string {
  if (status === 503) return "Las integraciones no están habilitadas en este servidor.";
  if (status === 403) return "No tienes permiso para cambiar el secreto de marketing-app.";
  return message || "Error al cambiar el secreto de marketing-app.";
}

export type WompiField = keyof WompiCredentialsRequest;

export interface WompiFormErrors {
  private_key?: string;
  events_secret?: string;
  form?: string;
}

const FIELD_LABELS: Record<WompiField, string> = {
  private_key: "La llave privada",
  events_secret: "El secreto de eventos",
};

const FIELD_PREFIXES: Record<WompiField, string> = {
  private_key: "prv_test_ o prv_prod_",
  events_secret: "test_events_ o prod_events_",
};

export function environmentLabel(environment: IntegrationEnvironment | string): string {
  if (environment === "sandbox") return "Sandbox";
  if (environment === "production") return "Producción";
  return environment;
}

function environmentWord(environment: string): string {
  if (environment === "sandbox") return "pruebas (sandbox)";
  if (environment === "production") return "producción";
  return environment;
}

const REQUIRED: Record<WompiField, string> = {
  private_key: "La llave privada es obligatoria.",
  events_secret: "El secreto de eventos es obligatorio.",
};

/** Empty fields are caught before sending; the prefixes are left to the backend. */
export function validateWompiDraft(draft: WompiCredentialsRequest): WompiFormErrors {
  const errors: WompiFormErrors = {};
  for (const field of ["private_key", "events_secret"] as const) {
    if (draft[field].trim() === "") errors[field] = REQUIRED[field];
  }
  return errors;
}

export function hasWompiErrors(errors: WompiFormErrors): boolean {
  return Boolean(errors.private_key || errors.events_secret || errors.form);
}

/** Puts a refused save next to the field the backend names, or on the form when it names none. */
export function wompiSaveErrors(status: number, message: string, field?: string): WompiFormErrors {
  if (status === 400 && (field === "private_key" || field === "events_secret")) {
    if (/is required/.test(message)) return { [field]: REQUIRED[field] };
    return { [field]: `${FIELD_LABELS[field]} debe empezar por ${FIELD_PREFIXES[field]}.` };
  }
  if (status === 400) {
    const cloud = message.match(/keys are for (\w+) and this cloud uses Wompi's (\w+)/);
    if (cloud) {
      return {
        form: `Las llaves son de ${environmentWord(cloud[1])} y este servidor usa Wompi en ${environmentWord(cloud[2])}.`,
      };
    }
    const mixed = message.match(/private key is for (\w+) and the events secret for (\w+)/);
    if (mixed) {
      return {
        form: `La llave privada es de ${environmentWord(mixed[1])} y el secreto de eventos de ${environmentWord(mixed[2])}. Deben ser del mismo ambiente.`,
      };
    }
    return { form: "Wompi no aceptó estas llaves. Revisa que las copiaste completas." };
  }
  if (status === 503) return { form: "Las integraciones no están habilitadas en este servidor." };
  if (status === 403) return { form: "No tienes permiso para cambiar las llaves de Wompi." };
  return { form: message || "Error al guardar las llaves." };
}

export function maskedHint(hint: string): string {
  return hint ? `••••${hint}` : "••••";
}
