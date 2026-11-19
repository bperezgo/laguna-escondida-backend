import type {
  NotificationChannel,
  NotificationConditions,
  NotificationConfirmedBy,
  NotificationEvent,
  NotificationOrderType,
  NotificationReservationSource,
  NotificationRule,
} from "../../types/notifications.ts";

export const NOTIFICATIONS_DISABLED = "NOTIFICATIONS_DISABLED";
export const NOTIFICATION_CHANNEL_UNREACHABLE = "NOTIFICATION_CHANNEL_UNREACHABLE";
export const TITLE_PREFIX_MAX = 100;

export const EVENTS: readonly NotificationEvent[] = [
  "order.created",
  "order.paid",
  "reservation.requested",
  "reservation.confirmed",
];

export const EVENT_LABELS: Record<NotificationEvent, string> = {
  "order.created": "Pedido creado",
  "order.paid": "Pedido pagado",
  "reservation.requested": "Reserva por confirmar",
  "reservation.confirmed": "Reserva confirmada",
};

export const ORDER_TYPE_LABELS: Record<NotificationOrderType, string> = {
  dine_in: "En mesa",
  takeaway: "Para llevar",
  delivery: "Domicilio",
};

export const SOURCE_LABELS: Record<NotificationReservationSource, string> = {
  whatsapp: "WhatsApp",
  phone: "Teléfono",
  walk_in: "Sin reserva",
  staff: "Personal",
};

export const CONFIRMED_BY_LABELS: Record<NotificationConfirmedBy, string> = {
  staff: "Por el personal",
  auto: "Automáticamente",
};

export type ConditionName = keyof NotificationConditions;

export const CONDITION_LABELS: Record<ConditionName, string> = {
  order_types: "Tipo de pedido",
  sources: "Origen",
  confirmed_by: "Confirmada por",
  min_party_size: "Mínimo de personas",
};

// Mirrors the backend's event catalogue; the server still validates and names the field.
export const EVENT_CONDITIONS: Record<NotificationEvent, readonly ConditionName[]> = {
  "order.created": ["order_types"],
  "order.paid": ["order_types"],
  "reservation.requested": ["sources", "min_party_size"],
  "reservation.confirmed": ["sources", "confirmed_by", "min_party_size"],
};

export const DEFAULT_EXPIRY_MINUTES: Record<NotificationEvent, number> = {
  "order.created": 60,
  "order.paid": 60,
  "reservation.requested": 720,
  "reservation.confirmed": 720,
};

export function eventLabel(event: string): string {
  return EVENT_LABELS[event as NotificationEvent] ?? event;
}

function labelsOf<K extends string>(values: readonly K[] | undefined, labels: Record<K, string>): string[] {
  return (values ?? []).map((v) => labels[v] ?? v);
}

/** "Reserva por confirmar · WhatsApp · Mínimo de personas: 6" */
export function ruleSummary(rule: Pick<NotificationRule, "event" | "conditions">): string {
  const c = rule.conditions ?? {};
  const parts = [
    eventLabel(rule.event),
    ...labelsOf(c.order_types, ORDER_TYPE_LABELS),
    ...labelsOf(c.sources, SOURCE_LABELS),
    ...labelsOf(c.confirmed_by, CONFIRMED_BY_LABELS),
  ];
  if (c.min_party_size) parts.push(`${CONDITION_LABELS.min_party_size}: ${c.min_party_size}`);
  return parts.join(" · ");
}

export function expiryLabel(minutes: number): string {
  if (minutes % 1440 === 0) return minutes === 1440 ? "1 día" : `${minutes / 1440} días`;
  if (minutes % 60 === 0) return minutes === 60 ? "1 hora" : `${minutes / 60} horas`;
  return minutes === 1 ? "1 minuto" : `${minutes} minutos`;
}

/** The backend sends "…1234"; the page shows "••••1234" like the integration hints. */
export function webhookHint(hint: string | undefined): string {
  return `••••${(hint ?? "").replace(/^…/, "")}`;
}

/** Keeps only the conditions the event has, dropping empty lists and a blank party size. */
export function conditionsForEvent(event: NotificationEvent, conditions: NotificationConditions): NotificationConditions {
  const allowed = EVENT_CONDITIONS[event];
  const out: NotificationConditions = {};
  if (allowed.includes("order_types") && conditions.order_types?.length) out.order_types = conditions.order_types;
  if (allowed.includes("sources") && conditions.sources?.length) out.sources = conditions.sources;
  if (allowed.includes("confirmed_by") && conditions.confirmed_by?.length) out.confirmed_by = conditions.confirmed_by;
  if (allowed.includes("min_party_size") && conditions.min_party_size) out.min_party_size = conditions.min_party_size;
  return out;
}

export type NotificationsPageState =
  | { kind: "unavailable" }
  | { kind: "error"; message: string }
  | { kind: "ready"; channels: NotificationChannel[]; rulesByChannel: Record<string, NotificationRule[]> };

export interface NotificationsLoadPart<T> {
  status: number;
  code?: string;
  data?: T;
}

export interface NotificationsLoad {
  channels: NotificationsLoadPart<NotificationChannel[]>;
  rules: NotificationsLoadPart<NotificationRule[]>;
}

