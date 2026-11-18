import { test } from "node:test";
import assert from "node:assert/strict";
import type { Reservation } from "../../types/reservation.ts";
import {
  actionsFor,
  applyChange,
  applyActiveChange,
  applyPendingChange,
  businessDayOf,
  isDay,
  isOverdue,
  matchesFilter,
  shiftDay,
  startsAtInZone,
  summarize,
} from "./reservations.ts";

const reservation = (overrides: Partial<Reservation>): Reservation => ({
  id: "r1",
  contact_id: "c1",
  contact_phone: "+573005551234",
  name: "Ana",
  party_size: 4,
  // 8 pm in Bogotá on 9 Nov.
  starts_at: "2026-11-10T01:00:00Z",
  source: "whatsapp",
  status: "requested",
  feedback_requested: false,
  origin: {},
  created_at: "2026-11-09T15:00:00Z",
  updated_at: "2026-11-09T15:00:00Z",
  ...overrides,
});

test("a reservation's day is the restaurant's, not UTC's", () => {
  assert.equal(businessDayOf("2026-11-10T01:00:00Z"), "2026-11-09");
  assert.equal(businessDayOf("2026-11-10T05:00:00Z"), "2026-11-10");
});

test("days move across months and years", () => {
  assert.equal(shiftDay("2026-11-30", 1), "2026-12-01");
  assert.equal(shiftDay("2027-01-01", -1), "2026-12-31");
});

test("only real calendar days are days", () => {
  assert.equal(isDay("2026-11-09"), true);
  assert.equal(isDay("2026-02-30"), false);
  assert.equal(isDay("09/11/2026"), false);
  assert.equal(isDay(null), false);
});

test("a new request for the day shown is added in time order", () => {
  const early = reservation({ id: "early", starts_at: "2026-11-09T23:00:00Z" });
  const late = reservation({ id: "late", starts_at: "2026-11-10T02:00:00Z" });

  const list = applyChange([early, late], reservation({ id: "new" }), "2026-11-09");

  assert.deepEqual(list.map((r) => r.id), ["early", "new", "late"]);
});

test("a change replaces the reservation it is about", () => {
  const list = applyChange(
    [reservation({})],
    reservation({ status: "confirmed", updated_at: "2026-11-09T16:00:00Z" }),
    "2026-11-09"
  );

  assert.equal(list.length, 1);
  assert.equal(list[0].status, "confirmed");
});

test("a reservation for another day is not shown", () => {
  assert.deepEqual(applyChange([], reservation({}), "2026-11-10"), []);
});

test("an older change never overwrites a newer one", () => {
  const shown = reservation({ status: "confirmed", updated_at: "2026-11-09T16:00:00Z" });

  const list = applyChange([shown], reservation({ status: "requested" }), "2026-11-09");

  assert.equal(list[0].status, "confirmed");
});

test("a request on a later day joins the pending queue in time order", () => {
  const today = reservation({ id: "today" });
  const later = reservation({ id: "later", starts_at: "2026-11-20T01:00:00Z" });

  const list = applyPendingChange([later], today, "2026-11-09");

  assert.deepEqual(list.map((r) => r.id), ["today", "later"]);
});

test("an answered request leaves the pending queue", () => {
  const confirmed = reservation({ status: "confirmed", updated_at: "2026-11-09T16:00:00Z" });

  assert.deepEqual(applyPendingChange([reservation({})], confirmed, "2026-11-09"), []);
});

test("a request before today is not pending", () => {
  assert.deepEqual(applyPendingChange([], reservation({}), "2026-11-10"), []);
});

test("an older change never brings back an answered request", () => {
  const shown = reservation({ updated_at: "2026-11-09T16:00:00Z", notes: "newer" });

  const list = applyPendingChange([shown], reservation({}), "2026-11-09");

  assert.equal(list[0].notes, "newer");
});

