import { shiftDay } from "../reservations/reservations.ts";
import type { Feedback } from "../../types/feedback.ts";

export type Period = "week" | "month";

export interface Range {
  from: string;
  /** Inclusive. */
  to: string;
}

/** The week (Monday to Sunday) or calendar month holding `day`, as the backend's week default. */
export function rangeFor(day: string, period: Period): Range {
  if (period === "month") {
    const [year, month] = day.split("-").map(Number);
    const last = new Date(Date.UTC(year, month, 0)).getUTCDate();
    const prefix = day.slice(0, 8);
    return { from: `${prefix}01`, to: `${prefix}${String(last).padStart(2, "0")}` };
  }
  const weekday = new Date(`${day}T12:00:00Z`).getUTCDay();
  const from = shiftDay(day, -((weekday + 6) % 7));
  return { from, to: shiftDay(from, 6) };
}

/** The range before or after `range`'s start, of the same period. */
export function shiftRange(range: Range, period: Period, delta: number): Range {
  if (period === "week") return rangeFor(shiftDay(range.from, 7 * delta), period);
  const [year, month] = range.from.split("-").map(Number);
  const first = new Date(Date.UTC(year, month - 1 + delta, 1)).toISOString().slice(0, 10);
  return rangeFor(first, period);
}

export type RatingFilter = "all" | "attention" | "good";

export const RATING_FILTER_LABEL: Record<RatingFilter, string> = {
  all: "Todas",
  attention: "Requieren atención",
  good: "Buenas (4–5)",
};

export function matchesRating(item: Feedback, filter: RatingFilter): boolean {
  if (filter === "all") return true;
  return filter === "attention" ? item.needs_attention : !item.needs_attention;
}

export interface FeedbackSummary {
  total: number;
  /** null with no answers. */
  average: number | null;
  attention: number;
  /** Count per rating, index 0 is one star. */
  distribution: [number, number, number, number, number];
}

export function summarizeFeedback(list: Feedback[]): FeedbackSummary {
  const distribution: FeedbackSummary["distribution"] = [0, 0, 0, 0, 0];
  let sum = 0;
  let attention = 0;
  for (const item of list) {
    if (item.rating >= 1 && item.rating <= 5) distribution[item.rating - 1] += 1;
    sum += item.rating;
    if (item.needs_attention) attention += 1;
  }
  return {
    total: list.length,
    average: list.length === 0 ? null : Math.round((sum / list.length) * 10) / 10,
    attention,
    distribution,
  };
}

/** A wa.me link for an E.164 phone. */
export function whatsappLink(phone: string): string {
  return `https://wa.me/${phone.replace(/\D/g, "")}`;
}
