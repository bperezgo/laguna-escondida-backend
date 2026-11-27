import { test } from "node:test";
import assert from "node:assert/strict";
import { LEGACY_EDGE_CONTEXT, isWaitingForUsers, parseSignInContext, signInBody, signInErrorMessage, signInFailure, signInNotice, signInOutcome } from "./signIn.ts";
import { supportEmail } from "./support.ts";

test("the cloud context needs no organization", () => {
  assert.deepEqual(parseSignInContext({ mode: "cloud" }), { mode: "cloud" });
});

test("the edge context carries the box's restaurant name", () => {
  assert.deepEqual(parseSignInContext({ mode: "edge", organization: { name: "Laguna Escondida" } }), {
    mode: "edge",
    organization: { name: "Laguna Escondida" },
  });
});

test("an unreadable context falls back to edge mode with no name", () => {
  for (const body of [null, "nope", {}, { mode: "other" }, { mode: "edge" }]) {
    assert.deepEqual(parseSignInContext(body), LEGACY_EDGE_CONTEXT);
  }
});

test("the sign-in body sends the trimmed email when there is one", () => {
  assert.deepEqual(signInBody({ email: " Ana@Example.com ", username: "ana", password: "secret" }), {
    email: "Ana@Example.com",
    password: "secret",
  });
});

test("the sign-in body sends the username otherwise", () => {
  assert.deepEqual(signInBody({ email: "  ", username: "cajero1", password: "secret" }), {
    username: "cajero1",
    password: "secret",
  });
});

test("the sign-in body needs a credential and a password", () => {
  assert.equal(signInBody({ password: "secret" }), null);
  assert.equal(signInBody({ username: "cajero1", password: "" }), null);
});

test("the opaque sign-in errors read in Spanish", () => {
  assert.equal(signInErrorMessage("Invalid email or password"), "Correo o contraseña incorrectos");
  assert.equal(signInErrorMessage("Invalid username or password"), "Usuario o contraseña incorrectos");
  assert.equal(signInErrorMessage("email is required"), "email is required");
});

test("an ended trial shows the trial message with a mailto when the support email is set", () => {
  const failure = signInFailure({ error: "Trial ended", code: "trial_expired" }, supportEmail("soporte@kuenta.co"));
  assert.equal(failure.message, "Tu prueba terminó. Escríbenos para continuar");
  assert.ok(failure.mailto?.startsWith("mailto:soporte@kuenta.co"));
});

test("an ended trial shows the message without a link when the support email is unset or blank", () => {
  for (const raw of [undefined, "", "   "]) {
    const failure = signInFailure({ error: "Trial ended", code: "trial_expired" }, supportEmail(raw));
    assert.equal(failure.message, "Tu prueba terminó. Escríbenos para continuar");
    assert.equal(failure.mailto, null);
  }
});

test("other sign-in failures read in Spanish without a link", () => {
  assert.deepEqual(signInFailure({ error: "Invalid email or password" }, "soporte@kuenta.co"), {
    message: "Correo o contraseña incorrectos",
    mailto: null,
  });
  assert.equal(signInFailure({ error: "Google sign-in failed" }, null).message, "No pudimos iniciar sesión con esa cuenta de Google.");
});

test("a pending admin continues signup at step 2", () => {
  assert.deepEqual(signInOutcome(false, { error: "Approval pending", code: "pending_approval", setup: true }, null), { next: "payment" });
});

test("a pending account without a setup session is told it is not active yet", () => {
  const outcome = signInOutcome(false, { error: "Approval pending", code: "pending_approval" }, null);
  assert.equal(outcome.next, "failure");
  if (outcome.next === "failure") assert.match(outcome.failure.message, /todavía no está activa/);
});

test("an expired setup session explains why the person is signing in again", () => {
  assert.match(signInNotice("sesion") ?? "", /continuar con tu registro/);
  assert.equal(signInNotice(undefined), null);
  assert.equal(signInNotice("otro"), null);
});

test("a successful sign-in goes home", () => {
  assert.deepEqual(signInOutcome(true, {}, null), { next: "home" });
});

test("an ended trial is still a failure with its notice", () => {
  const outcome = signInOutcome(false, { error: "Trial ended", code: "trial_expired" }, supportEmail("soporte@kuenta.co"));
  assert.equal(outcome.next, "failure");
  if (outcome.next === "failure") {
    assert.equal(outcome.failure.message, "Tu prueba terminó. Escríbenos para continuar");
    assert.ok(outcome.failure.mailto);
  }
});

test("the admin of a stopped account goes to the recovery page", () => {
  assert.deepEqual(signInOutcome(false, { error: "Trial ended", code: "trial_expired", setup: true }, null), { next: "recovery" });
  assert.deepEqual(signInOutcome(false, { error: "Payment required", code: "payment_required", setup: true }, null), { next: "recovery" });
});

test("anyone else of an unpaid account sees the refusal and the support contact", () => {
  const outcome = signInOutcome(false, { error: "Payment required", code: "payment_required" }, supportEmail("soporte@kuenta.co"));
  assert.equal(outcome.next, "failure");
  if (outcome.next === "failure") {
    assert.match(outcome.failure.message, /suscripción de tu restaurante está vencida/);
    assert.ok(outcome.failure.mailto?.startsWith("mailto:soporte@kuenta.co"));
  }
});

test("a setup session never turns another refusal into recovery", () => {
  assert.equal(signInOutcome(false, { error: "Invalid email or password", setup: true }, null).next, "failure");
});

test("a box with users is ready and reports no sync state worth showing", () => {
  const context = parseSignInContext({ mode: "edge", organization: { name: "Laguna" }, users_ready: true, sync: null });
  assert.equal(context.usersReady, true);
  assert.equal(isWaitingForUsers(context), false);
});

test("a box without users waits, with the sync state it reports", () => {
  for (const sync of ["waiting", "offline", "failed", "replaced", "suspended"]) {
    const context = parseSignInContext({ mode: "edge", organization: { name: "Laguna" }, users_ready: false, sync });
    assert.equal(isWaitingForUsers(context), true);
    assert.equal(context.sync, sync);
  }
});

test("an unknown or missing sync state is treated as waiting", () => {
  for (const sync of ["exploded", 7, null, undefined]) {
    assert.equal(parseSignInContext({ mode: "edge", users_ready: false, sync }).sync, "waiting");
  }
});

test("users_ready counts only when it is a boolean; otherwise the form shows", () => {
  for (const users_ready of ["false", 0, null, undefined]) {
    const context = parseSignInContext({ mode: "edge", organization: { name: "Laguna" }, users_ready });
    assert.equal(context.usersReady, undefined);
    assert.equal(isWaitingForUsers(context), false);
  }
});

test("a box whose organization could not be read still waits", () => {
  const context = parseSignInContext({ mode: "edge", users_ready: false, sync: "failed" });
  assert.equal(isWaitingForUsers(context), true);
  assert.equal(context.organization, undefined);
});

test("the cloud and an unreadable context never wait", () => {
  assert.equal(isWaitingForUsers(parseSignInContext({ mode: "cloud", users_ready: false })), false);
  assert.equal(isWaitingForUsers(parseSignInContext(null)), false);
  assert.equal(isWaitingForUsers(null), false);
});
