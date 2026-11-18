import { todayInBusinessTz } from "../format.ts";
import type {
  Reservation,
  ReservationAction,
  ReservationSource,
  ReservationStatus,
} from "../../types/reservation.ts";

export const STATUS_LABEL: Record<ReservationStatus, string> = {
  requested: "Por confirmar",
  confirmed: "Confirmada",
  visited: "Llegó",
  declined: "Rechazada",
  cancelled: "Cancelada",
  no_show: "No llegó",
};

export const SOURCE_LABEL: Record<ReservationSource, string> = {
  whatsapp: "WhatsApp",
  phone: "Teléfono",
  walk_in: "En el local",
  staff: "Personal",
};

export type StatusFilter = "all" | "requested" | "confirmed" | "visited" | "closed";

export const FILTER_LABEL: Record<StatusFilter, string> = {
  all: "Todas",
  requested: "Por confirmar",
  confirmed: "Confirmadas",
  visited: "Llegaron",
  closed: "No llegaron o canceladas",
};

const CLOSED: readonly ReservationStatus[] = ["declined", "cancelled", "no_show"];

export function matchesFilter(reservation: Reservation, filter: StatusFilter): boolean {
  if (filter === "all") return true;
  if (filter === "closed") return CLOSED.includes(reservation.status);
  return reservation.status === filter;
}

const DAY = /^\d{4}-\d{2}-\d{2}$/;

export function isDay(value: string | null | undefined): value is string {
  if (!value || !DAY.test(value)) return false;
  const parsed = new Date(`${value}T00:00:00Z`);
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().startsWith(value);
}

/** The business day (YYYY-MM-DD, organization timezone) a reservation starts on. */
export function businessDayOf(startsAt: string): string {
  return todayInBusinessTz(new Date(startsAt));
}

export function shiftDay(day: string, delta: number): string {
  const date = new Date(`${day}T12:00:00Z`);
  date.setUTCDate(date.getUTCDate() + delta);
  return date.toISOString().slice(0, 10);
}

function byStart(a: Reservation, b: Reservation): number {
  return a.starts_at === b.starts_at
    ? a.created_at.localeCompare(b.created_at)
    : new Date(a.starts_at).getTime() - new Date(b.starts_at).getTime();
}

export function sortReservations(list: Reservation[]): Reservation[] {
  return [...list].sort(byStart);
}

/**
 * Applies a live change to the list shown for `day`: adds or replaces it when it starts that day,
 * and drops it otherwise. A change older than the one already shown is ignored.
 */
export function applyChange(list: Reservation[], changed: Reservation, day: string): Reservation[] {
  const current = list.find((r) => r.id === changed.id);
  if (current && new Date(current.updated_at) > new Date(changed.updated_at)) return list;
  const others = list.filter((r) => r.id !== changed.id);
  if (businessDayOf(changed.starts_at) !== day) return others;
  return sortReservations([...others, changed]);
}

/** How many past days of unanswered requests the queue keeps showing. */
export const OVERDUE_DAYS = 7;

/**
 * Keeps the queue of requests waiting for staff, every day from `fromDay` on, current with a change:
 * a request is added or replaced, anything else leaves the queue.
 */
export function applyPendingChange(list: Reservation[], changed: Reservation, fromDay: string): Reservation[] {
  return applyStatusChange(list, changed, fromDay, ["requested"]);
}

/** The statuses the "Todas" view lists: everything still ahead, answered or not. */
export const ACTIVE_STATUSES: readonly ReservationStatus[] = ["requested", "confirmed"];

/** Keeps the list of pending and confirmed reservations, every day from `fromDay` on, current with a change. */
export function applyActiveChange(list: Reservation[], changed: Reservation, fromDay: string): Reservation[] {
  return applyStatusChange(list, changed, fromDay, ACTIVE_STATUSES);
}

function applyStatusChange(
  list: Reservation[],
  changed: Reservation,
  fromDay: string,
  statuses: readonly ReservationStatus[]
): Reservation[] {
  const current = list.find((r) => r.id === changed.id);
  if (current && new Date(current.updated_at) > new Date(changed.updated_at)) return list;
  const others = list.filter((r) => r.id !== changed.id);
  if (!statuses.includes(changed.status) || businessDayOf(changed.starts_at) < fromDay) return others;
  return sortReservations([...others, changed]);
}

/** What staff can do with a reservation now; mirrors the backend's lifecycle. */
export function actionsFor(reservation: Reservation, now: number): ReservationAction[] {
  const started = new Date(reservation.starts_at).getTime() <= now;
  switch (reservation.status) {
    case "requested":
      // Once its time has come, a request is closed by what happened: the guest came or not.
      return started ? ["confirm", "no-show"] : ["confirm", "decline", "cancel"];
    case "confirmed":
      // Marking a no-show before the time would be a mistake the guest pays for.
      return started ? ["no-show", "cancel"] : ["cancel"];
    case "visited":
      return reservation.feedback_requested ? [] : ["no-show"];
    default:
      return [];
  }
}

/** A request whose time has come without an answer; changing it no longer messages the guest. */
export function isOverdue(reservation: Reservation, now: number): boolean {
  return reservation.status === "requested" && new Date(reservation.starts_at).getTime() <= now;
}

export interface DaySummary {
  reservations: number;
  pending: number;
  /** Guests expected or seated: confirmed and visited reservations. */
  guests: number;
}

export function summarize(list: Reservation[]): DaySummary {
  return list.reduce<DaySummary>(
    (sum, r) => ({
      reservations: sum.reservations + (CLOSED.includes(r.status) ? 0 : 1),
      pending: sum.pending + (r.status === "requested" ? 1 : 0),
      guests: sum.guests + (r.status === "confirmed" || r.status === "visited" ? r.party_size : 0),
    }),
    { reservations: 0, pending: 0, guests: 0 }
  );
}

const pad = (n: number) => String(n).padStart(2, "0");

function offsetMinutes(instant: number, timeZone: string): number {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).formatToParts(new Date(instant));
  const part = (type: Intl.DateTimeFormatPartTypes) =>
    Number(parts.find((p) => p.type === type)?.value ?? 0);
  const wall = Date.UTC(part("year"), part("month") - 1, part("day"), part("hour"), part("minute"), part("second"));
  return Math.round((wall - instant) / 60_000);
}

/** A wall-clock day and "HH:MM" in `timeZone`, as RFC 3339 with that zone's offset then. */
export function startsAtInZone(day: string, time: string, timeZone: string): string {
  const [year, month, date] = day.split("-").map(Number);
  const [hour, minute] = time.split(":").map(Number);
  const wall = Date.UTC(year, month - 1, date, hour, minute);
  // Twice, so a day that changes offset (DST) uses the offset in force at that time.
  const first = offsetMinutes(wall, timeZone);
  const offset = offsetMinutes(wall - first * 60_000, timeZone);
  const sign = offset < 0 ? "-" : "+";
  const abs = Math.abs(offset);
  return `${day}T${pad(hour)}:${pad(minute)}:00${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`;
}

/** Bookings start in the future and at most 60 days ahead (the backend's limits). */
export const MAX_DAYS_AHEAD = 60;
