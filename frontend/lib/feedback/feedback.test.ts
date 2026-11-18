import { test } from "node:test";
import assert from "node:assert/strict";
import type { Feedback } from "../../types/feedback.ts";
import { matchesRating, rangeFor, shiftRange, summarizeFeedback, whatsappLink } from "./feedback.ts";

const feedback = (overrides: Partial<Feedback>): Feedback => ({
  id: "f1",
  feedback_request_id: "fr1",
  contact_id: "c1",
  contact_phone: "+573005551234",
  rating: 5,
  needs_attention: false,
  reservation: { id: "r1", name: "Ana", party_size: 4, starts_at: "2026-11-08T01:00:00Z" },
  created_at: "2026-11-08T16:00:00Z",
  updated_at: "2026-11-08T16:00:00Z",
  ...overrides,
});

test("rangeFor week runs Monday to Sunday", () => {
  assert.deepEqual(rangeFor("2026-11-09", "week"), { from: "2026-11-09", to: "2026-11-15" });
  assert.deepEqual(rangeFor("2026-11-15", "week"), { from: "2026-11-09", to: "2026-11-15" });
  assert.deepEqual(rangeFor("2026-11-11", "week"), { from: "2026-11-09", to: "2026-11-15" });
});

test("rangeFor week crosses a month and a year", () => {
  assert.deepEqual(rangeFor("2027-01-01", "week"), { from: "2026-12-28", to: "2027-01-03" });
});

test("rangeFor month covers the calendar month, leap years included", () => {
  assert.deepEqual(rangeFor("2026-11-09", "month"), { from: "2026-11-01", to: "2026-11-30" });
  assert.deepEqual(rangeFor("2028-02-10", "month"), { from: "2028-02-01", to: "2028-02-29" });
  assert.deepEqual(rangeFor("2026-12-31", "month"), { from: "2026-12-01", to: "2026-12-31" });
});

test("shiftRange moves by whole weeks and months", () => {
  const week = rangeFor("2026-11-09", "week");
  assert.deepEqual(shiftRange(week, "week", -1), { from: "2026-11-02", to: "2026-11-08" });
  assert.deepEqual(shiftRange(week, "week", 1), { from: "2026-11-16", to: "2026-11-22" });
  const month = rangeFor("2026-12-05", "month");
  assert.deepEqual(shiftRange(month, "month", 1), { from: "2027-01-01", to: "2027-01-31" });
  assert.deepEqual(shiftRange(rangeFor("2026-03-31", "month"), "month", -1), {
    from: "2026-02-01",
    to: "2026-02-28",
  });
});

test("matchesRating splits on needs_attention", () => {
  const bad = feedback({ rating: 2, needs_attention: true });
  const good = feedback({ rating: 4 });
  assert.equal(matchesRating(bad, "all"), true);
  assert.equal(matchesRating(bad, "attention"), true);
  assert.equal(matchesRating(bad, "good"), false);
  assert.equal(matchesRating(good, "attention"), false);
  assert.equal(matchesRating(good, "good"), true);
});

test("summarizeFeedback averages to one decimal and counts each rating", () => {
  const summary = summarizeFeedback([
    feedback({ id: "a", rating: 5 }),
    feedback({ id: "b", rating: 4 }),
    feedback({ id: "c", rating: 2, needs_attention: true }),
  ]);
  assert.equal(summary.total, 3);
  assert.equal(summary.average, 3.7);
  assert.equal(summary.attention, 1);
  assert.deepEqual(summary.distribution, [0, 1, 0, 1, 1]);
});

test("summarizeFeedback has no average without answers", () => {
  assert.deepEqual(summarizeFeedback([]), {
    total: 0,
    average: null,
    attention: 0,
    distribution: [0, 0, 0, 0, 0],
  });
});

test("whatsappLink keeps only digits", () => {
  assert.equal(whatsappLink("+573005551234"), "https://wa.me/573005551234");
});
