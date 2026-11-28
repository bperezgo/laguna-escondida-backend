import type { UnitOfMeasure } from "../../types/product.ts";
import type { SlotDraft } from "../products/optionSlots.ts";
import type { RecipeLineDraft } from "../products/recipeLines.ts";
import { generateSku } from "../products/sku.ts";
import { isIngredientType, isSold, type GridType } from "./values.ts";

export const MAX_ROWS = 1000;
export const DEFAULT_ICO = "8";
export const DEFAULT_VAT = "0";

export type Column =
  | "name"
  | "type"
  | "category"
  | "area"
  | "unit"
  | "price"
  | "ico"
  | "vat"
  | "sku"
  | "stock"
  | "detail";

export type TextColumn = Exclude<Column, "type" | "detail">;

// Recipe lines and slot items name an ingredient as "row:<row id>" or "existing:<product id>",
// or "unmatched:<key>" for an `existing` entry of a loaded file that matched no product.
export interface GridRow {
  id: string;
  name: string;
  type: GridType;
  category: string;
  area: string;
  unit: UnitOfMeasure | "";
  price: string;
  ico: string;
  vat: string;
  sku: string;
  skuTouched: boolean;
  stock: string;
  description: string;
  priority: number;
  recipe: RecipeLineDraft[];
  slots: SlotDraft[];
  marks: Partial<Record<Column, true>>;
}

export interface UnmatchedExisting {
  key: string;
  sku?: string;
  name?: string;
}

export interface GridState {
  rows: GridRow[];
  unmatched: UnmatchedExisting[];
}

export interface ExistingProduct {
  id: string;
  name: string;
  sku: string;
  product_type: string;
  unit_of_measure: string;
}

export type PasteCell = { value: string } | { invalid: true } | { skip: true };

export type GridAction =
  | { type: "add"; productType?: GridType }
  | { type: "duplicate"; id: string }
  | { type: "delete"; ids: string[] }
  | { type: "edit"; id: string; column: TextColumn; value: string }
  | { type: "changeType"; id: string; productType: GridType }
  | { type: "setPlate"; id: string; recipe: RecipeLineDraft[]; slots: SlotDraft[] }
  | { type: "fillTax"; column: "ico" | "vat"; value: string; ids?: string[] }
  | { type: "clearTax"; column: "ico" | "vat" }
  | { type: "paste"; startId: string | null; startColumn: Column; order: string[]; cells: PasteCell[][] }
  | { type: "replace"; state: GridState };

// The grid's column order. Price sits next to the name because a menu is usually pasted as
// "name<TAB>price".
export const PASTE_COLUMNS: Column[] = [
  "name",
  "price",
  "type",
  "category",
  "area",
  "unit",
  "ico",
  "vat",
  "sku",
  "stock",
];

export const EMPTY_STATE: GridState = { rows: [], unmatched: [] };

