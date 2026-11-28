import { test } from "node:test";
import assert from "node:assert/strict";
import { newRow, rowRef, type GridRow, type GridState } from "./state.ts";
import { locateProblem, toDocument } from "./toDocument.ts";
import type { GridType } from "./values.ts";

function row(id: string, type: GridType, patch: Partial<GridRow> = {}): GridRow {
  return { ...newRow(type, id), ...patch };
}

const limon = { id: "e-limon", name: "Limón", sku: "ING-LIM", product_type: "INGREDIENT", unit_of_measure: "unit" };

function fullGrid(): GridState {
  return {
    unmatched: [],
    rows: [
      row("tilapia", "COMPOSITE", {
        name: "Tilapia frita",
        category: "Platos",
        area: "kitchen",
        price: "32.000",
        ico: "8",
        vat: "0",
        recipe: [
          { key: "l1", ingredientId: rowRef("tilapia-cruda"), quantity: "1" },
          { key: "l2", ingredientId: "existing:e-limon", quantity: "0,5" },
        ],
        slots: [
          {
            key: "s1",
            name: "Arroz",
            choices: [
              { key: "c1", name: "Arroz blanco", isDefault: true, items: [{ key: "i1", ingredientProductId: rowRef("arroz"), quantity: "200" }] },
              { key: "c2", name: "Sin arroz", isDefault: false, items: [] },
            ],
          },
        ],
      }),
      row("arroz", "INGREDIENT", { name: "Arroz blanco", category: "Insumos", unit: "g", stock: "25000", price: "999" }),
      row("tilapia-cruda", "INGREDIENT", { name: "Tilapia entera", category: "Insumos" }),
      row("patacon", "BOTH", { name: "Patacón", category: "Acompañantes", price: "4000", ico: "8", vat: "0" }),
      row("coca", "SELLABLE", { name: "Coca-Cola 400ml", category: "Bebidas", area: "bar", price: "$5.000", ico: "0", vat: "19", stock: "24" }),
    ],
  };
}

test("a grid with every type builds the expected document", () => {
  const { document } = toDocument(fullGrid(), [limon], []);
  assert.deepEqual(document, {
    existing: [{ ref: "existing:e-limon", sku: "ING-LIM" }],
    ingredients: [
      { ref: "arroz", name: "Arroz blanco", category: "Insumos", sku: "ING-ARR-BLA", unit_of_measure: "g", product_type: "INGREDIENT" },
      { ref: "tilapia-cruda", name: "Tilapia entera", category: "Insumos", sku: "ING-TIL-ENT", unit_of_measure: "unit", product_type: "INGREDIENT" },
      {
        ref: "patacon",
        name: "Patacón",
        category: "Acompañantes",
        sku: "COC-PAT",
        unit_of_measure: "unit",
        total_price_with_taxes: 4000,
        vat: 0,
        ico: 8,
        taxes_format: "percentage",
        prep: { area: "kitchen", priority: 0 },
        product_type: "BOTH",
      },
    ],
    sellables: [
      {
        ref: "coca",
        name: "Coca-Cola 400ml",
        category: "Bebidas",
        sku: "BAR-COC-COL-400",
        unit_of_measure: "unit",
        total_price_with_taxes: 5000,
        vat: 19,
        ico: 0,
        taxes_format: "percentage",
        prep: { area: "bar", priority: 0 },
      },
    ],
    plates: [
      {
        ref: "tilapia",
        name: "Tilapia frita",
        category: "Platos",
        sku: "COC-TIL-FRI",
        unit_of_measure: "unit",
        total_price_with_taxes: 32000,
        vat: 0,
        ico: 8,
        taxes_format: "percentage",
        prep: { area: "kitchen", priority: 0 },
        recipe: [
          { ingredient: "tilapia-cruda", quantity: 1 },
          { ingredient: "existing:e-limon", quantity: 0.5 },
        ],
        slots: [
          {
            name: "Arroz",
            choices: [
              { name: "Arroz blanco", default: true, items: [{ ingredient: "arroz", quantity: 200 }] },
              { name: "Sin arroz", default: false, items: [] },
            ],
          },
        ],
      },
    ],
    opening_stock: [
      { product: "arroz", counted: 25000 },
      { product: "coca", counted: 24 },
    ],
  });
});

