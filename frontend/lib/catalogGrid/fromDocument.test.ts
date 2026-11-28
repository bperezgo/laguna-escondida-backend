import { test } from "node:test";
import assert from "node:assert/strict";
import { MAX_FILE_BYTES, parseDocumentText, readDocumentFile } from "./fromDocument.ts";
import { gridSkus, newRow, rowRef, type GridRow, type GridState } from "./state.ts";
import { toDocument } from "./toDocument.ts";
import type { GridType } from "./values.ts";

const limon = { id: "e-limon", name: "Limón", sku: "ING-LIM", product_type: "INGREDIENT", unit_of_measure: "unit" };

function row(id: string, type: GridType, patch: Partial<GridRow> = {}): GridRow {
  return { ...newRow(type, id), ...patch };
}

function file(text: string, size = text.length) {
  return { size, text: async () => text };
}

test("Oversized file", async () => {
  let read = false;
  const big = { size: 3 * 1024 * 1024, text: async () => ((read = true), "{}") };
  const got = await readDocumentFile(big, []);
  assert.equal(got.ok, false);
  assert.equal(!got.ok && got.reason, "too_large");
  assert.equal(read, false, "the file is not read past the size check");
  assert.equal((await readDocumentFile(file("{}", MAX_FILE_BYTES), [])).ok, true);
});

test("a file that is not JSON is refused", () => {
  const got = parseDocumentText("{ not json", []);
  assert.equal(!got.ok && got.reason, "invalid_json");
});

test("Malformed document", () => {
  const doc = { plates: [{ ref: "p", name: "Tilapia", category: "Platos", product_id: "123" }] };
  const got = parseDocumentText(JSON.stringify(doc), []);
  assert.equal(!got.ok && got.reason, "invalid_shape");
  assert.match(!got.ok ? got.message : "", /product_id/);
});

test("unknown sections and wrong types are refused", () => {
  for (const doc of [
    { menu: [] },
    { sellables: [{ ref: "a", name: 5, category: "x" }] },
    { sellables: {} },
    { ingredients: [{ ref: "a", name: "x", category: "y", product_type: "SELLABLE" }] },
    { plates: [{ ref: "p", name: "x", category: "y", recipe: [{ ingredient: "a", quantity: "1" }] }] },
    [],
    { defaults: { taxes_format: "fixed" } },
    { sellables: [{ ref: "a", name: "x", category: "y", vat: 0.19, taxes_format: "fixed" }] },
    { ingredients: [{ ref: "a", name: "x", category: "y", total_price_with_taxes: 1000 }] },
    { ingredients: [{ ref: "a", name: "x", category: "y", prep: { area: "kitchen" } }] },
    { existing: [{ ref: "lim", name: "Limón" }], opening_stock: [{ product: "lim", counted: 5 }] },
    { opening_stock: [{ product: "nadie", counted: 5 }] },
  ]) {
    const got = parseDocumentText(JSON.stringify(doc), []);
    assert.equal(!got.ok && got.reason, "invalid_shape", JSON.stringify(doc));
  }
});

test("a __proto__ key does not change Object.prototype", () => {
  const text = '{"__proto__":{"polluted":true},"sellables":[{"ref":"a","name":"x","category":"y","__proto__":{"polluted":true}}]}';
  const got = parseDocumentText(text, []);
  assert.equal(got.ok, false);
  const nested = '{"sellables":[{"ref":"a","name":"x","category":"y","prep":{"__proto__":{"polluted":true}}}]}';
  assert.equal(parseDocumentText(nested, []).ok, false);
  assert.equal(({} as Record<string, unknown>).polluted, undefined);
  assert.equal(Object.prototype.hasOwnProperty.call(Object.prototype, "polluted"), false);
});

test("an ingredient marked BOTH keeps its price and taxes", () => {
  const doc = { ingredients: [{ ref: "p", name: "Patacón", category: "x", product_type: "BOTH", total_price_with_taxes: 4000, vat: 0, ico: 8, taxes_format: "percentage" }] };
  const got = parseDocumentText(JSON.stringify(doc), []);
  assert.ok(got.ok);
  assert.deepEqual([got.state.rows[0].type, got.state.rows[0].price, got.state.rows[0].ico], ["BOTH", "4000", "8"]);
});

test("defaults are folded into the rows", () => {
  const doc = {
    defaults: { vat: 0, ico: 8, taxes_format: "percentage", unit_of_measure: "g", prep_area: "bar", prep_priority: 1 },
    sellables: [{ ref: "c", name: "Coca", category: "Bebidas", total_price_with_taxes: 5000 }],
    ingredients: [{ ref: "a", name: "Arroz", category: "Insumos" }],
  };
  const got = parseDocumentText(JSON.stringify(doc), []);
  assert.ok(got.ok);
  const [arroz, coca] = got.state.rows;
  assert.deepEqual([coca.vat, coca.ico, coca.unit, coca.area, coca.priority, coca.price], ["0", "8", "g", "bar", 1, "5000"]);
  assert.deepEqual([arroz.type, arroz.unit, arroz.vat, arroz.area], ["INGREDIENT", "g", "", ""]);
});