export function newRowId(): string {
  const bytes = new Uint8Array(6);
  globalThis.crypto.getRandomValues(bytes);
  return "r" + Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function rowRef(id: string): string {
  return `row:${id}`;
}

export function cellApplies(type: GridType, column: Column): boolean {
  switch (column) {
    case "price":
    case "ico":
    case "vat":
    case "area":
      return isSold(type);
    case "stock":
      return type !== "COMPOSITE";
    case "detail":
      return type === "COMPOSITE";
    default:
      return true;
  }
}

export function newRow(productType: GridType = "SELLABLE", id: string = newRowId()): GridRow {
  const sold = isSold(productType);
  return {
    id,
    name: "",
    type: productType,
    category: "",
    area: sold ? "kitchen" : "",
    unit: "unit",
    price: "",
    ico: sold ? DEFAULT_ICO : "",
    vat: sold ? DEFAULT_VAT : "",
    sku: "",
    skuTouched: false,
    stock: "",
    description: "",
    priority: 0,
    recipe: [],
    slots: [],
    marks: {},
  };
}

function usesRef(row: GridRow, ref: string): boolean {
  if (row.recipe.some((l) => l.ingredientId === ref)) return true;
  return row.slots.some((s) => s.choices.some((c) => c.items.some((it) => it.ingredientProductId === ref)));
}

export function platesUsing(state: GridState, ids: readonly string[]): string[] {
  const refs = ids.map(rowRef);
  return state.rows
    .filter((r) => r.type === "COMPOSITE" && !ids.includes(r.id) && refs.some((ref) => usesRef(r, ref)))
    .map((r) => r.name.trim() || "(sin nombre)");
}

export interface PasteImpact {
  // Plates that would lose an ingredient because a pasted type made it something else.
  plates: string[];
  // Plates that would stop being plates, losing their recipe and options.
  platesLosingDetail: string[];
}

// What a paste would remove through the type cells it changes, so the UI asks before
// dispatching, as it does for a type changed by hand. Null means nothing is removed.
export function pasteImpact(state: GridState, action: Extract<GridAction, { type: "paste" }>): PasteImpact | null {
  const after = gridReducer(state, action);
  const left: string[] = [];
  const platesLosingDetail: string[] = [];
  for (const before of state.rows) {
    const next = after.rows.find((r) => r.id === before.id);
    if (!next || next.type === before.type) continue;
    if (isIngredientType(before.type) && !isIngredientType(next.type)) left.push(before.id);
    if (before.type === "COMPOSITE" && (before.recipe.length > 0 || before.slots.length > 0)) {
      platesLosingDetail.push(before.name.trim() || "(sin nombre)");
    }
  }
  const plates = left.length > 0 ? platesUsing(state, left) : [];
  return plates.length > 0 || platesLosingDetail.length > 0 ? { plates, platesLosingDetail } : null;
}

export interface TypeChangeImpact {
  plates: string[];
  dropsPlateDetail: boolean;
}

// What a type change would remove, so the UI can ask before dispatching. Null means nothing.
export function typeChangeImpact(state: GridState, id: string, next: GridType): TypeChangeImpact | null {
  const row = state.rows.find((r) => r.id === id);
  if (!row || row.type === next) return null;
  const plates = isIngredientType(row.type) && !isIngredientType(next) ? platesUsing(state, [id]) : [];
  const dropsPlateDetail = row.type === "COMPOSITE" && (row.recipe.length > 0 || row.slots.length > 0);
  return plates.length > 0 || dropsPlateDetail ? { plates, dropsPlateDetail } : null;
}

function withoutRefs(row: GridRow, refs: readonly string[]): GridRow {
  if (row.type !== "COMPOSITE" || !refs.some((ref) => usesRef(row, ref))) return row;
  const keep = (ref: string) => !refs.includes(ref);
  return {
    ...row,
    recipe: row.recipe.filter((l) => keep(l.ingredientId)),
    slots: row.slots.map((s) => ({
      ...s,
      choices: s.choices.map((c) => ({ ...c, items: c.items.filter((it) => keep(it.ingredientProductId)) })),
    })),
  };
}

function updateRow(state: GridState, id: string, fn: (r: GridRow) => GridRow): GridState {
  return { ...state, rows: state.rows.map((r) => (r.id === id ? fn(r) : r)) };
}

function unmark(row: GridRow, column: Column): Partial<Record<Column, true>> {
  if (!row.marks[column]) return row.marks;
  const marks = { ...row.marks };
  delete marks[column];
  return marks;
}

function setCell(row: GridRow, column: TextColumn, value: string): GridRow {
  const marks = unmark(row, column);
  if (column === "sku") return { ...row, sku: value, skuTouched: value.trim() !== "", marks };
  return { ...row, [column]: value, marks };
}

function changeType(state: GridState, id: string, next: GridType): GridState {
  const row = state.rows.find((r) => r.id === id);
  if (!row || row.type === next) return state;
  const dropRefs = isIngredientType(row.type) && !isIngredientType(next);
  const rows = state.rows.map((r) => {
    if (r.id !== id) return dropRefs ? withoutRefs(r, [rowRef(id)]) : r;
    const becameSold = !isSold(r.type) && isSold(next);
    return {
      ...r,
      type: next,
      recipe: next === "COMPOSITE" ? r.recipe : [],
      slots: next === "COMPOSITE" ? r.slots : [],
      area: becameSold && !r.area ? "kitchen" : r.area,
      ico: becameSold && r.ico === "" && r.vat === "" ? DEFAULT_ICO : r.ico,
      vat: becameSold && r.ico === "" && r.vat === "" ? DEFAULT_VAT : r.vat,
    };
  });
  return { ...state, rows };
}

function applyPaste(state: GridState, action: Extract<GridAction, { type: "paste" }>): GridState {
  const firstColumn = PASTE_COLUMNS.indexOf(action.startColumn);
  if (firstColumn < 0 || action.cells.length === 0) return state;
  const rows = [...state.rows];
  const indexById = new Map(rows.map((r, i) => [r.id, i]));
  const start = action.startId === null ? action.order.length : Math.max(0, action.order.indexOf(action.startId));
  const leftIngredients: string[] = [];

  for (let line = 0; line < action.cells.length; line++) {
    const targetId = action.order[start + line];
    let index = targetId !== undefined ? indexById.get(targetId) : undefined;
    if (index === undefined) {
      if (rows.length >= MAX_ROWS) break;
      rows.push(newRow());
      index = rows.length - 1;
    }
    let row = rows[index];
    action.cells[line].forEach((cell, offset) => {
      const column = PASTE_COLUMNS[firstColumn + offset];
      if (!column || "skip" in cell) return;
      if ("invalid" in cell) {
        row = { ...row, ...(column === "type" ? {} : { [column]: "" }), marks: { ...row.marks, [column]: true } };
        if (column === "sku") row = { ...row, skuTouched: false };
        return;
      }
      if (column === "type") {
        const next = changeType({ rows: [row], unmatched: [] }, row.id, cell.value as GridType).rows[0];
        if (isIngredientType(row.type) && !isIngredientType(next.type)) leftIngredients.push(rowRef(row.id));
        row = { ...next, marks: unmark(next, "type") };
        return;
      }
      if (!cellApplies(row.type, column)) return;
      row = setCell(row, column as TextColumn, cell.value);
    });
    rows[index] = row;
  }
  return { ...state, rows: leftIngredients.length > 0 ? rows.map((r) => withoutRefs(r, leftIngredients)) : rows };
}

export function gridReducer(state: GridState, action: GridAction): GridState {
  switch (action.type) {
    case "add":
      if (state.rows.length >= MAX_ROWS) return state;
      return { ...state, rows: [...state.rows, newRow(action.productType)] };
    case "duplicate": {
      const index = state.rows.findIndex((r) => r.id === action.id);
      if (index < 0 || state.rows.length >= MAX_ROWS) return state;
      const copy: GridRow = { ...state.rows[index], id: newRowId(), sku: "", skuTouched: false, marks: {} };
      const rows = [...state.rows];
      rows.splice(index + 1, 0, copy);
      return { ...state, rows };
    }
    case "delete": {
      const refs = action.ids.map(rowRef);
      const rows = state.rows.filter((r) => !action.ids.includes(r.id)).map((r) => withoutRefs(r, refs));
      return { ...state, rows };
    }
    case "edit":
      return updateRow(state, action.id, (r) => setCell(r, action.column, action.value));
    case "changeType":
      return changeType(state, action.id, action.productType);
    case "setPlate":
      return updateRow(state, action.id, (r) =>
        r.type === "COMPOSITE" ? { ...r, recipe: action.recipe, slots: action.slots, marks: unmark(r, "detail") } : r,
      );
    case "fillTax": {
      const ids = action.ids;
      return {
        ...state,
        rows: state.rows.map((r) =>
          isSold(r.type) && (!ids || ids.includes(r.id)) ? setCell(r, action.column, action.value) : r,
        ),
      };
    }
    case "clearTax":
      return {
        ...state,
        rows: state.rows.map((r) => (isSold(r.type) ? setCell(r, action.column, "0") : r)),
      };
    case "paste":
      return applyPaste(state, action);
    case "replace":
      return action.state;
  }
}

export interface SkuView {
  value: string;
  suggested: boolean;
}

// Typed SKUs are reserved first, then each untouched row gets the next free suggestion, the
// same order the import uses, so the grid shows what the server would generate.
export function gridSkus(rows: readonly GridRow[], organizationSkus: readonly string[]): Record<string, SkuView> {
  const taken = new Set<string>(organizationSkus.map((s) => s.toUpperCase()));
  for (const r of rows) if (r.skuTouched) taken.add(r.sku.trim().toUpperCase());
  const out: Record<string, SkuView> = {};
  for (const r of rows) {
    if (r.skuTouched) {
      out[r.id] = { value: r.sku, suggested: false };
      continue;
    }
    if (!r.name.trim()) {
      out[r.id] = { value: "", suggested: true };
      continue;
    }
    const sku = generateSku(r.name, r.type, isSold(r.type) ? r.area : "", taken);
    taken.add(sku.toUpperCase());
    out[r.id] = { value: sku, suggested: true };
  }
  return out;
}

export interface Candidate {
  id: string;
  name: string;
  unit: string;
}

// Only INGREDIENT and BOTH products can be in a recipe or a choice, from the grid or the
// organization. A grid row placed after the plate is offered like any other.
export function ingredientCandidates(
  state: GridState,
  plateId: string,
  existing: readonly ExistingProduct[],
): Candidate[] {
  const fromRows = state.rows
    .filter((r) => r.id !== plateId && isIngredientType(r.type))
    .map((r) => ({ id: rowRef(r.id), name: r.name.trim() || "(sin nombre)", unit: r.unit }));
  const fromOrganization = existing
    .filter((p) => p.product_type === "INGREDIENT" || p.product_type === "BOTH")
    .map((p) => ({ id: `existing:${p.id}`, name: `${p.name} (existente)`, unit: p.unit_of_measure }));
  const unmatched = state.unmatched.map((u) => ({
    id: `unmatched:${u.key}`,
    name: `${u.name ?? u.sku ?? u.key} (no encontrado)`,
    unit: "",
  }));
  return [...fromRows, ...fromOrganization, ...unmatched];
}

export function plateSummary(row: GridRow): string {
  const parts: string[] = [];
  const lines = row.recipe.length;
  const slots = row.slots.length;
  if (lines > 0) parts.push(`${lines} ${lines === 1 ? "ingrediente" : "ingredientes"}`);
  if (slots > 0) parts.push(`${slots} ${slots === 1 ? "opción" : "opciones"}`);
  return parts.length > 0 ? parts.join(", ") : "Sin receta";
}
