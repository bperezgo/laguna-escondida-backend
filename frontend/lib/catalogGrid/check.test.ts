import { test } from "node:test";
import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import type { CatalogImportResult } from "../../types/catalogImport.ts";
import {
  APPLY_URL,
  canCreate,
  checkCatalog,
  createCatalog,
  passedCheck,
  placeProblems,
  VALIDATE_URL,
  type FetchLike,
} from "./check.ts";
import { parseDocumentText } from "./fromDocument.ts";
import { panelProblem } from "./panel.ts";
import { gridReducer, ingredientCandidates, newRow, plateSummary, rowRef, type GridRow, type GridState } from "./state.ts";
import { toDocument } from "./toDocument.ts";
import type { GridType } from "./values.ts";

function row(id: string, type: GridType, patch: Partial<GridRow> = {}): GridRow {
  return { ...newRow(type, id), category: "General", ...patch };
}

const summary = {
  products: 0,
  ingredients: 0,
  sellables: 0,
  plates: 0,
  existing: 0,
  recipe_lines: 0,
  option_slots: 0,
  option_choices: 0,
  preparation_areas: 0,
  opening_stock: 0,
  generated_skus: [],
};

function result(patch: Partial<CatalogImportResult>): CatalogImportResult {
  return { valid: true, applied: false, problems: [], summary, digest: "d1", ...patch };
}

function server(responses: { status: number; body: unknown }[]) {
  const calls: { url: string; body: Record<string, unknown> }[] = [];
  const fetchFn: FetchLike = async (url, init) => {
    calls.push({ url, body: JSON.parse(String(init.body)) });
    const next = responses.shift()!;
    return new Response(JSON.stringify(next.body), { status: next.status });
  };
  return { calls, fetchFn };
}

function twelveRows(): GridState {
  return { rows: Array.from({ length: 12 }, (_, i) => row(`r${i + 1}`, "SELLABLE", { name: `P${i + 1}`, price: "1000" })), unmatched: [] };
}

test("Problem shown on its row", () => {
  const { pointers } = toDocument(twelveRows(), [], []);
  const placed = placeProblems(
    [
      { path: "/sellables/11", code: "name_conflict", message: 'a product named "P12" already exists' },
      { path: "/existing/0", code: "existing_not_found", message: "no product" },
    ],
    pointers,
  );
  assert.deepEqual(placed.cells.r12, { name: ['a product named "P12" already exists'] });
  assert.deepEqual(placed.general.map((p) => p.code), ["existing_not_found"]);
});

test("a problem inside a plate lands on its detail cell and its panel", () => {
  const grid: GridState = {
    unmatched: [],
    rows: [
      row("a", "INGREDIENT", { name: "Arroz" }),
      row("p", "COMPOSITE", { name: "Bandeja", price: "20000", recipe: [{ key: "l1", ingredientId: rowRef("a"), quantity: "200" }] }),
    ],
  };
  const { pointers } = toDocument(grid, [], []);
  const placed = placeProblems([{ path: "/plates/0/recipe/0", code: "invalid_quantity", message: "bad" }], pointers);
  assert.deepEqual(placed.cells.p, { detail: ["bad"] });
  assert.deepEqual(placed.panels.p, [{ panel: { kind: "recipe", key: "l1" }, message: "bad" }]);
});

test("Edit after a passing check", async () => {
  let grid = twelveRows();
  const { fetchFn } = server([{ status: 200, body: result({}) }]);
  const before = toDocument(grid, [], []).document;
  const check = passedCheck(await checkCatalog(fetchFn, before), before);
  assert.ok(check);
  assert.equal(canCreate(check, toDocument(grid, [], []).document), true);

  grid = gridReducer(grid, { type: "edit", id: "r3", column: "price", value: "2000" });
  assert.equal(canCreate(check, toDocument(grid, [], []).document), false);
});

test("a check with problems never enables create", async () => {
  const document = toDocument(twelveRows(), [], []).document;
  const { fetchFn } = server([{ status: 200, body: result({ valid: false, problems: [{ path: "/sellables/0", code: "x", message: "y" }] }) }]);
  assert.equal(passedCheck(await checkCatalog(fetchFn, document), document), null);
});

test("Successful create", async () => {
  const document = toDocument(twelveRows(), [], []).document;
  const { calls, fetchFn } = server([
    { status: 200, body: result({ digest: "abc" }) },
    { status: 201, body: result({ applied: true, digest: "abc", summary: { ...summary, products: 12 } }) },
  ]);
  const check = passedCheck(await checkCatalog(fetchFn, document), document)!;
  let cleared = 0;
  const created = await createCatalog(fetchFn, document, check, () => cleared++);

  assert.deepEqual(created, { kind: "created", products: 12 });
  assert.equal(cleared, 1);
  assert.equal(calls[0].url, VALIDATE_URL);
  assert.equal(calls[1].url, APPLY_URL);
  const { expected_digest, ...sent } = calls[1].body;
  assert.equal(expected_digest, "abc");
  assert.deepEqual(sent, calls[0].body);
});

