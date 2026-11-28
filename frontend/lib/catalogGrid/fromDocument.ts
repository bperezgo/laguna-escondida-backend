import type { UnitOfMeasure } from "../../types/product.ts";
import { draftKey, type SlotDraft } from "../products/optionSlots.ts";
import type { RecipeLineDraft } from "../products/recipeLines.ts";
import { MAX_ROWS, newRow, newRowId, rowRef, type ExistingProduct, type GridRow, type GridState, type UnmatchedExisting } from "./state.ts";
import { GRID_UNITS, type GridType } from "./values.ts";

export const MAX_FILE_BYTES = 2 * 1024 * 1024;

export type LoadResult =
  | { ok: true; state: GridState }
  | { ok: false; reason: "too_large" | "invalid_json" | "invalid_shape" | "too_many_rows"; message: string };

type Json = unknown;
type Obj = Record<string, Json>;

class ShapeError extends Error {}

const SECTIONS = ["defaults", "existing", "ingredients", "sellables", "plates", "opening_stock"];
const DEFAULT_FIELDS = ["vat", "ico", "taxes_format", "unit_of_measure", "prep_area", "prep_priority"];
const EXISTING_FIELDS = ["ref", "sku", "name"];
const PRODUCT_FIELDS = [
  "ref",
  "name",
  "category",
  "sku",
  "description",
  "unit_of_measure",
  "total_price_with_taxes",
  "vat",
  "ico",
  "taxes_format",
  "prep",
];
const INGREDIENT_FIELDS = [...PRODUCT_FIELDS, "product_type"];
const PLATE_FIELDS = [...PRODUCT_FIELDS, "recipe", "slots"];

// Only own keys of plain objects are read, every key must be in the allowed list, and values
// are copied one by one into new objects, so a "__proto__" key is refused, never merged.
function object(value: Json, allowed: readonly string[], where: string): Obj {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new ShapeError(`${where} debe ser un objeto`);
  }
  for (const key of Object.keys(value)) {
    if (allowed.indexOf(key) < 0) throw new ShapeError(`${where} tiene un campo desconocido "${key}"`);
  }
  return value as Obj;
}

function array(value: Json, where: string): Json[] {
  if (value === undefined) return [];
  if (!Array.isArray(value)) throw new ShapeError(`${where} debe ser una lista`);
  return value;
}

function str(o: Obj, key: string, where: string, required = false): string | undefined {
  const v = o[key];
  if (v === undefined && !required) return undefined;
  if (typeof v !== "string") throw new ShapeError(`${where}.${key} debe ser texto`);
  return v;
}

function num(o: Obj, key: string, where: string, required = false): number | undefined {
  const v = o[key];
  if (v === undefined && !required) return undefined;
  if (typeof v !== "number" || !Number.isFinite(v)) throw new ShapeError(`${where}.${key} debe ser un número`);
  return v;
}

function bool(o: Obj, key: string, where: string): boolean {
  const v = o[key];
  if (v === undefined) return false;
  if (typeof v !== "boolean") throw new ShapeError(`${where}.${key} debe ser verdadero o falso`);
  return v;
}

// The grid shows and sends percentages, so another format would be read with the wrong meaning.
function percentageFormat(o: Obj, where: string): void {
  const format = str(o, "taxes_format", where);
  if (format !== undefined && format !== "percentage") {
    throw new ShapeError(`${where}.taxes_format debe ser "percentage"`);
  }
}

function fold(s: string): string {
  return s.trim().toLowerCase();
}

interface Defaults {
  vat?: number;
  ico?: number;
  unit?: string;
  area?: string;
  priority?: number;
}

interface Pending {
  row: GridRow;
  recipe: { ingredient: string; quantity: number }[];
  slots: { name: string; choices: { name: string; isDefault: boolean; items: { ingredient: string; quantity: number }[] }[] }[];
}

function readDefaults(value: Json): Defaults {
  if (value === undefined) return {};
  const o = object(value, DEFAULT_FIELDS, "defaults");
  percentageFormat(o, "defaults");
  return {
    vat: num(o, "vat", "defaults"),
    ico: num(o, "ico", "defaults"),
    unit: str(o, "unit_of_measure", "defaults"),
    area: str(o, "prep_area", "defaults"),
    priority: num(o, "prep_priority", "defaults"),
  };
}

