import { test } from "node:test";
import assert from "node:assert/strict";
import { LEGACY_EDGE_CONTEXT, parseSignInContext, signInBody, signInErrorMessage } from "./signIn.ts";

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