test("Ingredient row has no price: the row is sent without price or taxes", () => {
  const { document } = toDocument(fullGrid(), [limon], []);
  const arroz = document.ingredients![0];
  for (const key of ["total_price_with_taxes", "vat", "ico", "taxes_format", "prep"]) {
    assert.equal(key in arroz, false, key);
  }
});

test("taxes are sent as the percentages the import expects", () => {
  const { document } = toDocument(fullGrid(), [limon], []);
  assert.deepEqual([document.sellables![0].vat, document.sellables![0].ico], [19, 0]);
});

test("an unreadable quantity is sent as 0, never as null", () => {
  const grid = fullGrid();
  grid.rows[0].recipe[0].quantity = "mucho";
  const json = JSON.stringify(toDocument(grid, [limon], []).document);
  assert.equal(json.includes("null"), false);
});

test("problem paths land on the right row, cell and panel", () => {
  const { pointers } = toDocument(fullGrid(), [limon], []);
  const at = (path: string, code = "invalid_field", message = "x") => locateProblem(pointers, { path, code, message });

  assert.deepEqual(at("/sellables/0", "name_conflict"), { rowId: "coca", cell: "name" });
  assert.deepEqual(at("/sellables/0", "sku_conflict"), { rowId: "coca", cell: "sku" });
  assert.deepEqual(at("/ingredients/2", "missing_field", "total_price_with_taxes is required"), { rowId: "patacon", cell: "price" });
  assert.deepEqual(at("/plates/0", "missing_field", "prep.area is required"), { rowId: "tilapia", cell: "area" });
  assert.deepEqual(at("/plates/0/recipe/1", "unknown_ref"), {
    rowId: "tilapia",
    cell: "detail",
    panel: { kind: "recipe", key: "l2" },
  });
  assert.deepEqual(at("/plates/0/slots/0/choices/0/items/0", "invalid_quantity"), {
    rowId: "tilapia",
    cell: "detail",
    panel: { kind: "slot", key: "s1" },
  });
  assert.deepEqual(at("/opening_stock/1", "invalid_quantity"), { rowId: "coca", cell: "stock" });
});

test("an unknown deeper path falls back to its longest known prefix", () => {
  const { pointers } = toDocument(fullGrid(), [limon], []);
  assert.deepEqual(locateProblem(pointers, { path: "/plates/0/slots/0/choices/7", code: "invalid_slot", message: "" }), {
    rowId: "tilapia",
    cell: "detail",
    panel: { kind: "slot", key: "s1" },
  });
  assert.deepEqual(locateProblem(pointers, { path: "/sellables/0/prep/area", code: "invalid_field", message: "prep.area is too long" }), {
    rowId: "coca",
    cell: "area",
  });
});

test("problems with no row go above the grid", () => {
  const { pointers } = toDocument(fullGrid(), [limon], []);
  assert.equal(locateProblem(pointers, { path: "/existing/0", code: "existing_not_found", message: "" }), null);
  assert.equal(locateProblem(pointers, { path: "/expected_digest", code: "digest_mismatch", message: "" }), null);
  assert.equal(locateProblem(pointers, { path: "", code: "limit_exceeded", message: "" }), null);
});

test("an unmatched existing entry is still sent so the server can report it", () => {
  const grid: GridState = { rows: [], unmatched: [{ key: "u1", name: "Yuca" }] };
  assert.deepEqual(toDocument(grid, [], []).document, { existing: [{ ref: "unmatched:u1", name: "Yuca" }] });
});
