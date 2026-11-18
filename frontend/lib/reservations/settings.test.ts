import { test } from "node:test";
import assert from "node:assert/strict";
import type { ReservationSettings } from "../../types/organizationSettings.ts";
import {
  describeNotice,
  describePolicy,
  draftFrom,
  hasErrors,
  isEmptyPatch,
  parsePolicyErrorField,
  patchFrom,
  validatePolicy,
} from "./settings.ts";

const defaults: ReservationSettings = {
  auto_confirm_enabled: false,
  max_party_size: 8,
  min_notice_minutes: 120,
  daily_guest_cap: null,
  schedule: [],
};

test("an untouched draft sends nothing", () => {
  const draft = draftFrom(defaults);
  assert.deepEqual(draft, { enabled: false, maxPartySize: "8", minNoticeMinutes: "120", dailyGuestCap: "", schedule: [] });
  assert.ok(isEmptyPatch(patchFrom(draft, defaults)));
  assert.equal(hasErrors(validatePolicy(draft, defaults)), false);
});

test("turning auto-confirm on sends only the switch and the cap", () => {
  const draft = { ...draftFrom(defaults), enabled: true, dailyGuestCap: "120" };
  assert.equal(hasErrors(validatePolicy(draft, defaults)), false);
  assert.deepEqual(patchFrom(draft, defaults), { auto_confirm_enabled: true, daily_guest_cap: 120 });
});

test("enabling without a cap is refused before saving", () => {
  const errors = validatePolicy({ ...draftFrom(defaults), enabled: true }, defaults);
  assert.match(errors.fields.daily_guest_cap ?? "", /Obligatorio/);
});

test("a saved cap can't be blanked, even with the policy off", () => {
  const saved = { ...defaults, daily_guest_cap: 80 };
  const errors = validatePolicy({ ...draftFrom(saved), dailyGuestCap: "" }, saved);
  assert.ok(errors.fields.daily_guest_cap);
});

test("ranges match the backend", () => {
  const check = (overrides: Partial<ReturnType<typeof draftFrom>>) =>
    validatePolicy({ ...draftFrom(defaults), ...overrides }, defaults).fields;
  assert.ok(check({ maxPartySize: "0" }).max_party_size);
  assert.ok(check({ maxPartySize: "51" }).max_party_size);
  assert.ok(check({ maxPartySize: "4.5" }).max_party_size);
  assert.equal(check({ maxPartySize: "50" }).max_party_size, undefined);
  assert.equal(check({ minNoticeMinutes: "0" }).min_notice_minutes, undefined);
  assert.ok(check({ minNoticeMinutes: "10081" }).min_notice_minutes);
  assert.ok(check({ minNoticeMinutes: "-5" }).min_notice_minutes);
  assert.ok(check({ dailyGuestCap: "0" }).daily_guest_cap);
});

test("schedule overlaps are caught, and an unchanged schedule isn't resent", () => {
  const saved = { ...defaults, schedule: [{ day: "fri" as const, opens: "18:00", closes: "02:00" }] };
  assert.ok(isEmptyPatch(patchFrom(draftFrom(saved), saved)));
  const overlapping = {
    ...draftFrom(saved),
    schedule: [...saved.schedule, { day: "sat" as const, opens: "01:00", closes: "04:00" }],
  };
  assert.equal(validatePolicy(overlapping, saved).schedule.length, 1);
  assert.equal(patchFrom(overlapping, saved).schedule?.length, 2);
});

test("backend error fields point at the input", () => {
  assert.deepEqual(parsePolicyErrorField("reservations.daily_guest_cap"), { field: "daily_guest_cap" });
  assert.deepEqual(parsePolicyErrorField("reservations.schedule[2]"), { scheduleIndex: 2 });
  assert.deepEqual(parsePolicyErrorField("reservations.schedule"), { scheduleIndex: null });
  assert.equal(parsePolicyErrorField("ordering.takeaway.schedule[0]"), null);
  assert.equal(parsePolicyErrorField(undefined), null);
});

test("notice reads in staff terms", () => {
  assert.equal(describeNotice(0), "sin anticipación mínima");
  assert.equal(describeNotice(90), "1 hora y 30 minutos");
  assert.equal(describeNotice(120), "2 horas");
  assert.equal(describeNotice(45), "45 minutos");
  assert.equal(describeNotice(1500), "1 día y 1 hora");
});

test("the summary says what the agent confirms", () => {
  assert.match(describePolicy(defaults), /Apagada/);
  const on = describePolicy({ ...defaults, auto_confirm_enabled: true, daily_guest_cap: 120 });
  assert.match(on, /hasta 8 personas/);
  assert.match(on, /2 horas/);
  assert.match(on, /a cualquier hora/);
  assert.match(on, /120 invitados/);
});
