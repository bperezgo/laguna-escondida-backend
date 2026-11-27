import { test } from "node:test";
import assert from "node:assert/strict";
import { generateSku, initialSkuField, skuFieldValue, takenSkus, typeSku } from "./sku.ts";

// One case per row of the example table in openspec specs/product-sku/spec.md.
const EXAMPLES: [name: string, type: string, area: string, taken: string[], want: string][] = [
  ["Bandeja paisa", "COMPOSITE", "kitchen", [], "COC-BAN-PAI"],
  ["Coca-Cola 400ml", "SELLABLE", "bar", [], "BAR-COC-COL-400"],
  ["Limonada de coco", "SELLABLE", "bar", [], "BAR-LIM-COC"],
  ["Limonada de coca", "SELLABLE", "bar", ["BAR-LIM-COC"], "BAR-LIM-COC-2"],
  ["Limonada de cereza", "SELLABLE", "bar", ["BAR-LIM-CER", "bar-lim-cer-2"], "BAR-LIM-CER-3"],
  ["Arroz", "INGREDIENT", "", [], "ING-ARR"],
  ["Piña colada", "SELLABLE", "bar", [], "BAR-PIN-COL"],
  ["Té de la casa", "SELLABLE", "", [], "PRD-TE-CAS"],
  ["Churrasco a la parrilla con papas", "COMPOSITE", "grill", [], "PAR-CHU-PAR-PAP"],
  ["Postre del día", "SELLABLE", "terraza", [], "TER-POS-DIA"],
  ["¡¡!!", "SELLABLE", "bar", [], "BAR"],
];

for (const [name, type, area, taken, want] of EXAMPLES) {
  test(`generateSku: ${name}`, () => {
    assert.equal(generateSku(name, type, area, takenSkus(taken)), want);
  });
}

const none = takenSkus([]);

test("form: suggestion follows the name", () => {
  const field = initialSkuField();
  assert.equal(skuFieldValue(field, { name: "", productType: "SELLABLE", area: "kitchen" }, none), "");
  assert.equal(
    skuFieldValue(field, { name: "Bandeja paisa", productType: "SELLABLE", area: "kitchen" }, none),
    "COC-BAN-PAI",
  );
});

test("form: suggestion avoids an existing SKU", () => {
  const taken = takenSkus(["BAR-LIM-COC"]);
  const inputs = { name: "Limonada de coca", productType: "SELLABLE", area: "bar" };
  assert.equal(skuFieldValue(initialSkuField(), inputs, taken), "BAR-LIM-COC-2");
});

test("form: user's SKU is kept, and emptying resumes suggesting", () => {
  let field = typeSku(initialSkuField(), "LIMCOCO");
  const inputs = { name: "Limonada de coco", productType: "SELLABLE", area: "bar" };
  assert.equal(skuFieldValue(field, inputs, none), "LIMCOCO");
  assert.equal(skuFieldValue(field, { ...inputs, name: "Limonada de mango" }, none), "LIMCOCO");

  field = typeSku(field, "");
  assert.equal(skuFieldValue(field, inputs, none), "BAR-LIM-COC");
});

test("form: editing never regenerates", () => {
  let field = initialSkuField("OLD1");
  const inputs = { name: "Bandeja paisa", productType: "COMPOSITE", area: "kitchen" };
  assert.equal(skuFieldValue(field, inputs, none), "OLD1");

  field = typeSku(field, "");
  assert.equal(skuFieldValue(field, inputs, none), "");
});
