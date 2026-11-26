import assert from "node:assert/strict";
import { test } from "node:test";

import { findShot } from "./product-shots.ts";

const shots = { "/src/assets/screens/hero.png": "hero", "/src/assets/screens/whatsapp.png": "list", "/src/assets/screens/whatsapp-2.webp": "order" };

test("a present image resolves by its name, whatever its extension", () => {
  assert.equal(findShot(shots, "hero"), "hero");
  assert.equal(findShot(shots, "whatsapp-2"), "order");
});

test("a name that is the start of another does not match it", () => {
  assert.equal(findShot(shots, "whatsapp"), "list");
});

test("a missing image throws and names it", () => {
  assert.throws(() => findShot(shots, "kitchen"), /Missing product image "kitchen" in src\/assets\/screens\//);
});