function loadError(part: NotificationsLoadPart<unknown>): NotificationsPageState | null {
  if (part.status === 503 && part.code === NOTIFICATIONS_DISABLED) return { kind: "unavailable" };
  if (part.status === 403) return { kind: "error", message: "No tienes permiso para ver las notificaciones." };
  if (part.status === 404) return { kind: "error", message: "Las notificaciones solo se administran en la nube." };
  if (part.status < 200 || part.status >= 300 || !part.data) {
    return { kind: "error", message: "Error al cargar las notificaciones." };
  }
  return null;
}

export function notificationsPageState(load: NotificationsLoad): NotificationsPageState {
  const failed = loadError(load.channels) ?? loadError(load.rules);
  if (failed) return failed;
  const channels = load.channels.data ?? [];
  const rulesByChannel: Record<string, NotificationRule[]> = {};
  for (const channel of channels) rulesByChannel[channel.id] = [];
  for (const rule of load.rules.data ?? []) {
    rulesByChannel[rule.channel_id]?.push(rule);
  }
  return { kind: "ready", channels, rulesByChannel };
}

const ROLE_MENTION = /^<@&(\d+)>\s?([\s\S]*)$/;

export function composeTitlePrefix(roleId: string, text: string): string {
  const id = roleId.trim();
  const rest = text.trim();
  if (!id) return rest;
  return rest ? `<@&${id}> ${rest}` : `<@&${id}>`;
}

export function parseTitlePrefix(prefix: string): { roleId: string; text: string } {
  const match = (prefix ?? "").match(ROLE_MENTION);
  if (!match) return { roleId: "", text: prefix ?? "" };
  return { roleId: match[1], text: match[2] };
}

export function validateRoleId(roleId: string): string | undefined {
  const id = roleId.trim();
  if (id === "" || /^\d+$/.test(id)) return undefined;
  return "El ID del rol debe tener solo dígitos.";
}

/** The backend counts characters (runes), not UTF-16 units. */
export function charCount(text: string): number {
  return Array.from(text).length;
}

/** How many characters the free text may use once `<@&id> ` takes its share of the 100. */
export function freeTextBudget(roleId: string): number {
  const id = roleId.trim();
  return id ? TITLE_PREFIX_MAX - charCount(`<@&${id}> `) : TITLE_PREFIX_MAX;
}

export interface NotificationFormErrors {
  fields: Record<string, string>;
  form?: string;
}

export function hasNotificationErrors(errors: NotificationFormErrors): boolean {
  return Boolean(errors.form) || Object.keys(errors.fields).length > 0;
}

function fieldMessage(field: string, message: string): string {
  switch (field) {
    case "name":
      if (/already used/.test(message)) return "Ya hay un canal con este nombre.";
      return "El nombre es obligatorio y puede tener hasta 80 caracteres.";
    case "type":
      return "El único tipo de canal es Discord.";
    case "settings.webhook_url":
      if (/is required/.test(message)) return "El webhook es obligatorio.";
      return "Debe ser un webhook de Discord: https://discord.com/api/webhooks/…";
    case "event":
      return "Elige un evento.";
    case "channel_id":
      return "El canal no existe o es de otra organización.";
    case "title_prefix":
      return `El prefijo puede tener hasta ${TITLE_PREFIX_MAX} caracteres, contando la mención del rol.`;
    case "expires_after_minutes":
      return "Debe estar entre 1 y 10080 minutos (7 días).";
    case "conditions.min_party_size":
      return "Debe ser un número entero mayor que cero.";
  }
  if (field.startsWith("conditions.")) {
    if (/not a condition/.test(message)) return "Este evento no tiene esta condición.";
    return "Uno de los valores no es válido.";
  }
  return message;
}

/** Puts a refused save next to the field the backend names, or on the form when it names none. */
export function notificationSaveErrors(
  status: number,
  message: string,
  code?: string,
  field?: string,
): NotificationFormErrors {
  if (status === 400 && field) return { fields: { [field]: fieldMessage(field, message) } };
  if (status === 400) return { fields: {}, form: "El servidor no aceptó los datos. Revísalos e intenta de nuevo." };
  if (status === 503 && code === NOTIFICATIONS_DISABLED) {
    return { fields: {}, form: "Las notificaciones no están habilitadas en este servidor." };
  }
  if (status === 403) return { fields: {}, form: "Solo un administrador puede cambiar las notificaciones." };
  if (status === 404) return { fields: {}, form: "Ya no existe. Recarga la página." };
  return { fields: {}, form: message || "Error al guardar." };
}

export type ChannelTestResult = { ok: true; message: string } | { ok: false; message: string };

export function channelTestResult(status: number, code?: string, destinationStatus?: number): ChannelTestResult {
  if (status === 204 || (status >= 200 && status < 300)) {
    return { ok: true, message: "Mensaje de prueba enviado. Revisa el canal de Discord." };
  }
  if (status === 502 && code === NOTIFICATION_CHANNEL_UNREACHABLE) {
    return {
      ok: false,
      message: destinationStatus
        ? `Discord no aceptó el mensaje (estado ${destinationStatus}). Revisa el webhook.`
        : "No se pudo contactar a Discord. Intenta de nuevo.",
    };
  }
  const { form } = notificationSaveErrors(status, "No se pudo enviar el mensaje de prueba.", code);
  return { ok: false, message: form ?? "No se pudo enviar el mensaje de prueba." };
}
