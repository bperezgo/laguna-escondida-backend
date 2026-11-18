import { test } from "node:test";
import assert from "node:assert/strict";
import {
  describeAvailability,
  isPastMidnight,
  parseSettingsErrorField,
  sortSchedule,
  validateSchedule,
} from "./schedule.ts";
import type { Availability, DayCode, OpeningWindow } from "@/types/organizationSettings";

const w = (day: DayCode, opens: string, closes: string): OpeningWindow => ({ day, opens, closes });

const closed: Availability = {
  available: false,
  reason: "outside_hours",
  paused_reason: null,
  closes_at: null,
  next_opens_at: null,
};

test("valid schedules have no errors", () => {
  assert.deepEqual(validateSchedule([]), []);
  assert.deepEqual(validateSchedule([w("fri", "18:00", "02:00")]), []);
  assert.deepEqual(validateSchedule([w("mon", "11:00", "15:00"), w("mon", "15:00", "18:00")]), []);
  assert.deepEqual(validateSchedule([w("fri", "18:00", "02:00"), w("sat", "02:00", "04:00")]), []);
});

test("bad times and equal times point at the window", () => {
  assert.deepEqual(
    validateSchedule([w("mon", "24:00", "12:00"), w("tue", "7:00", "12:00"), w("wed", "12:00", "12:00")]).map(
      (e) => e.index,
    ),
    [0, 1, 2],
  );
});

test("overlaps are caught on one day, across midnight and across the week's end", () => {
  assert.equal(validateSchedule([w("mon", "11:00", "15:00"), w("mon", "14:00", "18:00")])[0].index, 1);
  assert.equal(validateSchedule([w("fri", "18:00", "02:00"), w("sat", "01:00", "04:00")])[0].index, 1);
  assert.equal(validateSchedule([w("sun", "22:00", "02:00"), w("mon", "01:00", "03:00")])[0].index, 1);
});

test("more than 21 windows is an error about the whole list", () => {
  const days: DayCode[] = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"];
  const many = ["08:00", "11:00", "14:00", "17:00"].flatMap((opens) =>
    days.map((day) => w(day, opens, opens.replace(/^(\d\d)/, (h) => String(Number(h) + 1).padStart(2, "0")))),
  );
  assert.equal(many.length, 28);
  assert.ok(validateSchedule(many).some((e) => e.index === null));
});

test("past midnight and sorting", () => {
  assert.equal(isPastMidnight(w("fri", "18:00", "02:00")), true);
  assert.equal(isPastMidnight(w("fri", "11:00", "15:00")), false);
  assert.deepEqual(
    sortSchedule([w("sun", "10:00", "11:00"), w("mon", "18:00", "22:00"), w("mon", "11:00", "15:00")]),
    [w("mon", "11:00", "15:00"), w("mon", "18:00", "22:00"), w("sun", "10:00", "11:00")],
  );
});

test("the backend's error field points at a type and window", () => {
  assert.deepEqual(parseSettingsErrorField("ordering.takeaway.schedule[2]"), { type: "takeaway", index: 2 });
  assert.deepEqual(parseSettingsErrorField("ordering.delivery.schedule"), { type: "delivery", index: null });
  assert.equal(parseSettingsErrorField("ordering.delivery.paused_reason"), null);
  assert.equal(parseSettingsErrorField(undefined), null);
});

test("the Ahora line describes each state in the organization's timezone", () => {
  const now = new Date("2026-10-26T19:00:00-05:00"); // Monday in Bogotá
  const tz = "America/Bogota";

  assert.equal(
    describeAvailability({ ...closed, available: true, reason: null, closes_at: "2026-10-26T22:30:00-05:00" }, tz, now),
    "Recibiendo pedidos hasta hoy a las 22:30",
  );
  assert.equal(
    describeAvailability({ ...closed, available: true, reason: null }, tz, now),
    "Recibiendo pedidos (sin horario: abierto siempre)",
  );
  assert.equal(
    describeAvailability({ ...closed, reason: "paused", paused_reason: "cocina saturada" }, tz, now),
    "Pausado: cocina saturada",
  );
  assert.equal(
    describeAvailability({ ...closed, next_opens_at: "2026-10-27T11:30:00-05:00" }, tz, now),
    "Cerrado, abre mañana a las 11:30",
  );
  assert.equal(
    describeAvailability({ ...closed, next_opens_at: "2026-10-30T18:00:00-05:00" }, tz, now),
    "Cerrado, abre el viernes a las 18:00",
  );
  assert.equal(describeAvailability(closed, tz, now), "Cerrado, sin apertura en los próximos 7 días");
});
