import { test } from "node:test";
import assert from "node:assert/strict";
import { CHANGE_CARD_PATH, SUBSCRIPTION_PATH, subscriptionNotice, type NoticeBilling } from "./notice.ts";

const now = Date.parse("2026-12-12T14:00:00Z");
const trialEnd = "2026-12-15T14:00:00Z";
const trial = { status: "trial", trial_ends_at: trialEnd };
const paid = { status: "active", trial_ends_at: trialEnd };

function billing(overrides: Partial<NoticeBilling> = {}): NoticeBilling {
  return {
    has_card: true,
    card_entry: true,
    cancelled: false,
    due_date: trialEnd,
    next_charge_amount: 40000,
    payment_failed: false,
    access_ends_at: "2026-12-18T14:00:00Z",
    ...overrides,
  };
}

const notice = (organization: typeof trial | null, b: NoticeBilling | null, isEdge = false, supportEmail: string | null = "soporte@kuenta.co") =>
  subscriptionNotice({ organization, billing: b, isEdge, now, supportEmail });

test("a trial with a saved card says the days left and the date and amount of the first charge", () => {
  const got = notice(trial, billing());
  assert.equal(got?.message, "Te quedan 3 días de prueba gratis. El 15 de diciembre de 2026, el último día de tu prueba, se cobrarán $40.000.");
  assert.deepEqual(got?.link, { href: SUBSCRIPTION_PATH, label: "Suscripción" });
  assert.equal(got?.tone, "warning");
});

test("one day left is singular", () => {
  const got = subscriptionNotice({ organization: trial, billing: billing(), isEdge: false, now: Date.parse("2026-12-14T15:00:00Z"), supportEmail: null });
  assert.ok(got?.message.startsWith("Te queda 1 día de prueba gratis."));
});

test("a trial without a saved card says the days left and links to Suscripción", () => {
  const got = notice(trial, billing({ has_card: false, next_charge_amount: null }));
  assert.equal(got?.message, "Te quedan 3 días de prueba gratis.");
  assert.deepEqual(got?.link, { href: SUBSCRIPTION_PATH, label: "Suscripción" });
});

test("a trial without a saved card links to support when card entry is off", () => {
  const got = notice(trial, billing({ has_card: false, card_entry: false, next_charge_amount: null }));
  assert.equal(got?.message, "Te quedan 3 días de prueba gratis.");
  assert.ok(got?.link?.href.startsWith("mailto:soporte@kuenta.co"));
  const noEmail = notice(trial, billing({ has_card: false, card_entry: false, next_charge_amount: null }), false, null);
  assert.equal(noEmail?.link, null);
});

test("a failed charge of a working organization says the day access ends and links to changing the card", () => {
  const got = notice(paid, billing({ payment_failed: true, due_date: "2027-01-15T14:00:00Z", access_ends_at: "2027-01-18T14:00:00Z" }));
  assert.equal(got?.message, "No pudimos cobrar tu suscripción. Tu acceso termina el 18 de enero de 2027.");
  assert.deepEqual(got?.link, { href: CHANGE_CARD_PATH, label: "Cambiar la tarjeta" });
  assert.equal(got?.tone, "danger");
});

test("a failed first charge during the trial's grace shows the failure, not the days left", () => {
  const got = notice(trial, billing({ payment_failed: true }));
  assert.ok(got?.message.startsWith("No pudimos cobrar tu suscripción."));
});

test("a cancelled subscription that still works says its last day and offers to resume", () => {
  const got = notice(paid, billing({ cancelled: true, next_charge_amount: null, due_date: "2027-01-15T14:00:00Z", access_ends_at: "2027-01-15T14:00:00Z" }));
  assert.equal(got?.message, "Cancelaste tu suscripción. Tu último día de acceso es el 15 de enero de 2027.");
  assert.deepEqual(got?.link, { href: SUBSCRIPTION_PATH, label: "Reanudar" });
});

test("a cancelled trial says its last day too", () => {
  const got = notice(trial, billing({ cancelled: true, next_charge_amount: null, access_ends_at: trialEnd }));
  assert.equal(got?.message, "Cancelaste tu suscripción. Tu último día de acceso es el 15 de diciembre de 2026.");
});

test("a paid organization with nothing wrong sees no notice", () => {
  assert.equal(notice(paid, billing({ due_date: "2027-01-15T14:00:00Z", access_ends_at: "2027-01-18T14:00:00Z" })), null);
});

test("an organization activated by hand sees no notice", () => {
  assert.equal(notice(paid, billing({ has_card: false, due_date: null, next_charge_amount: null, access_ends_at: null })), null);
});

test("a non-admin during a trial sees only the days left, as before", () => {
  const got = notice(trial, null);
  assert.equal(got?.message, "Te quedan 3 días de prueba gratis.");
  assert.ok(got?.link?.href.startsWith("mailto:"));
  assert.equal(got?.link?.label, "Escríbenos para continuar");
  assert.ok(!got?.message.includes("$"));
});

test("a non-admin of a paid organization sees nothing, whatever its payments", () => {
  assert.equal(notice(paid, null), null);
});

test("an edge box shows none", () => {
  assert.equal(notice(trial, billing(), true), null);
  assert.equal(notice(trial, null, true), null);
  assert.equal(notice(paid, billing({ payment_failed: true }), true), null);
});

test("no organization, no notice", () => {
  assert.equal(notice(null, billing()), null);
});