function readEntry(
  value: Json,
  type: GridType,
  allowed: readonly string[],
  where: string,
  d: Defaults,
  refs: Map<string, string>,
): Pending {
  const o = object(value, allowed, where);
  const ref = str(o, "ref", where, true)!;
  const row = newRow(type, newRowId());
  row.name = str(o, "name", where) ?? "";
  row.category = str(o, "category", where) ?? "";
  row.description = str(o, "description", where) ?? "";
  const sku = str(o, "sku", where);
  row.sku = sku ?? "";
  row.skuTouched = !!sku && sku.trim() !== "";
  const unit = str(o, "unit_of_measure", where) ?? d.unit;
  if (unit === undefined) row.unit = "";
  else if (GRID_UNITS.indexOf(unit as UnitOfMeasure) >= 0) row.unit = unit as UnitOfMeasure;
  else row.marks = { ...row.marks, unit: true };
  percentageFormat(o, where);
  // The import refuses these on an INGREDIENT; refusing them here keeps them from vanishing on load.
  if (type === "INGREDIENT") {
    for (const key of ["total_price_with_taxes", "vat", "ico", "taxes_format", "prep"]) {
      if (o[key] !== undefined) throw new ShapeError(`${where}.${key} no aplica a un INGREDIENT; usa product_type BOTH para venderlo`);
    }
  }

  const price = num(o, "total_price_with_taxes", where);
  const vat = num(o, "vat", where) ?? d.vat;
  const ico = num(o, "ico", where) ?? d.ico;
  let area: string | undefined;
  let priority: number | undefined;
  if (o.prep !== undefined) {
    const prep = object(o.prep, ["area", "priority"], `${where}.prep`);
    area = str(prep, "area", `${where}.prep`);
    priority = num(prep, "priority", `${where}.prep`);
  }
  if (type !== "INGREDIENT") {
    row.price = price === undefined ? "" : String(price);
    row.vat = vat === undefined ? "" : String(vat);
    row.ico = ico === undefined ? "" : String(ico);
    row.area = area ?? d.area ?? "";
    row.priority = priority ?? d.priority ?? 0;
  }

  const pending: Pending = { row, recipe: [], slots: [] };
  if (type === "COMPOSITE") {
    pending.recipe = array(o.recipe, `${where}.recipe`).map((v, i) => {
      const w = `${where}.recipe[${i}]`;
      const line = object(v, ["ingredient", "quantity"], w);
      return { ingredient: str(line, "ingredient", w, true)!, quantity: num(line, "quantity", w, true)! };
    });
    pending.slots = array(o.slots, `${where}.slots`).map((v, i) => {
      const w = `${where}.slots[${i}]`;
      const slot = object(v, ["name", "choices"], w);
      return {
        name: str(slot, "name", w, true)!,
        choices: array(slot.choices, `${w}.choices`).map((cv, c) => {
          const cw = `${w}.choices[${c}]`;
          const choice = object(cv, ["name", "default", "items"], cw);
          return {
            name: str(choice, "name", cw, true)!,
            isDefault: bool(choice, "default", cw),
            items: array(choice.items, `${cw}.items`).map((iv, n) => {
              const iw = `${cw}.items[${n}]`;
              const item = object(iv, ["ingredient", "quantity"], iw);
              return { ingredient: str(item, "ingredient", iw, true)!, quantity: num(item, "quantity", iw, true)! };
            }),
          };
        }),
      };
    });
  }
  if (!refs.has(ref)) refs.set(ref, rowRef(row.id));
  return pending;
}

