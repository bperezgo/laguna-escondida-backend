import { test } from "node:test";
import assert from "node:assert/strict";
import { googleClientId } from "./google.ts";

test("an unset or blank client id disables the Google button", () => {
  assert.equal(googleClientId(undefined), null);
  assert.equal(googleClientId(""), null);
  assert.equal(googleClientId("  "), null);
});

test("a client id is used trimmed", () => {
  assert.equal(googleClientId(" 123.apps.googleusercontent.com "), "123.apps.googleusercontent.com");
});