test("a confirmed request stays in the active list", () => {
  const confirmed = reservation({ status: "confirmed", updated_at: "2026-11-09T16:00:00Z" });

  const list = applyActiveChange([reservation({})], confirmed, "2026-11-09");

  assert.deepEqual(list.map((r) => r.status), ["confirmed"]);
});

test("a cancelled or visited reservation leaves the active list", () => {
  const cancelled = reservation({ status: "cancelled", updated_at: "2026-11-09T16:00:00Z" });
  const visited = reservation({ status: "visited", updated_at: "2026-11-09T16:00:00Z" });

  assert.deepEqual(applyActiveChange([reservation({})], cancelled, "2026-11-09"), []);
  assert.deepEqual(applyActiveChange([reservation({})], visited, "2026-11-09"), []);
});

test("a request is overdue once its time has come", () => {
  assert.equal(isOverdue(reservation({}), new Date("2026-11-10T00:59:00Z").getTime()), false);
  assert.equal(isOverdue(reservation({}), new Date("2026-11-10T01:00:00Z").getTime()), true);
  assert.equal(isOverdue(reservation({ status: "confirmed" }), new Date("2026-11-11T00:00:00Z").getTime()), false);
});

test("staff actions follow the lifecycle", () => {
  const before = new Date("2026-11-10T00:00:00Z").getTime();
  const after = new Date("2026-11-10T01:30:00Z").getTime();

  assert.deepEqual(actionsFor(reservation({}), before), ["confirm", "decline", "cancel"]);
  assert.deepEqual(actionsFor(reservation({}), after), ["confirm", "no-show"]);
  assert.deepEqual(actionsFor(reservation({ status: "confirmed" }), before), ["cancel"]);
  assert.deepEqual(actionsFor(reservation({ status: "confirmed" }), after), ["no-show", "cancel"]);
  assert.deepEqual(actionsFor(reservation({ status: "visited" }), after), ["no-show"]);
  assert.deepEqual(actionsFor(reservation({ status: "visited", feedback_requested: true }), after), []);
  for (const status of ["declined", "cancelled", "no_show"] as const) {
    assert.deepEqual(actionsFor(reservation({ status }), after), [], status);
  }
});

test("the day's summary counts open reservations and expected guests", () => {
  const summary = summarize([
    reservation({ status: "requested", party_size: 6 }),
    reservation({ status: "confirmed", party_size: 4 }),
    reservation({ status: "visited", party_size: 2 }),
    reservation({ status: "cancelled", party_size: 8 }),
  ]);

  assert.deepEqual(summary, { reservations: 3, pending: 1, guests: 6 });
});

test("the closed filter groups every way a reservation ends without a visit", () => {
  for (const status of ["declined", "cancelled", "no_show"] as const) {
    assert.equal(matchesFilter(reservation({ status }), "closed"), true);
  }
  assert.equal(matchesFilter(reservation({ status: "visited" }), "closed"), false);
});

test("a booking's time carries the restaurant's offset", () => {
  assert.equal(startsAtInZone("2026-11-09", "20:00", "America/Bogota"), "2026-11-09T20:00:00-05:00");
  assert.equal(startsAtInZone("2026-07-01", "21:30", "Europe/Madrid"), "2026-07-01T21:30:00+02:00");
  assert.equal(startsAtInZone("2026-12-01", "21:30", "Europe/Madrid"), "2026-12-01T21:30:00+01:00");
  assert.equal(startsAtInZone("2026-11-09", "08:05", "UTC"), "2026-11-09T08:05:00+00:00");
});

test("the offset is the one in force at the booked time, not at midnight", () => {
  // Madrid moves to summer time at 02:00 on 29 March 2026.
  assert.equal(startsAtInZone("2026-03-29", "01:00", "Europe/Madrid"), "2026-03-29T01:00:00+01:00");
  assert.equal(startsAtInZone("2026-03-29", "20:00", "Europe/Madrid"), "2026-03-29T20:00:00+02:00");
});
