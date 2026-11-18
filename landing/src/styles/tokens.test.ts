import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { contrastRatio } from "../lib/contrast.ts";

const css = readFileSync(new URL("./global.css", import.meta.url), "utf8");

function token(name: string): string {
  const match = css.match(new RegExp(`--color-${name}:\\s*(#[0-9a-fA-F]{6})`));
  assert.ok(match?.[1], `--color-${name} is defined`);
  return match[1];
}

test("primary button text is readable", () => {
  assert.ok(contrastRatio(token("on-primary"), token("primary")) >= 4.5);
  assert.ok(contrastRatio(token("on-primary"), token("primary-strong")) >= 4.5);
});

test("body text and links are readable on the page", () => {
  assert.ok(contrastRatio(token("ink"), token("page")) >= 7);
  assert.ok(contrastRatio(token("muted"), token("page")) >= 4.5);
  assert.ok(contrastRatio(token("primary"), token("page")) >= 4.5);
});

test("night section text is readable", () => {
  assert.ok(contrastRatio(token("on-night"), token("night")) >= 7);
});

test("the accent teal fails as a button fill, which is why it is accent only", () => {
  assert.ok(contrastRatio("#ffffff", token("accent")) < 4.5);
});