test("Create refused by the server", async () => {
  const document = toDocument(twelveRows(), [], []).document;
  const problems = [{ path: "/sellables/0", code: "name_conflict", message: "taken" }];
  for (const answer of [
    { status: 422, body: result({ valid: false, problems }) },
    { status: 409, body: result({ valid: false, problems: [{ path: "/expected_digest", code: "digest_mismatch", message: "changed" }] }) },
    { status: 500, body: { error: "boom" } },
  ]) {
    const { fetchFn } = server([answer]);
    let cleared = 0;
    const got = await createCatalog(fetchFn, document, { digest: "abc", documentHash: "h" }, () => cleared++);
    assert.equal(got.kind, "refused", String(answer.status));
    assert.equal(cleared, 0, "the draft is kept");
  }
});

test("errors that are not problems come back as messages", async () => {
  for (const [status, pattern] of [
    [403, /permiso/],
    [413, /2 MiB/],
    [404, /nube/],
  ] as const) {
    const { fetchFn } = server([{ status, body: { error: "x" } }]);
    const got = await checkCatalog(fetchFn, {});
    assert.equal(got.kind, "error");
    assert.match(got.kind === "error" ? got.message : "", pattern);
  }
});

test("Agent document reviewed: loading calls no endpoint", () => {
  const { calls } = server([]);
  const doc = { sellables: [{ ref: "a", name: "Coca", category: "Bebidas", total_price_with_taxes: 5000 }] };
  assert.ok(parseDocumentText(JSON.stringify(doc), []).ok);
  assert.equal(calls.length, 0);
});

test("Markup in a name: the grid never renders raw HTML", () => {
  const dir = join(import.meta.dirname, "../../components/catalog-grid");
  for (const name of readdirSync(dir)) {
    const source = readFileSync(join(dir, name), "utf8");
    assert.equal(/dangerouslySetInnerHTML|innerHTML|insertAdjacentHTML/.test(source), false, name);
  }
});

test("Recipe from grid rows", () => {
  let grid: GridState = {
    unmatched: [],
    rows: [row("bandeja", "COMPOSITE", { name: "Bandeja paisa", price: "30000" }), row("arroz", "INGREDIENT", { name: "Arroz" }), row("frijol", "INGREDIENT", { name: "Frijol" })],
  };
  const recipe = [
    { key: "l1", ingredientId: rowRef("arroz"), quantity: "200" },
    { key: "l2", ingredientId: rowRef("frijol"), quantity: "150" },
  ];
  assert.equal(panelProblem("bandeja", recipe, []), null);
  grid = gridReducer(grid, { type: "setPlate", id: "bandeja", recipe, slots: [] });
  assert.equal(plateSummary(grid.rows[0]), "2 ingredientes");
  assert.deepEqual(toDocument(grid, [], []).document.plates![0].recipe, [
    { ingredient: "arroz", quantity: 200 },
    { ingredient: "frijol", quantity: 150 },
  ]);
});

test("Existing ingredient picked", () => {
  const limon = { id: "e1", name: "Limón", sku: "ING-LIM", product_type: "INGREDIENT", unit_of_measure: "unit" };
  const grid: GridState = { unmatched: [], rows: [row("p", "COMPOSITE", { name: "Pescado", price: "30000" })] };
  const picked = ingredientCandidates(grid, "p", [limon]).find((c) => c.name.startsWith("Limón"))!;
  const next = gridReducer(grid, { type: "setPlate", id: "p", recipe: [{ key: "l1", ingredientId: picked.id, quantity: "1" }], slots: [] });
  const { document } = toDocument(next, [limon], []);
  assert.deepEqual(document.existing, [{ ref: picked.id, sku: "ING-LIM" }]);
  assert.deepEqual(document.plates![0].recipe, [{ ingredient: picked.id, quantity: 1 }]);
  assert.equal(document.ingredients, undefined, "Limón is not created again");
});

test("Plate offered as an ingredient", () => {
  const grid: GridState = {
    unmatched: [],
    rows: [row("p", "COMPOSITE", { name: "Bandeja" }), row("q", "COMPOSITE", { name: "Sancocho" }), row("i", "INGREDIENT", { name: "Yuca" })],
  };
  const existing = [{ id: "e", name: "Sopa", sku: "COC-SOP", product_type: "COMPOSITE", unit_of_measure: "unit" }];
  const names = ingredientCandidates(grid, "p", existing).map((c) => c.name);
  assert.deepEqual(names, ["Yuca"]);
});

test("Slot without a default", () => {
  const slots = [
    {
      key: "s1",
      name: "Proteína",
      choices: [
        { key: "c1", name: "Pollo", isDefault: false, items: [] },
        { key: "c2", name: "Res", isDefault: false, items: [] },
      ],
    },
  ];
  const problem = panelProblem("p", [], slots);
  assert.equal(problem?.kind, "slot");
  assert.equal(problem?.key, "s1");
});

test("a recipe line with quantity 0 keeps the panel open", () => {
  const problem = panelProblem("p", [{ key: "l1", ingredientId: rowRef("a"), quantity: "0" }], []);
  assert.deepEqual(problem && [problem.kind, problem.key], ["recipe", "l1"]);
});
