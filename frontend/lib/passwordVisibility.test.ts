import { test } from "node:test";
import assert from "node:assert/strict";
import { inputType, passwordVisibilityReducer, toggleLabel } from "./passwordVisibility.ts";

test("the toggle is named for what pressing it does", () => {
  assert.equal(toggleLabel(false), "Mostrar contraseña");
  assert.equal(toggleLabel(true), "Ocultar contraseña");
});

test("a shown password is a text input and a hidden one a password input", () => {
  assert.equal(inputType(true), "text");
  assert.equal(inputType(false), "password");
});

test("toggling flips the state", () => {
  assert.equal(passwordVisibilityReducer(false, "toggled"), true);
  assert.equal(passwordVisibilityReducer(true, "toggled"), false);
});

test("submitting or resetting always hides the password", () => {
  for (const shown of [true, false]) {
    assert.equal(passwordVisibilityReducer(shown, "submitted"), false);
    assert.equal(passwordVisibilityReducer(shown, "reset"), false);
  }
});
