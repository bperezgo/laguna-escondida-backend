import { test } from "node:test";
import assert from "node:assert/strict";
import { hasCloudRole, userErrorMessage } from "./user.ts";

test("admin, manager, accountant and agent are cloud roles", () => {
  for (const name of ["admin", "manager", "accountant", "agent"]) {
    assert.equal(hasCloudRole([name]), true, name);
  }
});

test("floor staff hold no cloud role", () => {
  assert.equal(hasCloudRole(["server", "cooker"]), false);
  assert.equal(hasCloudRole([]), false);
  assert.equal(hasCloudRole(["server", "manager"]), true);
});

test("the email errors read in Spanish", () => {
  assert.equal(userErrorMessage("Email is already in use"), "Ese correo ya lo usa otro usuario");
  assert.equal(
    userErrorMessage("email is required for a user with a cloud role"),
    "El correo es obligatorio para Administrador, Gerente, Contador y Agente"
  );
  assert.equal(userErrorMessage("User already exists"), "User already exists");
});
