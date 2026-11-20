import { test } from "node:test";
import assert from "node:assert/strict";
import { hasCloudRole, roleLabel, userErrorMessage } from "./user.ts";

test("admin, manager, accountant, agent and courier are cloud roles", () => {
  for (const name of ["admin", "manager", "accountant", "agent", "courier"]) {
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
    "El correo es obligatorio para Administrador, Gerente, Contador, Agente y Repartidor"
  );
  assert.equal(userErrorMessage("User already exists"), "User already exists");
});

test("the courier role reads as Repartidor", () => {
  assert.equal(roleLabel("courier"), "Repartidor");
});
