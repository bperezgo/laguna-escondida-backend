import { test } from "node:test";
import assert from "node:assert/strict";
import { gridAccess, showGridLink } from "./access.ts";
import { newRow, type GridRow } from "./state.ts";
import { filterRows, moveCell, pageCount, pageRows } from "./view.ts";

test("Admin on the cloud", () => {
  const admin = { isLoading: false, isEdge: false, canImport: true };
  assert.equal(gridAccess(admin), "ready");
  assert.equal(showGridLink(admin), true);
});

test("Role without the permission", () => {
  const manager = { isLoading: false, isEdge: false, canImport: false };
  assert.equal(gridAccess(manager), "forbidden");
  assert.equal(showGridLink(manager), false);
});

test("Edge box", () => {
  // On the box catalog:import is a cloud-only permission, so an admin there holds neither.
  for (const canImport of [true, false]) {
    const edge = { isLoading: false, isEdge: true, canImport };
    assert.equal(gridAccess(edge), "edge");
    assert.equal(showGridLink(edge), false);
  }
});

test("nothing is shown while permissions load", () => {
  assert.equal(gridAccess({ isLoading: true, isEdge: false, canImport: false }), "loading");
});

function rows(): GridRow[] {
  return [
    { ...newRow("SELLABLE", "a"), name: "Limonada de coco" },
    { ...newRow("INGREDIENT", "b"), name: "Arroz" },
    { ...newRow("COMPOSITE", "c"), name: "Arroz con pollo" },
  ];
}

test("filter by type and search by name, accents ignored", () => {
  assert.deepEqual(filterRows(rows(), "INGREDIENT", "").map((r) => r.id), ["b"]);
  assert.deepEqual(filterRows(rows(), "", "arróz").map((r) => r.id), ["b", "c"]);
  assert.deepEqual(filterRows(rows(), "COMPOSITE", "arroz").map((r) => r.id), ["c"]);
});

test("rows are paged by 100", () => {
  const many = Array.from({ length: 250 }, (_, i) => i);
  assert.equal(pageCount(250), 3);
  assert.equal(pageCount(0), 1);
  assert.deepEqual(pageRows(many, 0).length, 100);
  assert.deepEqual(pageRows(many, 2), many.slice(200));
  assert.deepEqual(pageRows(many, 9), many.slice(200), "past the end shows the last page");
});

test("arrow and Enter moves skip cells that do not apply", () => {
  const r = rows();
  assert.deepEqual(moveCell(r, { rowId: "a", column: "price" }, "ArrowDown"), { rowId: "c", column: "price" });
  assert.deepEqual(moveCell(r, { rowId: "a", column: "name" }, "Enter"), { rowId: "b", column: "name" });
  assert.equal(moveCell(r, { rowId: "c", column: "name" }, "ArrowDown"), null);
  assert.deepEqual(moveCell(r, { rowId: "b", column: "name" }, "ArrowRight"), { rowId: "b", column: "type" });
  assert.equal(moveCell(r, { rowId: "a", column: "name" }, "ArrowLeft"), null);
});

test("Tab wraps to the next row's first cell", () => {
  const r = rows();
  assert.deepEqual(moveCell(r, { rowId: "a", column: "stock" }, "Tab"), { rowId: "b", column: "name" });
  assert.deepEqual(moveCell(r, { rowId: "b", column: "name" }, "ShiftTab"), { rowId: "a", column: "stock" });
});
