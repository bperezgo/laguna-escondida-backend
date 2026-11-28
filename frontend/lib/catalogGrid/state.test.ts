import { test } from "node:test";
import assert from "node:assert/strict";
import {
  cellApplies,
  EMPTY_STATE,
  gridReducer,
  gridSkus,
  ingredientCandidates,
  newRow,
  platesUsing,
  rowRef,
  typeChangeImpact,
  type GridRow,
  type GridState,
} from "./state.ts";
import type { GridType } from "./values.ts";

function row(id: string, type: GridType, patch: Partial<GridRow> = {}): GridRow {
  return { ...newRow(type, id), ...patch };
}

function slotWith(ref: string) {
  return {
    key: "s1",
    name: "Acompañante",
    choices: [
      { key: "c1", name: "Con arroz", isDefault: true, items: [{ key: "i1", ingredientProductId: ref, quantity: "200" }] },
      { key: "c2", name: "Sin arroz", isDefault: false, items: [] },
    ],
  };
}

function state(...rows: GridRow[]): GridState {
  return { rows, unmatched: [] };
}

test("Ingredient row has no price", () => {
  for (const column of ["price", "ico", "vat"] as const) {
    assert.equal(cellApplies("INGREDIENT", column), false, column);
    assert.equal(cellApplies("SELLABLE", column), true, column);
  }
  const s = gridReducer(state(row("a", "SELLABLE")), { type: "changeType", id: "a", productType: "INGREDIENT" });
  assert.equal(s.rows[0].type, "INGREDIENT");
});

test("opening stock and plate detail follow the type", () => {
  assert.equal(cellApplies("COMPOSITE", "stock"), false);
  assert.equal(cellApplies("COMPOSITE", "detail"), true);
  assert.equal(cellApplies("BOTH", "detail"), false);
});

test("Changing a plate's type", () => {
  const plate = row("p", "COMPOSITE", {
    name: "Bandeja paisa",
    recipe: [{ key: "l1", ingredientId: rowRef("a"), quantity: "200" }],
    slots: [slotWith(rowRef("a"))],
  });
  const s = state(row("a", "INGREDIENT", { name: "Arroz" }), plate);

  assert.deepEqual(typeChangeImpact(s, "p", "SELLABLE"), { plates: [], dropsPlateDetail: true });
  const next = gridReducer(s, { type: "changeType", id: "p", productType: "SELLABLE" });
  assert.deepEqual(next.rows[1].recipe, []);
  assert.deepEqual(next.rows[1].slots, []);
});

test("Deleting an ingredient used by plates", () => {
  const s = state(
    row("arroz", "INGREDIENT", { name: "Arroz" }),
    row("frijol", "INGREDIENT", { name: "Frijol" }),
    row("bandeja", "COMPOSITE", {
      name: "Bandeja paisa",
      recipe: [
        { key: "l1", ingredientId: rowRef("arroz"), quantity: "200" },
        { key: "l2", ingredientId: rowRef("frijol"), quantity: "150" },
      ],
    }),
    row("pollo", "COMPOSITE", { name: "Arroz con pollo", slots: [slotWith(rowRef("arroz"))] }),
    row("limonada", "SELLABLE", { name: "Limonada" }),
  );

  assert.deepEqual(platesUsing(s, ["arroz"]), ["Bandeja paisa", "Arroz con pollo"]);
  const next = gridReducer(s, { type: "delete", ids: ["arroz"] });
  assert.equal(next.rows.some((r) => r.id === "arroz"), false);
  assert.deepEqual(next.rows.find((r) => r.id === "bandeja")!.recipe.map((l) => l.ingredientId), [rowRef("frijol")]);
  assert.deepEqual(next.rows.find((r) => r.id === "pollo")!.slots[0].choices[0].items, []);
});

test("Ingredient becomes a sellable", () => {
  const s = state(
    row("arroz", "INGREDIENT", { name: "Arroz" }),
    row("pollo", "COMPOSITE", { name: "Arroz con pollo", slots: [slotWith(rowRef("arroz"))] }),
  );
  assert.deepEqual(typeChangeImpact(s, "arroz", "SELLABLE"), { plates: ["Arroz con pollo"], dropsPlateDetail: false });
  const next = gridReducer(s, { type: "changeType", id: "arroz", productType: "SELLABLE" });
  assert.deepEqual(next.rows[1].slots[0].choices[0].items, []);
});

test("Ingredient becomes Ambos", () => {
  const s = state(
    row("club", "INGREDIENT", { name: "Cerveza Club" }),
    row("p", "COMPOSITE", { name: "Michelada", recipe: [{ key: "l1", ingredientId: rowRef("club"), quantity: "1" }] }),
  );
  assert.equal(typeChangeImpact(s, "club", "BOTH"), null);
  const next = gridReducer(s, { type: "changeType", id: "club", productType: "BOTH" });
  assert.equal(next.rows[1].recipe.length, 1);
  assert.equal(next.rows[0].ico, "8");
  assert.equal(next.rows[0].vat, "0");
});

