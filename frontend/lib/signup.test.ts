import { test } from "node:test";
import assert from "node:assert/strict";
import { signupBody, signupFailure, signupOutcome, validateSignup } from "./signup.ts";

test("a trimmed name and a chosen plan are valid", () => {
  assert.deepEqual(validateSignup({ restaurantName: "  Doña Rosa ", plan: "einvoicing" }), {});
});

test("the name is required after trimming", () => {
  assert.equal(validateSignup({ restaurantName: "   ", plan: "full" }).restaurant_name, "Escribe el nombre del restaurante.");
});

test("the name admits up to 255 characters, counted as characters", () => {
  assert.deepEqual(validateSignup({ restaurantName: "ñ".repeat(255), plan: "full" }), {});
  assert.ok(validateSignup({ restaurantName: "a".repeat(256), plan: "full" }).restaurant_name);
});

test("a plan must be chosen", () => {
  assert.equal(validateSignup({ restaurantName: "Rosa", plan: null }).plan, "Elige un plan.");
});

test("the body sends the trimmed name", () => {
  assert.deepEqual(signupBody("tok", { restaurantName: " Rosa ", plan: "starter" }), {
    id_token: "tok",
    restaurant_name: "Rosa",
    plan: "starter",
  });
});

test("each failure reads in Spanish", () => {
  assert.equal(signupFailure(400, { field: "restaurant_name" }, null).field, "restaurant_name");
  assert.equal(signupFailure(400, { field: "plan" }, null).field, "plan");
  assert.match(signupFailure(401, { error: "Google sign-in failed" }, null).message, /Google/);
  assert.match(signupFailure(404, {}, null).message, /no está disponible/);
  assert.match(signupFailure(429, {}, null).message, /Demasiados intentos/);
});

test("an existing account on an ended trial shows the trial message", () => {
  const failure = signupFailure(403, { code: "trial_expired" }, "soporte@kuenta.co");
  assert.equal(failure.message, "Tu prueba terminó. Escríbenos para continuar");
  assert.ok(failure.mailto);
  assert.equal(signupFailure(403, { code: "trial_expired" }, null).mailto, null);
});

test("a new pending signup goes to the pending page", () => {
  assert.deepEqual(signupOutcome(201, { status: "pending" }, null), { next: "pending" });
});

test("a repeated signup before approval goes to the pending page", () => {
  assert.deepEqual(signupOutcome(403, { error: "Approval pending", code: "pending_approval" }, null), { next: "pending" });
});

test("an existing approved account signs in", () => {
  assert.deepEqual(signupOutcome(200, {}, null), { next: "home" });
});

test("an ended trial still produces its notice", () => {
  const outcome = signupOutcome(403, { code: "trial_expired" }, "soporte@kuenta.co");
  assert.equal(outcome.next, "failure");
  if (outcome.next === "failure") assert.equal(outcome.failure.message, "Tu prueba terminó. Escríbenos para continuar");
});
