import { test } from "node:test";
import assert from "node:assert/strict";
import { parseRecoveryState, payOutcome, recoveryView } from "./recovery.ts";
import { payRequestBody, recoveryDecision, recoveryStateDecision } from "./recoveryDecision.ts";
import { CARD_REFUSED_MESSAGE, PROVIDER_DOWN_MESSAGE, TOO_MANY_ATTEMPTS_MESSAGE } from "./setup.ts";

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

const charge = (status: string) => ({ id: "c1", date: "2027-01-18T14:04:02Z", amount: 40000, currency: "COP", status, covers_from: null, covers_until: null });

const stopped = (overrides: Record<string, unknown> = {}) => ({
  organization_name: "Doña Rosa",
  reason: "payment_required",
  plan: "einvoicing",
  period: "monthly",
  amount_due: 40000,
  currency: "COP",
  card: { brand: "VISA", last_four: "4242" },
  latest_charge: null,
  card_entry: true,
  wompi,
  ...overrides,
});

test("the recovery state is read from the backend's body", () => {
  const state = parseRecoveryState(stopped({ latest_charge: charge("declined") }));
  assert.equal(state?.organizationName, "Doña Rosa");
  assert.equal(state?.reason, "payment_required");
  assert.equal(state?.amountDue, 40000);
  assert.deepEqual(state?.card, { brand: "VISA", lastFour: "4242" });
  assert.equal(state?.latestCharge?.status, "declined");
  assert.equal(state?.wompi?.publicKey, "pub_test_abc");
});

test("a state without a known reason is not shown", () => {
  assert.equal(parseRecoveryState(stopped({ reason: "suspended" })), null);
  assert.equal(parseRecoveryState({}), null);
});

test("card entry off: the page shows the reason and the support contact only", () => {
  const state = parseRecoveryState(stopped({ card_entry: false, wompi: undefined }));
  assert.equal(state?.cardEntry, false);
  assert.equal(state?.wompi, null);
});

test("the page offers to pay when no charge is waiting", () => {
  assert.equal(recoveryView(200, stopped()).next, "pay");
  assert.equal(recoveryView(200, stopped({ latest_charge: charge("approved") })).next, "pay");
});

test("a pending charge is waited for", () => {
  const view = recoveryView(200, stopped({ latest_charge: charge("pending") }));
  assert.equal(view.next, "pending");
});

test("a declined charge says so and offers another card", () => {
  for (const status of ["declined", "voided", "error"]) {
    assert.equal(recoveryView(200, stopped({ latest_charge: charge(status) })).next, "declined");
  }
});

test("an approved charge takes the person into the app", () => {
  assert.deepEqual(recoveryView(200, { signed_in: true }), { next: "home" });
});

test("an account that already works, or an expired session, goes to sign-in", () => {
  assert.deepEqual(recoveryView(409, { code: "not_stopped" }), { next: "signin", expired: false });
  assert.deepEqual(recoveryView(401, { error: "Setup session expired" }), { next: "signin", expired: true });
});

test("anything unreadable is tried again, never shown as paid", () => {
  assert.deepEqual(recoveryView(502, {}), { next: "retry" });
  assert.deepEqual(recoveryView(200, { token: "jwt" }), { next: "retry" });
  assert.deepEqual(recoveryView(200, null), { next: "retry" });
});

test("a payment that was taken, or one already waiting, is followed by polling", () => {
  assert.deepEqual(payOutcome(202, {}), { next: "poll" });
  assert.deepEqual(payOutcome(409, { code: "charge_pending" }), { next: "poll" });
});

test("a payment's failures say what to do", () => {
  assert.deepEqual(payOutcome(402, { code: "card_refused" }), { next: "retry", message: CARD_REFUSED_MESSAGE, clearCard: true });
  assert.deepEqual(payOutcome(429, {}), { next: "retry", message: TOO_MANY_ATTEMPTS_MESSAGE, clearCard: false });
  assert.deepEqual(payOutcome(502, {}), { next: "retry", message: PROVIDER_DOWN_MESSAGE, clearCard: false });
  assert.equal(payOutcome(400, { field: "card_token" }).next, "retry");
  assert.deepEqual(payOutcome(409, { code: "not_stopped" }), { next: "signin", expired: false });
  assert.deepEqual(payOutcome(401, {}), { next: "signin", expired: true });
});

test("the recovery state passes through while the account is stopped", async () => {
  const decision = await recoveryStateDecision(json(200, stopped()));
  assert.equal(decision.status, 200);
  assert.equal(decision.body.reason, "payment_required");
  assert.equal(decision.token, undefined);
  assert.equal(decision.clearSetup, undefined);
});

test("the sign-in of an approved payment sets access_token, clears setup_token and never reaches the page", async () => {
  const decision = await recoveryStateDecision(
    json(200, { token: "sign-in-jwt", username: "rosa@example.com", roles: [{ id: 2, name: "admin" }], permissions: ["billing:manage"] })
  );
  assert.deepEqual(decision, { status: 200, body: { signed_in: true }, token: "sign-in-jwt", clearSetup: true });
  assert.ok(!JSON.stringify(decision.body).includes("sign-in-jwt"));
});

test("not_stopped and an expired session end the setup session", async () => {
  const notStopped = await recoveryStateDecision(json(409, { error: "Organization is not stopped", code: "not_stopped" }));
  assert.deepEqual(notStopped, { status: 409, body: { error: "Organization is not stopped", code: "not_stopped" }, clearSetup: true });
  const expired = await recoveryDecision(json(401, { error: "Invalid or expired token" }));
  assert.equal(expired.clearSetup, true);
});

test("a pending charge keeps the setup session, and the limit keeps its Retry-After", async () => {
  const pending = await recoveryDecision(json(409, { error: "A charge is pending", code: "charge_pending" }));
  assert.equal(pending.clearSetup, undefined);
  assert.equal(pending.body.code, "charge_pending");
  const limited = await recoveryDecision(json(429, { error: "Too many requests" }, { "Retry-After": "1800" }));
  assert.equal(limited.retryAfter, "1800");
});

test("a pay answer passes through with its charge", async () => {
  const decision = await recoveryDecision(json(202, { charge: charge("pending") }));
  assert.equal(decision.status, 202);
  assert.deepEqual(decision.body, { charge: charge("pending") });
});

test("a backend error never leaks more than its error, code and field", async () => {
  const decision = await recoveryDecision(json(400, { error: "invalid card_token: is required", field: "card_token", debug: "prv_test_secret" }));
  assert.deepEqual(decision.body, { error: "invalid card_token: is required", field: "card_token" });
});

test("paying with the saved card sends nothing; a new card sends only Wompi's token and the acceptances", () => {
  assert.deepEqual(payRequestBody(null), {});
  assert.deepEqual(payRequestBody({}), {});
  assert.deepEqual(payRequestBody({ card_token: "" }), {});
  assert.deepEqual(
    payRequestBody({ card_token: "tok_1", acceptance_token: "acc-1", personal_data_acceptance_token: "acc-2", number: "4242424242424242", cvc: "123" }),
    { card_token: "tok_1", acceptance_token: "acc-1", personal_data_acceptance_token: "acc-2" }
  );
});

test("half a new card is refused instead of charging the saved one", () => {
  assert.equal(payRequestBody({ card_token: "tok_1" }), null);
  assert.equal(payRequestBody({ card_token: "tok_1", acceptance_token: "acc-1" }), null);
  assert.equal(payRequestBody("tok_1"), null);
});
