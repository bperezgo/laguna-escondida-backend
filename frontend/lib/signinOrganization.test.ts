import { test } from "node:test";
import assert from "node:assert/strict";
import { normalizeOrganizationSlug, signInBody } from "./signinOrganization.ts";

test("the slug is trimmed and lowercased", () => {
  assert.equal(normalizeOrganizationSlug("  Laguna-Escondida "), "laguna-escondida");
});

test("the sign-in body names the organization when one is given", () => {
  assert.deepEqual(signInBody(" Laguna-Escondida", "cajero1", "secret"), {
    organization: "laguna-escondida",
    username: "cajero1",
    password: "secret",
  });
});

test("the sign-in body leaves the organization out when it is empty", () => {
  assert.deepEqual(signInBody("   ", "cajero1", "secret"), { username: "cajero1", password: "secret" });
});
