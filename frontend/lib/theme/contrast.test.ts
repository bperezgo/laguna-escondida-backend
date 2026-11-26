import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { contrastRatio } from "./contrast.ts";

// The first definition is the light theme, the one the primary button uses.
const css = readFileSync(new URL("../../app/globals.css", import.meta.url), "utf8");

function token(name: string): string {
  const match = css.match(new RegExp(`--color-${name}:\\s*(#[0-9a-fA-F]{6})`));
  assert.ok(match?.[1], `--color-${name} is defined`);
  return match[1];
}

test("white text on the primary button is readable", () => {
  assert.ok(contrastRatio("#ffffff", token("primary")) >= 4.5);
  assert.ok(contrastRatio("#ffffff", token("primary-hover")) >= 4.5);
});

test("the accent teal fails as a button fill, which is why it is accent only", () => {
  assert.ok(contrastRatio("#ffffff", token("accent")) < 4.5);
});
