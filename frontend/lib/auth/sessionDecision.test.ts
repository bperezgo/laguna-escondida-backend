import { test } from "node:test";
import assert from "node:assert/strict";
import { sessionDecision, signupDecision } from "./sessionDecision.ts";

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

test("a token is kept for the cookie and never echoed to the browser", async () => {
  const decision = await sessionDecision(json(200, { token: "jwt", username: "ana" }), "x");
  assert.deepEqual(decision, { status: 200, body: { success: true }, token: "jwt" });
});

test("a signup's 201 keeps its status", async () => {
  const decision = await sessionDecision(json(201, { token: "jwt" }), "x");
  assert.equal(decision.status, 201);
  assert.equal(decision.token, "jwt");
});

test("a 401 passes through without a token", async () => {
  const decision = await sessionDecision(json(401, { error: "Google sign-in failed" }), "x");
  assert.deepEqual(decision, { status: 401, body: { error: "Google sign-in failed" } });
});

test("a 403 trial_expired passes through with its code", async () => {
  const decision = await sessionDecision(json(403, { error: "Trial ended", code: "trial_expired" }), "x");
  assert.deepEqual(decision, { status: 403, body: { error: "Trial ended", code: "trial_expired" } });
});

test("the field of a 400 passes through", async () => {
  const decision = await sessionDecision(
    json(400, { error: "invalid plan: must be starter, einvoicing or full", field: "plan" }),
    "x"
  );
  assert.equal(decision.body.field, "plan");
});

test("a non-JSON error gets the fallback message", async () => {
  const decision = await sessionDecision(new Response("404 page not found", { status: 404 }), "Sign-in failed");
  assert.deepEqual(decision, { status: 404, body: { error: "Sign-in failed" } });
});

test("a success without a token is a server error", async () => {
  const decision = await sessionDecision(json(200, {}), "x");
  assert.equal(decision.status, 500);
});

test("a pending signup's 201 passes through with no token, so no cookie is set", async () => {
  const decision = await signupDecision(
    json(201, { status: "pending", organization_name: "Doña Rosa", plan: "einvoicing" }),
    "x"
  );
  assert.deepEqual(decision, { status: 201, body: { status: "pending", organization_name: "Doña Rosa", plan: "einvoicing" } });
});

test("a signup of an existing approved account still sets the session", async () => {
  const decision = await signupDecision(json(200, { token: "jwt" }), "x");
  assert.deepEqual(decision, { status: 200, body: { success: true }, token: "jwt" });
});

test("a signup's pending_approval passes through with its code", async () => {
  const decision = await signupDecision(json(403, { error: "Approval pending", code: "pending_approval" }), "x");
  assert.deepEqual(decision, { status: 403, body: { error: "Approval pending", code: "pending_approval" } });
});

test("a signup's setup token goes to its cookie and never to the page", async () => {
  const decision = await signupDecision(
    json(201, { status: "pending", organization_name: "Doña Rosa", plan: "einvoicing", setup_token: "setup-jwt" }),
    "x"
  );
  assert.deepEqual(decision, {
    status: 201,
    body: { status: "pending", organization_name: "Doña Rosa", plan: "einvoicing", setup: true },
    setupToken: "setup-jwt",
  });
  assert.ok(!JSON.stringify(decision.body).includes("setup-jwt"));
});

test("a pending admin's sign-in keeps the setup token out of the body", async () => {
  const decision = await sessionDecision(json(403, { error: "Approval pending", code: "pending_approval", setup_token: "setup-jwt" }), "x");
  assert.deepEqual(decision, {
    status: 403,
    body: { error: "Approval pending", code: "pending_approval", setup: true },
    setupToken: "setup-jwt",
  });
});

test("a setup token on any other answer is ignored", async () => {
  const decision = await sessionDecision(json(403, { error: "Trial ended", code: "trial_expired", setup_token: "setup-jwt" }), "x");
  assert.deepEqual(decision, { status: 403, body: { error: "Trial ended", code: "trial_expired" } });
});
