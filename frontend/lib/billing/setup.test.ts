import { test } from "node:test";
import assert from "node:assert/strict";
import {
  CARD_REFUSED_MESSAGE,
  firstChargeDate,
  formatLongDate,
  parseSetupState,
  parseSignupSummary,
  PROVIDER_DOWN_MESSAGE,
  setupOutcome,
  TOO_MANY_ATTEMPTS_MESSAGE,
} from "./setup.ts";
import { cardDecision, cardRequestBody, setupDecision } from "./setupDecision.ts";

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });

const wompi = {
  public_key: "pub_test_abc",
  api_url: "https://sandbox.wompi.co/v1",
  terms_url: "https://wompi.co/terms.pdf",
  personal_data_url: "https://wompi.co/data.pdf",
  acceptance_token: "acc-1",
  personal_data_acceptance_token: "acc-2",
};

test("the first charge is 14 days after the card is saved, in Colombia's date", () => {
  const savedAt = new Date("2026-11-26T15:00:00-05:00");
  assert.equal(formatLongDate(firstChargeDate(savedAt, 14)), "10 de diciembre de 2026");
  assert.equal(formatLongDate(new Date("2026-12-11T03:30:00Z")), "10 de diciembre de 2026", "late evening in Bogotá is still the 10th");
  assert.equal(formatLongDate(firstChargeDate(new Date("2026-12-25T12:00:00-05:00"), 14)), "8 de enero de 2027");
});

test("the setup state is read with and without card entry", () => {
  const on = parseSetupState({ organization_name: "Doña Rosa", plan: "einvoicing", period: "annual", card_entry: true, wompi });
  assert.equal(on?.cardEntry, true);
  assert.equal(on?.wompi?.acceptanceToken, "acc-1");
  const off = parseSetupState({ organization_name: "Doña Rosa", plan: "starter", period: "monthly", card_entry: false });
  assert.deepEqual(off, { organizationName: "Doña Rosa", plan: "starter", period: "monthly", cardEntry: false, wompi: null });
  assert.equal(parseSetupState({ organization_name: "Rosa", plan: "gold", period: "monthly" }), null);
});

test("card entry without everything it needs shows no card form", () => {
  const state = parseSetupState({ organization_name: "Rosa", plan: "full", period: "monthly", card_entry: true, wompi: { ...wompi, acceptance_token: "" } });
  assert.equal(state?.cardEntry, false);
  assert.equal(state?.wompi, null);
});

test("each answer of the card save leads where the page says", () => {
  assert.deepEqual(setupOutcome(200, {}), { next: "done" });
  assert.deepEqual(setupOutcome(402, { code: "card_refused" }), { next: "retry", message: CARD_REFUSED_MESSAGE, clearCard: true });
  assert.deepEqual(setupOutcome(429, {}), { next: "retry", message: TOO_MANY_ATTEMPTS_MESSAGE, clearCard: false });
  assert.deepEqual(setupOutcome(502, { code: "payment_provider_unavailable" }), { next: "retry", message: PROVIDER_DOWN_MESSAGE, clearCard: false });
  assert.deepEqual(setupOutcome(401, {}), { next: "signin" });
  assert.deepEqual(setupOutcome(409, { code: "not_pending" }), { next: "home" });
  assert.equal(setupOutcome(500, {}).next, "retry");
});

test("the three messages differ, so the person knows what to do", () => {
  assert.equal(new Set([CARD_REFUSED_MESSAGE, TOO_MANY_ATTEMPTS_MESSAGE, PROVIDER_DOWN_MESSAGE]).size, 3);
  assert.equal(CARD_REFUSED_MESSAGE, "No pudimos guardar tu tarjeta. Revisa los datos o prueba con otra.");
});

test("the summary for step 3 holds no more than a receipt would", () => {
  const raw = JSON.stringify({ plan: "einvoicing", period: "monthly", amount: 40000, cardBrand: "visa", cardLastFour: "4242", number: "4242424242424242" });
  assert.deepEqual(parseSignupSummary(raw), { plan: "einvoicing", period: "monthly", amount: 40000, cardBrand: "visa", cardLastFour: "4242" });
  assert.equal(parseSignupSummary(JSON.stringify({ plan: "einvoicing", period: "monthly", cardLastFour: "4242424242424242" }))?.cardLastFour, null);
  assert.equal(parseSignupSummary("not json"), null);
  assert.equal(parseSignupSummary(null), null);
});

test("a saved card sets the session and ends the setup session, and no token reaches the page", async () => {
  const decision = await cardDecision(json(200, { token: "signin-jwt", username: "rosa@example.com", roles: [], permissions: [] }));
  assert.deepEqual(decision, { status: 200, body: { success: true }, token: "signin-jwt", clearSetup: true });
  assert.ok(!JSON.stringify(decision.body).includes("jwt"));
});

test("a refused card keeps the setup session", async () => {
  const decision = await cardDecision(json(402, { error: "Card refused", code: "card_refused" }));
  assert.deepEqual(decision, { status: 402, body: { error: "Card refused", code: "card_refused" } });
});

test("too many attempts pass Retry-After on", async () => {
  const decision = await cardDecision(json(429, { error: "Too many requests" }, { "Retry-After": "1800" }));
  assert.equal(decision.status, 429);
  assert.equal(decision.retryAfter, "1800");
});

test("an expired or finished setup session is cleared", async () => {
  assert.equal((await setupDecision(json(401, { error: "Invalid or expired token" }))).clearSetup, true);
  assert.equal((await setupDecision(json(409, { error: "x", code: "not_pending" }))).clearSetup, true);
  assert.equal((await setupDecision(json(502, { error: "x", code: "payment_provider_unavailable" }))).clearSetup, undefined);
});

test("the setup state passes through, and it carries no token of ours", async () => {
  const state = { organization_name: "Doña Rosa", plan: "starter", period: "monthly", card_entry: true, wompi };
  assert.deepEqual(await setupDecision(json(200, state)), { status: 200, body: state });
});

test("only Wompi's token and the acceptances go on to the backend", () => {
  const sent = { card_token: "tok_1", acceptance_token: "a", personal_data_acceptance_token: "b", number: "4242424242424242", cvc: "123", exp_month: "12" };
  assert.deepEqual(cardRequestBody(sent), { card_token: "tok_1", acceptance_token: "a", personal_data_acceptance_token: "b" });
  assert.equal(cardRequestBody({ card_token: "tok_1" }), null);
  assert.equal(cardRequestBody(null), null);
});
