import { test } from "node:test";
import assert from "node:assert/strict";
import { parseClipboard, pasteCell, pasteCells } from "./paste.ts";
import { EMPTY_STATE, gridReducer, gridSkus, newRow, pasteImpact, rowRef, type GridState } from "./state.ts";
import { parseAmount } from "./values.ts";

test("Paste names and prices", () => {
  const text = Array.from({ length: 30 }, (_, i) => `Plato ${i + 1}\t${(i + 1) * 1000}`).join("\n") + "\n";
  const s = gridReducer(EMPTY_STATE, {
    type: "paste",
    startId: null,
    startColumn: "name",
    order: [],
    cells: pasteCells(text, "name"),
  });
  assert.equal(s.rows.length, 30);
  assert.deepEqual([s.rows[0].name, s.rows[0].price], ["Plato 1", "1000"]);
  assert.deepEqual([s.rows[29].name, s.rows[29].price], ["Plato 30", "30000"]);
  const skus = gridSkus(s.rows, []);
  for (const r of s.rows) assert.ok(skus[r.id].value && skus[r.id].suggested, r.name);
});

test("Bad value in a paste", () => {
  const s = gridReducer(EMPTY_STATE, {
    type: "paste",
    startId: null,
    startColumn: "name",
    order: [],
    cells: pasteCells("Bandeja\ttreinta mil\tPlato\tPlatos\tCocina\tund\t8\t0", "name"),
  });
  const r = s.rows[0];
  assert.equal(r.price, "");
  assert.equal(r.marks.price, true);
  assert.deepEqual([r.name, r.type, r.category, r.area, r.unit, r.ico, r.vat], ["Bandeja", "COMPOSITE", "Platos", "kitchen", "unit", "8", "0"]);

  const edited = gridReducer(s, { type: "edit", id: r.id, column: "price", value: "30000" });
  assert.equal(edited.rows[0].marks.price, undefined);
});

test("a paste fills down from the focused cell into existing rows, then adds rows", () => {
  const start: GridState = { rows: [newRow("SELLABLE", "a"), newRow("SELLABLE", "b"), newRow("SELLABLE", "c")], unmatched: [] };
  const s = gridReducer(start, {
    type: "paste",
    startId: "b",
    startColumn: "price",
    order: ["a", "b", "c"],
    cells: pasteCells("1000\tPlato\n2000\n3000", "price"),
  });
  assert.deepEqual(s.rows.map((r) => r.price), ["", "1000", "2000", "3000"]);
  assert.equal(s.rows[1].type, "COMPOSITE");
});

test("unknown types and units are marked; empty ones keep the row's value", () => {
  assert.deepEqual(pasteCell("type", "Bebida"), { invalid: true });
  assert.deepEqual(pasteCell("unit", "docena"), { invalid: true });
  assert.deepEqual(pasteCell("type", ""), { skip: true });
  assert.deepEqual(pasteCell("type", "ingrediente"), { value: "INGREDIENT" });
  assert.deepEqual(pasteCell("type", "BOTH"), { value: "BOTH" });
  assert.deepEqual(pasteCell("unit", "Kg"), { value: "kg" });
  assert.deepEqual(pasteCell("unit", "gramos"), { value: "g" });
});

test("Colombian price formats", () => {
  for (const [raw, want] of [
    ["32000", 32000],
    ["32.000", 32000],
    ["$32.000", 32000],
    ["$ 32.000", 32000],
    ["1.250.000", 1250000],
    ["4.500,50", 4500.5],
    ["1,500", 1500],
    ["3,5", 3.5],
    ["3.5", 3.5],
  ] as const) {
    assert.equal(parseAmount(raw), want, raw);
  }
  for (const raw of ["treinta mil", "32.00.0", "-5", "1e3"]) assert.equal(parseAmount(raw), null, raw);
});

test("percent and count cells", () => {
  assert.deepEqual(pasteCell("vat", "19%"), { value: "19" });
  assert.deepEqual(pasteCell("ico", "8"), { value: "8" });
  assert.deepEqual(pasteCell("ico", "120"), { invalid: true });
  assert.deepEqual(pasteCell("stock", "25.000"), { value: "25000" });
  assert.deepEqual(pasteCell("stock", "2,5"), { invalid: true });
});

test("the 1,000-row cap", () => {
  const text = Array.from({ length: 1200 }, (_, i) => `P${i}`).join("\n");
  assert.equal(pasteCells(text, "name").length, 1000);

  const start: GridState = { rows: Array.from({ length: 990 }, (_, i) => newRow("SELLABLE", `r${i}`)), unmatched: [] };
  const s = gridReducer(start, { type: "paste", startId: null, startColumn: "name", order: [], cells: pasteCells(text, "name") });
  assert.equal(s.rows.length, 1000);
});

test("quoted cells keep tabs and newlines", () => {
  assert.deepEqual(parseClipboard('"Arroz\ncon pollo"\t12000\r\n"Say ""hi"""\t1'), [
    ["Arroz\ncon pollo", "12000"],
    ['Say "hi"', "1"],
  ]);
});

test("a pasted type that takes an ingredient out of plates is reported before it is applied", () => {
  const start: GridState = {
    unmatched: [],
    rows: [
      { ...newRow("INGREDIENT", "arroz"), name: "Arroz" },
      { ...newRow("COMPOSITE", "p"), name: "Bandeja", recipe: [{ key: "l1", ingredientId: rowRef("arroz"), quantity: "200" }] },
    ],
  };
  const toSellable = { type: "paste" as const, startId: "arroz", startColumn: "type" as const, order: ["arroz", "p"], cells: pasteCells("Producto", "type") };
  assert.deepEqual(pasteImpact(start, toSellable), ["Bandeja"]);
  assert.deepEqual(start.rows[1].recipe.length, 1, "the impact check changes nothing");

  const toBoth = { ...toSellable, cells: pasteCells("Ambos", "type") };
  assert.deepEqual(pasteImpact(start, toBoth), []);
  const names = { ...toSellable, startColumn: "name" as const, cells: pasteCells("Arroz blanco", "name") };
  assert.deepEqual(pasteImpact(start, names), []);
});
