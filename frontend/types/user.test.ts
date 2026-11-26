import { test } from "node:test";
import assert from "node:assert/strict";
import { requiresEmail, roleLabel, userErrorMessage } from "./user.ts";

test("admin, accountant, agent and courier require an email", () => {
  for (const name of ["admin", "accountant", "agent", "courier"]) {
    assert.equal(requiresEmail([name]), true, name);
  }
});

test("a manager and floor staff may go without an email", () => {
  assert.equal(requiresEmail(["manager"]), false);
  assert.equal(requiresEmail(["server", "cooker"]), false);
  assert.equal(requiresEmail([]), false);
  assert.equal(requiresEmail(["manager", "accountant"]), true);
});

test("the email errors read in Spanish", () => {
  assert.equal(userErrorMessage("Email is already in use"), "Ese correo ya lo usa otro usuario");
  assert.equal(
    userErrorMessage("email is required for an admin, accountant, agent or courier"),
    "El correo es obligatorio para Administrador, Contador, Agente y Repartidor"
  );
  assert.equal(userErrorMessage("User already exists"), "User already exists");
});

test("the courier role reads as Repartidor", () => {
  assert.equal(roleLabel("courier"), "Repartidor");
});
