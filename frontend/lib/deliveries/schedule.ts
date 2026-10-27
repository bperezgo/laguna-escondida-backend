import type {
  Availability,
  DayCode,
  OpeningWindow,
  OrderingType,
} from "@/types/organizationSettings";

export const DAYS: { code: DayCode; label: string }[] = [
  { code: "mon", label: "Lunes" },
  { code: "tue", label: "Martes" },
  { code: "wed", label: "Miércoles" },
  { code: "thu", label: "Jueves" },
  { code: "fri", label: "Viernes" },
  { code: "sat", label: "Sábado" },
  { code: "sun", label: "Domingo" },
];

export const ORDERING_TYPE_LABELS: Record<OrderingType, string> = {
  delivery: "Domicilios",
  takeaway: "Para llevar",
};

export const MAX_WINDOWS_PER_TYPE = 21;

const MINUTES_PER_DAY = 24 * 60;
const MINUTES_PER_WEEK = 7 * MINUTES_PER_DAY;
const CLOCK = /^([01]\d|2[0-3]):([0-5]\d)$/;

export function parseClock(value: string): number | null {
  const match = CLOCK.exec(value);
  return match ? Number(match[1]) * 60 + Number(match[2]) : null;
}

export function isPastMidnight(window: OpeningWindow): boolean {
  const opens = parseClock(window.opens);
  const closes = parseClock(window.closes);
  return opens !== null && closes !== null && closes < opens;
}

function dayIndex(day: DayCode): number {
  return DAYS.findIndex((d) => d.code === day);
}

/** Sorted by day, then opening time: the order the backend's error indexes refer to once saved. */
export function sortSchedule<T extends OpeningWindow>(schedule: T[]): T[] {
  return [...schedule].sort(
    (a, b) =>
      dayIndex(a.day) - dayIndex(b.day) ||
      (parseClock(a.opens) ?? 0) - (parseClock(b.opens) ?? 0),
  );
}

interface WeekInterval {
  start: number;
  end: number;
}

function weekInterval(window: OpeningWindow, opens: number, closes: number): WeekInterval {
  const start = dayIndex(window.day) * MINUTES_PER_DAY + opens;
  const length = closes > opens ? closes - opens : closes + MINUTES_PER_DAY - opens;
  return { start, end: start + length };
}

function overlaps(a: WeekInterval, b: WeekInterval): boolean {
  return [-MINUTES_PER_WEEK, 0, MINUTES_PER_WEEK].some(
    (shift) => a.start < b.end + shift && b.start + shift < a.end,
  );
}

export interface ScheduleError {
  /** Index into the schedule, or null for an error about the whole list. */
  index: number | null;
  message: string;
}

/** Same rules as the backend, for feedback before saving; the backend stays authoritative. */
export function validateSchedule(schedule: OpeningWindow[]): ScheduleError[] {
  const errors: ScheduleError[] = [];
  if (schedule.length > MAX_WINDOWS_PER_TYPE) {
    errors.push({ index: null, message: `Máximo ${MAX_WINDOWS_PER_TYPE} franjas` });
  }
  const parsed: { index: number; interval: WeekInterval }[] = [];
  schedule.forEach((window, index) => {
    if (dayIndex(window.day) < 0) {
      errors.push({ index, message: "Día inválido" });
      return;
    }
    const opens = parseClock(window.opens);
    const closes = parseClock(window.closes);
    if (opens === null || closes === null) {
      errors.push({ index, message: "Usa horas de 24 horas, HH:MM" });
      return;
    }
    if (opens === closes) {
      errors.push({ index, message: "La apertura y el cierre deben ser distintos" });
      return;
    }
    const interval = weekInterval(window, opens, closes);
    const clash = parsed.find((p) => overlaps(p.interval, interval));
    if (clash) {
      const other = schedule[clash.index];
      errors.push({
        index,
        message: `Se cruza con ${dayLabel(other.day)} ${other.opens}–${other.closes}`,
      });
    }
    parsed.push({ index, interval });
  });
  return errors;
}

export function dayLabel(day: DayCode): string {
  return DAYS.find((d) => d.code === day)?.label ?? day;
}

/** Reads the backend's field, e.g. "ordering.takeaway.schedule[2]", to point at the window. */
export function parseSettingsErrorField(
  field: string | undefined,
): { type: OrderingType; index: number | null } | null {
  const match = /^ordering\.(delivery|takeaway)\.schedule(?:\[(\d+)\])?$/.exec(field ?? "");
  if (!match) return null;
  return {
    type: match[1] as OrderingType,
    index: match[2] === undefined ? null : Number(match[2]),
  };
}

function formatInstant(iso: string, timezone: string, now: Date): string {
  const instant = new Date(iso);
  const time = new Intl.DateTimeFormat("es-CO", {
    timeZone: timezone,
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(instant);
  const dayKey = (d: Date) =>
    new Intl.DateTimeFormat("en-CA", { timeZone: timezone }).format(d);
  const tomorrow = new Date(now.getTime() + MINUTES_PER_DAY * 60_000);
  if (dayKey(instant) === dayKey(now)) return `hoy a las ${time}`;
  if (dayKey(instant) === dayKey(tomorrow)) return `mañana a las ${time}`;
  const weekday = new Intl.DateTimeFormat("es-CO", { timeZone: timezone, weekday: "long" }).format(
    instant,
  );
  return `el ${weekday} a las ${time}`;
}

/** The "Ahora" line, from the backend's availability; nothing is computed client-side. */
export function describeAvailability(
  availability: Availability,
  timezone: string,
  now: Date = new Date(),
): string {
  if (availability.available) {
    return availability.closes_at
      ? `Recibiendo pedidos hasta ${formatInstant(availability.closes_at, timezone, now)}`
      : "Recibiendo pedidos (sin horario: abierto siempre)";
  }
  if (availability.reason === "paused") {
    return availability.paused_reason
      ? `Pausado: ${availability.paused_reason}`
      : "Pausado";
  }
  return availability.next_opens_at
    ? `Cerrado, abre ${formatInstant(availability.next_opens_at, timezone, now)}`
    : "Cerrado, sin apertura en los próximos 7 días";
}
