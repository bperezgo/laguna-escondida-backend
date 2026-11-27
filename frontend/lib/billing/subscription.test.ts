import { test } from "node:test";
import assert from "node:assert/strict";
import { parseSubscription, savedCardLabel, subscriptionState, type Subscription } from "./subscription.ts";

const trialBody = {
  plan: "einvoicing",
  period: "monthly",
  status: "trial",
  trial_ends_at: "2026-12-15T14:00:00Z",
  paid_until: null,
  due_date: "2026-12-15T14:00:00Z",
  next_charge_amount: 40000,
  currency: "COP",
  cancelled: false,
  payment_failed: false,
  access_ends_at: "2026-12-18T14:00:00Z",
  card: { brand: "VISA", last_four: "4242" },
  latest_charge: null,
  charges: [],
  card_entry: true,
};

const approved = { id: "c1", date: "2026-12-15T14:02:41Z", amount: 40000, currency: "COP", status: "approved", covers_from: "2026-12-15T14:00:00Z", covers_until: "2027-01-15T14:00:00Z" };
const declined = { id: "c2", date: "2027-01-15T14:03:10Z", amount: 40000, currency: "COP", status: "declined", covers_from: null, covers_until: null };

const parsed = (overrides: Record<string, unknown> = {}): Subscription => {
  const sub = parseSubscription({ ...trialBody, ...overrides });
  assert.ok(sub);
  return sub;
};

test("the subscription is read from the backend's body", () => {
  const sub = parsed({ charges: [declined, approved], latest_charge: declined });
  assert.equal(sub.plan, "einvoicing");
  assert.equal(sub.nextChargeAmount, 40000);
  assert.deepEqual(sub.card, { brand: "VISA", lastFour: "4242" });
  assert.equal(sub.charges.length, 2);
  assert.equal(sub.charges[1].coversUntil, "2027-01-15T14:00:00Z");
  assert.equal(sub.latestCharge?.status, "declined");
});

test("a body that is not a subscription is refused", () => {
  for (const body of [null, "x", {}, { ...trialBody, plan: "gold" }, { ...trialBody, period: "weekly" }]) {
    assert.equal(parseSubscription(body), null);
  }
});

test("an unknown charge is dropped, not shown wrong", () => {
  const sub = parsed({ charges: [approved, { id: "c9", date: "x", amount: "1", status: "refunded" }] });
  assert.equal(sub.charges.length, 1);
});

test("in words: on trial until a date", () => {
  const state = subscriptionState(parsed());
  assert.equal(state.kind, "trial");
  assert.equal(state.label, "En prueba hasta el 15 de diciembre de 2026");
});

test("in words: paid until a date", () => {
  const state = subscriptionState(parsed({ status: "active", paid_until: "2027-01-15T14:00:00Z", due_date: "2027-01-15T14:00:00Z" }));
  assert.equal(state.kind, "paid");
  assert.equal(state.label, "Pagada hasta el 15 de enero de 2027");
});

test("in words: cancelled, with its last day", () => {
  const state = subscriptionState(
    parsed({ status: "active", paid_until: "2027-01-15T14:00:00Z", due_date: "2027-01-15T14:00:00Z", cancelled: true, next_charge_amount: null, access_ends_at: "2027-01-15T14:00:00Z" })
  );
  assert.equal(state.kind, "cancelled");
  assert.equal(state.label, "Cancelada. Tu último día es el 15 de enero de 2027");
});

test("in words: the payment failed, with the day access ends", () => {
  const state = subscriptionState(
    parsed({ status: "active", paid_until: "2027-01-15T14:00:00Z", due_date: "2027-01-15T14:00:00Z", payment_failed: true, access_ends_at: "2027-01-18T14:00:00Z", charges: [declined] })
  );
  assert.equal(state.kind, "failed");
  assert.equal(state.label, "El pago falló");
  assert.ok(state.detail?.includes("18 de enero de 2027"));
});

test("a failed payment comes before a cancellation", () => {
  assert.equal(subscriptionState(parsed({ payment_failed: true, cancelled: true })).kind, "failed");
});

test("in words: without a card, billing is handled with support", () => {
  const byHand = subscriptionState(parsed({ status: "active", card: null, due_date: null, next_charge_amount: null, access_ends_at: null, trial_ends_at: null }));
  assert.equal(byHand.kind, "no-card");
  assert.equal(byHand.label, "Activa");
  assert.ok(byHand.detail?.includes("soporte"));

  const trialWithoutCard = subscriptionState(parsed({ card: null, next_charge_amount: null }));
  assert.equal(trialWithoutCard.kind, "no-card");
  assert.equal(trialWithoutCard.label, "En prueba hasta el 15 de diciembre de 2026");
});

test("a card on file with no paid period is an organization activated by hand", () => {
  const state = subscriptionState(parsed({ status: "active", due_date: null, next_charge_amount: null, access_ends_at: null }));
  assert.equal(state.kind, "no-card");
});

test("the card reads as its brand and last four digits", () => {
  assert.equal(savedCardLabel({ brand: "VISA", lastFour: "4242" }), "Visa terminada en 4242");
  assert.equal(savedCardLabel({ brand: "mastercard", lastFour: "5100" }), "Mastercard terminada en 5100");
  assert.equal(savedCardLabel({ brand: "", lastFour: "1234" }), "Tarjeta terminada en 1234");
  assert.equal(savedCardLabel({ brand: "VISA", lastFour: "" }), "Tarjeta guardada");
});