export function documentToState(value: Json, existing: readonly ExistingProduct[]): GridState {
  const doc = object(value, SECTIONS, "El documento");
  const refs = new Map<string, string>();
  const d = readDefaults(doc.defaults);
  const unmatched: UnmatchedExisting[] = [];

  array(doc.existing, "existing").forEach((v, i) => {
    const w = `existing[${i}]`;
    const o = object(v, EXISTING_FIELDS, w);
    const ref = str(o, "ref", w, true)!;
    const sku = str(o, "sku", w);
    const name = str(o, "name", w);
    const matches = existing.filter((p) =>
      sku ? fold(p.sku) === fold(sku) : name !== undefined && fold(p.name) === fold(name),
    );
    if (matches.length === 1) {
      refs.set(ref, `existing:${matches[0].id}`);
      return;
    }
    const key = draftKey();
    unmatched.push(sku ? { key, sku } : { key, name: name ?? ref });
    refs.set(ref, `unmatched:${key}`);
  });

  const pending: Pending[] = [];
  array(doc.ingredients, "ingredients").forEach((v, i) => {
    const productType = (v as Obj | null)?.product_type;
    if (productType !== undefined && productType !== "INGREDIENT" && productType !== "BOTH") {
      throw new ShapeError(`ingredients[${i}].product_type debe ser INGREDIENT o BOTH`);
    }
    pending.push(readEntry(v, productType === "BOTH" ? "BOTH" : "INGREDIENT", INGREDIENT_FIELDS, `ingredients[${i}]`, d, refs));
  });
  array(doc.sellables, "sellables").forEach((v, i) => {
    pending.push(readEntry(v, "SELLABLE", PRODUCT_FIELDS, `sellables[${i}]`, d, refs));
  });
  array(doc.plates, "plates").forEach((v, i) => {
    pending.push(readEntry(v, "COMPOSITE", PLATE_FIELDS, `plates[${i}]`, d, refs));
  });

  const counts = array(doc.opening_stock, "opening_stock").map((v, i) => {
    const w = `opening_stock[${i}]`;
    const o = object(v, ["product", "counted"], w);
    return { product: str(o, "product", w, true)!, counted: num(o, "counted", w, true)! };
  });

  if (pending.length > MAX_ROWS) throw new RangeError(`El documento tiene ${pending.length} productos; el máximo es ${MAX_ROWS}`);

  // A ref nothing defines is kept visible as a missing existing product, not dropped.
  const resolve = (ref: string): string => {
    const known = refs.get(ref);
    if (known) return known;
    const key = draftKey();
    unmatched.push({ key, name: ref });
    refs.set(ref, `unmatched:${key}`);
    return `unmatched:${key}`;
  };

  const rows = pending.map(({ row, recipe, slots }) => {
    const lines: RecipeLineDraft[] = recipe.map((l) => ({
      key: draftKey(),
      ingredientId: resolve(l.ingredient),
      quantity: String(l.quantity),
    }));
    const drafts: SlotDraft[] = slots.map((s) => ({
      key: draftKey(),
      name: s.name,
      choices: s.choices.map((c) => ({
        key: draftKey(),
        name: c.name,
        isDefault: c.isDefault,
        items: c.items.map((it) => ({
          key: draftKey(),
          ingredientProductId: resolve(it.ingredient),
          quantity: String(it.quantity),
        })),
      })),
    }));
    return { ...row, recipe: lines, slots: drafts };
  });

  const byRef = new Map<string, GridRow>();
  for (const r of rows) byRef.set(rowRef(r.id), r);
  for (const count of counts) {
    const target = byRef.get(refs.get(count.product) ?? "");
    if (!target) {
      throw new ShapeError(`opening_stock "${count.product}" no es un producto que el documento crea`);
    }
    if (target.type === "COMPOSITE") {
      throw new ShapeError(`opening_stock "${count.product}" es un plato, que no lleva stock propio`);
    }
    target.stock = String(count.counted);
  }
  return { rows, unmatched };
}

export function parseDocumentText(text: string, existing: readonly ExistingProduct[]): LoadResult {
  let parsed: Json;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { ok: false, reason: "invalid_json", message: "El archivo no es un JSON válido." };
  }
  try {
    return { ok: true, state: documentToState(parsed, existing) };
  } catch (err) {
    if (err instanceof ShapeError) {
      return { ok: false, reason: "invalid_shape", message: `El archivo no es un documento de catálogo válido: ${err.message}.` };
    }
    if (err instanceof RangeError) return { ok: false, reason: "too_many_rows", message: `${err.message}.` };
    throw err;
  }
}

export async function readDocumentFile(
  file: { size: number; text(): Promise<string> },
  existing: readonly ExistingProduct[],
): Promise<LoadResult> {
  if (file.size > MAX_FILE_BYTES) {
    return { ok: false, reason: "too_large", message: "El archivo es demasiado grande (máximo 2 MiB)." };
  }
  return parseDocumentText(await file.text(), existing);
}