test("unmatched existing entries become marked links", () => {
  const doc = {
    existing: [
      { ref: "lim", name: "limón " },
      { ref: "yuca", sku: "ING-YUC" },
    ],
    plates: [
      {
        ref: "p",
        name: "Pescado",
        category: "Platos",
        recipe: [
          { ingredient: "lim", quantity: 1 },
          { ingredient: "yuca", quantity: 2 },
          { ingredient: "nadie", quantity: 3 },
        ],
      },
    ],
  };
  const got = parseDocumentText(JSON.stringify(doc), [limon]);
  assert.ok(got.ok);
  const ids = got.state.rows[0].recipe.map((l) => l.ingredientId);
  assert.equal(ids[0], "existing:e-limon");
  assert.match(ids[1], /^unmatched:/);
  assert.match(ids[2], /^unmatched:/);
  assert.deepEqual(
    got.state.unmatched.map((u) => u.sku ?? u.name),
    ["ING-YUC", "nadie"],
  );
});

test("Markup in a name is kept as text", () => {
  const name = "<img src=x onerror=alert(1)>";
  const got = parseDocumentText(JSON.stringify({ sellables: [{ ref: "a", name, category: "x" }] }), []);
  assert.ok(got.ok);
  assert.equal(got.state.rows[0].name, name);
});

test("Agent document reviewed: 120 products load as 120 rows", () => {
  const ingredients = Array.from({ length: 60 }, (_, i) => ({ ref: `i${i}`, name: `Insumo ${i}`, category: "Insumos" }));
  const plates = Array.from({ length: 60 }, (_, i) => ({
    ref: `p${i}`,
    name: `Plato ${i}`,
    category: "Platos",
    total_price_with_taxes: 20000,
    recipe: [{ ingredient: `i${i}`, quantity: 100 }],
    slots: [{ name: "Acompañante", choices: [{ name: "Papas", default: true, items: [{ ingredient: `i${i}`, quantity: 1 }] }, { name: "Sin" }] }],
  }));
  const got = parseDocumentText(JSON.stringify({ defaults: { vat: 0, ico: 8, taxes_format: "percentage", unit_of_measure: "unit", prep_area: "kitchen" }, ingredients, plates }), []);
  assert.ok(got.ok);
  assert.equal(got.state.rows.length, 120);
  const plate = got.state.rows[60];
  const ingredientRow = got.state.rows[0];
  assert.equal(plate.recipe[0].ingredientId, rowRef(ingredientRow.id));
  assert.equal(plate.slots[0].choices.length, 2);
  assert.equal(plate.slots[0].choices[0].isDefault, true);
});

test("more than 1,000 products are refused", () => {
  const sellables = Array.from({ length: 1001 }, (_, i) => ({ ref: `s${i}`, name: `S ${i}`, category: "x" }));
  const got = parseDocumentText(JSON.stringify({ sellables }), []);
  assert.equal(!got.ok && got.reason, "too_many_rows");
});

test("Round trip", () => {
  const grid: GridState = {
    unmatched: [],
    rows: [
      row("plate", "COMPOSITE", {
        name: "Tilapia frita",
        category: "Platos",
        price: "32000",
        recipe: [
          { key: "l1", ingredientId: rowRef("arroz"), quantity: "200" },
          { key: "l2", ingredientId: "existing:e-limon", quantity: "0.5" },
        ],
        slots: [
          {
            key: "s1",
            name: "Arroz",
            choices: [
              { key: "c1", name: "Blanco", isDefault: true, items: [{ key: "i1", ingredientProductId: rowRef("arroz"), quantity: "200" }] },
              { key: "c2", name: "Sin arroz", isDefault: false, items: [] },
            ],
          },
        ],
      }),
      row("arroz", "INGREDIENT", { name: "Arroz", category: "Insumos", unit: "g", stock: "5000" }),
      row("coca", "SELLABLE", { name: "Coca", category: "Bebidas", area: "bar", price: "5000", vat: "19", ico: "0", sku: "COCA1", skuTouched: true }),
    ],
  };
  const exported = JSON.stringify(toDocument(grid, [limon], []).document);
  const got = parseDocumentText(exported, [limon]);
  assert.ok(got.ok);

  const before = gridSkus(grid.rows, []);
  const after = gridSkus(got.state.rows, []);
  // Sections regroup the rows, so they are matched by name; the order is not part of the format.
  const loadedByName = new Map(got.state.rows.map((r) => [r.name, r.id]));
  const idOf = new Map(grid.rows.map((r) => [r.id, loadedByName.get(r.name)!]));
  const comparable = (rows: GridRow[], skus: Record<string, { value: string }>, mapId: (id: string) => string) =>
    [...rows].sort((a, b) => a.name.localeCompare(b.name)).map((r) => ({
      name: r.name,
      type: r.type,
      category: r.category,
      area: r.area,
      unit: r.unit,
      price: r.price,
      vat: r.vat,
      ico: r.ico,
      stock: r.stock,
      sku: skus[r.id].value,
      recipe: r.recipe.map((l) => [mapId(l.ingredientId), l.quantity]),
      slots: r.slots.map((s) => [s.name, s.choices.map((c) => [c.name, c.isDefault, c.items.map((it) => [mapId(it.ingredientProductId), it.quantity])])]),
    }));
  const forward = (id: string) => (id.startsWith("row:") ? rowRef(idOf.get(id.slice(4))!) : id);
  assert.deepEqual(comparable(got.state.rows, after, (id) => id), comparable(grid.rows, before, forward));
});