test("a plate can use an ingredient row placed after it", () => {
  const s = state(row("p", "COMPOSITE", { name: "Bandeja" }), row("a", "INGREDIENT", { name: "Arroz" }));
  const ids = ingredientCandidates(s, "p", []).map((c) => c.id);
  assert.deepEqual(ids, [rowRef("a")]);
});

test("candidates leave out plates, sellables and the plate itself", () => {
  const s = state(
    row("p", "COMPOSITE", { name: "Bandeja" }),
    row("q", "COMPOSITE", { name: "Otro plato" }),
    row("s", "SELLABLE", { name: "Coca" }),
    row("b", "BOTH", { name: "Patacón" }),
  );
  const existing = [
    { id: "e1", name: "Limón", sku: "ING-LIM", product_type: "INGREDIENT", unit_of_measure: "unit" },
    { id: "e2", name: "Sopa", sku: "COC-SOP", product_type: "COMPOSITE", unit_of_measure: "unit" },
  ];
  assert.deepEqual(
    ingredientCandidates(s, "p", existing).map((c) => c.id),
    [rowRef("b"), "existing:e1"],
  );
});

test("Two rows with the same name", () => {
  const rows = [
    row("a", "SELLABLE", { name: "Limonada de coco", area: "bar" }),
    row("b", "SELLABLE", { name: "Limonada de coco", area: "bar" }),
  ];
  const skus = gridSkus(rows, []);
  assert.deepEqual(skus.a, { value: "BAR-LIM-COC", suggested: true });
  assert.deepEqual(skus.b, { value: "BAR-LIM-COC-2", suggested: true });
});

test("suggestions avoid the organization's SKUs and typed SKUs further down", () => {
  const rows = [
    row("a", "SELLABLE", { name: "Limonada de coco", area: "bar" }),
    row("b", "SELLABLE", { name: "Otra", area: "bar", sku: "BAR-LIM-COC-2", skuTouched: true }),
  ];
  assert.equal(gridSkus(rows, ["bar-lim-coc"]).a.value, "BAR-LIM-COC-3");
});

test("Typed SKU kept", () => {
  let s = state(row("a", "SELLABLE", { name: "Limonada de coco", area: "bar" }));
  s = gridReducer(s, { type: "edit", id: "a", column: "sku", value: "LIMCOCO" });
  s = gridReducer(s, { type: "edit", id: "a", column: "name", value: "Limonada natural" });
  assert.deepEqual(gridSkus(s.rows, []).a, { value: "LIMCOCO", suggested: false });

  s = gridReducer(s, { type: "edit", id: "a", column: "sku", value: "" });
  assert.deepEqual(gridSkus(s.rows, []).a, { value: "BAR-LIM-NAT", suggested: true });
});

test("New row defaults", () => {
  const s = gridReducer(EMPTY_STATE, { type: "add", productType: "SELLABLE" });
  assert.equal(s.rows[0].ico, "8");
  assert.equal(s.rows[0].vat, "0");
  const ing = gridReducer(EMPTY_STATE, { type: "add", productType: "INGREDIENT" });
  assert.equal(ing.rows[0].ico, "");
  assert.equal(ing.rows[0].vat, "");
});

test("Clear ICO and fill IVA", () => {
  let s = state(
    row("p1", "SELLABLE"),
    row("p2", "SELLABLE"),
    row("p3", "SELLABLE"),
    row("i1", "INGREDIENT"),
    row("i2", "INGREDIENT"),
  );
  s = gridReducer(s, { type: "clearTax", column: "ico" });
  s = gridReducer(s, { type: "fillTax", column: "vat", value: "19" });
  for (const r of s.rows.slice(0, 3)) assert.deepEqual([r.ico, r.vat], ["0", "19"], r.id);
  for (const r of s.rows.slice(3)) assert.deepEqual([r.ico, r.vat], ["", ""], r.id);
});

test("Fill selected rows", () => {
  let s = state(...["a", "b", "c", "d", "e"].map((id) => row(id, "SELLABLE")));
  s = gridReducer(s, { type: "fillTax", column: "vat", value: "19", ids: ["b", "d"] });
  assert.deepEqual(s.rows.map((r) => r.vat), ["0", "19", "0", "19", "0"]);
});

test("duplicate places a copy below with a fresh id and a suggested SKU", () => {
  const s = gridReducer(state(row("a", "SELLABLE", { name: "Coca", sku: "X", skuTouched: true }), row("b", "SELLABLE")), {
    type: "duplicate",
    id: "a",
  });
  assert.equal(s.rows.length, 3);
  assert.equal(s.rows[1].name, "Coca");
  assert.notEqual(s.rows[1].id, "a");
  assert.equal(s.rows[1].skuTouched, false);
  assert.equal(s.rows[2].id, "b");
});

test("rows stop at 1,000", () => {
  let s: GridState = { rows: Array.from({ length: 1000 }, (_, i) => row(`r${i}`, "SELLABLE")), unmatched: [] };
  s = gridReducer(s, { type: "add" });
  assert.equal(s.rows.length, 1000);
});
