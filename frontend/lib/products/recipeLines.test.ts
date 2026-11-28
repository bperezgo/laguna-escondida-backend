import { test } from "node:test";
import assert from "node:assert/strict";
import {
  addLine,
  availableCandidates,
  recipeChange,
  removeLine,
  setLineQuantity,
  validRecipeQuantity,
  type RecipeLineDraft,
} from "./recipeLines.ts";

const arroz = { key: "a", ingredientId: "p-arroz", quantity: "200" };
const frijol = { key: "f", ingredientId: "p-frijol", quantity: "150" };

test("zero is a valid quantity only where allowed", () => {
  assert.equal(validRecipeQuantity("0", true), true);
  assert.equal(validRecipeQuantity("0", false), false);
  assert.equal(validRecipeQuantity("0.5", false), true);
  assert.equal(validRecipeQuantity("", true), false);
  assert.equal(validRecipeQuantity("abc", true), false);
  assert.equal(validRecipeQuantity("-1", true), false);
});

test("candidates already in the recipe are not offered again", () => {
  const candidates = [
    { id: "p-arroz", name: "Arroz", unit: "g" },
    { id: "p-limon", name: "Limón", unit: "unit" },
  ];
  assert.deepEqual(
    availableCandidates([arroz], candidates).map((c) => c.id),
    ["p-limon"],
  );
});

test("each edit is reported as one change", () => {
  const added = addLine([arroz], "p-frijol", " 150 ");
  const change = recipeChange([arroz], added);
  assert.equal(change?.kind, "add");
  assert.equal(change?.kind === "add" && change.line.quantity, "150");

  assert.deepEqual(recipeChange([arroz, frijol], setLineQuantity([arroz, frijol], "f", "100")), {
    kind: "update",
    line: { ...frijol, quantity: "100" },
  });
  assert.deepEqual(recipeChange([arroz, frijol], removeLine([arroz, frijol], "a")), {
    kind: "remove",
    key: "a",
  });
  const same: RecipeLineDraft[] = [arroz];
  assert.equal(recipeChange(same, [...same]), null);
});
