import { test } from "node:test";
import assert from "node:assert/strict";
import { LEGACY_EDGE_CONTEXT, parseSignInContext, signInBody, signInErrorMessage, signInFailure, signInOutcome } from "./signIn.ts";
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

test("a pending account goes to the pending page", () => {
  assert.deepEqual(signInOutcome(false, { error: "Approval pending", code: "pending_approval" }, null), { next: "pending" });
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
