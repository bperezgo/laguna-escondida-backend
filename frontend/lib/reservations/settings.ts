import type {
  OpeningWindow,
  ReservationSettings,
  ReservationSettingsPatch,
} from "../../types/organizationSettings.ts";
import { sortSchedule, validateSchedule, type ScheduleError } from "../deliveries/schedule.ts";

export const MAX_PARTY_SIZE = 50;
export const MAX_NOTICE_MINUTES = 7 * 24 * 60;

export type PolicyField = "max_party_size" | "min_notice_minutes" | "daily_guest_cap";

/** The form's state: numbers stay strings so a half-typed value isn't coerced. */
export interface PolicyDraft {
  enabled: boolean;
  maxPartySize: string;
  minNoticeMinutes: string;
  dailyGuestCap: string;
  schedule: OpeningWindow[];
}

export interface PolicyErrors {
  fields: Partial<Record<PolicyField, string>>;
  schedule: ScheduleError[];
}

export function draftFrom(settings: ReservationSettings): PolicyDraft {
  return {
    enabled: settings.auto_confirm_enabled,
    maxPartySize: String(settings.max_party_size),
    minNoticeMinutes: String(settings.min_notice_minutes),
    dailyGuestCap: settings.daily_guest_cap === null ? "" : String(settings.daily_guest_cap),
    schedule: sortSchedule(settings.schedule),
  };
}

function wholeNumber(value: string): number | null {
  const trimmed = value.trim();
  return /^\d+$/.test(trimmed) ? Number(trimmed) : null;
}

/** Same rules as the backend, for feedback before saving; the backend stays authoritative. */
export function validatePolicy(draft: PolicyDraft, saved: ReservationSettings): PolicyErrors {
  const fields: PolicyErrors["fields"] = {};

  const party = wholeNumber(draft.maxPartySize);
  if (party === null || party < 1 || party > MAX_PARTY_SIZE) {
    fields.max_party_size = `Entre 1 y ${MAX_PARTY_SIZE} personas`;
  }

  const notice = wholeNumber(draft.minNoticeMinutes);
  if (notice === null || notice > MAX_NOTICE_MINUTES) {
    fields.min_notice_minutes = `Entre 0 y ${MAX_NOTICE_MINUTES} minutos (una semana)`;
  }

  if (draft.dailyGuestCap.trim() === "") {
    if (draft.enabled) {
      fields.daily_guest_cap = "Obligatorio para confirmar automáticamente";
    } else if (saved.daily_guest_cap !== null) {
      // The API can't clear a saved cap; turning the policy off is how to stop using it.
      fields.daily_guest_cap = "Ya hay un cupo guardado: escribe un número";
    }
  } else {
    const guestCap = wholeNumber(draft.dailyGuestCap);
    if (guestCap === null || guestCap < 1) fields.daily_guest_cap = "Un número de personas mayor que 0";
  }

  return { fields, schedule: validateSchedule(sortSchedule(draft.schedule)) };
}

export function hasErrors(errors: PolicyErrors): boolean {
  return Object.keys(errors.fields).length > 0 || errors.schedule.length > 0;
}

function sameSchedule(a: OpeningWindow[], b: OpeningWindow[]): boolean {
  const key = (s: OpeningWindow[]) =>
    JSON.stringify(sortSchedule(s).map(({ day, opens, closes }) => [day, opens, closes]));
  return key(a) === key(b);
}

/** Only what changed, so an untouched field is never re-validated or overwritten. */
export function patchFrom(draft: PolicyDraft, saved: ReservationSettings): ReservationSettingsPatch {
  const patch: ReservationSettingsPatch = {};
  if (draft.enabled !== saved.auto_confirm_enabled) patch.auto_confirm_enabled = draft.enabled;

  const party = wholeNumber(draft.maxPartySize);
  if (party !== null && party !== saved.max_party_size) patch.max_party_size = party;

  const notice = wholeNumber(draft.minNoticeMinutes);
  if (notice !== null && notice !== saved.min_notice_minutes) patch.min_notice_minutes = notice;

  const guestCap = wholeNumber(draft.dailyGuestCap);
  if (guestCap !== null && guestCap !== saved.daily_guest_cap) patch.daily_guest_cap = guestCap;

  if (!sameSchedule(draft.schedule, saved.schedule)) {
    patch.schedule = sortSchedule(draft.schedule).map(({ day, opens, closes }) => ({ day, opens, closes }));
  }
  return patch;
}

export function isEmptyPatch(patch: ReservationSettingsPatch): boolean {
  return Object.keys(patch).length === 0;
}

/** Reads the backend's field, e.g. "reservations.schedule[2]" or "reservations.daily_guest_cap". */
export function parsePolicyErrorField(
  field: string | undefined,
): { field: PolicyField } | { scheduleIndex: number | null } | null {
  const scalar = /^reservations\.(max_party_size|min_notice_minutes|daily_guest_cap)$/.exec(field ?? "");
  if (scalar) return { field: scalar[1] as PolicyField };
  const schedule = /^reservations\.schedule(?:\[(\d+)\])?$/.exec(field ?? "");
  if (schedule) return { scheduleIndex: schedule[1] === undefined ? null : Number(schedule[1]) };
  return null;
}

/** "2 horas", "90 minutos", "1 día y 3 horas". */
export function describeNotice(minutes: number): string {
  if (minutes === 0) return "sin anticipación mínima";
  const days = Math.floor(minutes / (24 * 60));
  const hours = Math.floor((minutes % (24 * 60)) / 60);
  const mins = minutes % 60;
  if (days === 0 && hours === 0) return `${mins} minutos`;
  const parts: string[] = [];
  if (days > 0) parts.push(days === 1 ? "1 día" : `${days} días`);
  if (hours > 0) parts.push(hours === 1 ? "1 hora" : `${hours} horas`);
  if (mins > 0) parts.push(`${mins} minutos`);
  return parts.length === 1 ? parts[0] : `${parts.slice(0, -1).join(", ")} y ${parts[parts.length - 1]}`;
}

/** One line for staff: what the agent confirms on its own right now. */
export function describePolicy(settings: ReservationSettings): string {
  if (!settings.auto_confirm_enabled) {
    return "Apagada: toda reserva por WhatsApp espera a que el equipo la confirme.";
  }
  const hours = settings.schedule.length === 0 ? "a cualquier hora" : "dentro del horario de abajo";
  return (
    `Se confirman solas las reservas de hasta ${settings.max_party_size} personas, ` +
    `con al menos ${describeNotice(settings.min_notice_minutes)} de anticipación, ${hours}, ` +
    `mientras el día no pase de ${settings.daily_guest_cap ?? "—"} invitados confirmados.`
  );
}
