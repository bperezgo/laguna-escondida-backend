import type {
  CatalogImportDocument,
  CatalogImportExisting,
  CatalogImportIngredient,
  CatalogImportPlate,
  CatalogImportProblem,
  CatalogImportProductFields,
  CatalogImportStockCount,
} from "../../types/catalogImport.ts";
import type { UnitOfMeasure } from "../../types/product.ts";
import { cellApplies, gridSkus, type Column, type ExistingProduct, type GridRow, type GridState } from "./state.ts";
import { isSold, parseAmount, parseCount, parsePercent } from "./values.ts";

export type PanelTarget = { kind: "recipe" | "slot"; key: string };

export interface ProblemTarget {
  rowId: string;
  cell?: Column;
  panel?: PanelTarget;
}

export interface BuiltDocument {
  document: CatalogImportDocument;
  pointers: Record<string, ProblemTarget>;
}

const ROW_REF = "row:";

// The grid's ingredient ids become document refs: a row's ref is its id, an existing
// product's ref is the same "existing:<id>" string the grid uses.
function refFor(ingredientId: string): string {
  return ingredientId.startsWith(ROW_REF) ? ingredientId.slice(ROW_REF.length) : ingredientId;
}

function productFields(row: GridRow, sku: string): CatalogImportProductFields {
  const fields: CatalogImportProductFields = {
    ref: row.id,
    name: row.name.trim(),
    category: row.category.trim(),
  };
  if (sku.trim()) fields.sku = sku.trim();
  if (row.description.trim()) fields.description = row.description.trim();
  if (row.unit) fields.unit_of_measure = row.unit as UnitOfMeasure;
  if (isSold(row.type)) {
    const price = parseAmount(row.price);
    const vat = parsePercent(row.vat);
    const ico = parsePercent(row.ico);
    if (price !== null) fields.total_price_with_taxes = price;
    if (vat !== null) fields.vat = vat;
    if (ico !== null) fields.ico = ico;
    fields.taxes_format = "percentage";
    if (row.area.trim()) fields.prep = { area: row.area.trim(), priority: row.priority };
  }
  return fields;
}

// An unreadable quantity goes as 0 so the server reports it on its line; NaN would become
// null in JSON and fail the whole request.
function quantityOf(raw: string): number {
  return parseAmount(raw) ?? 0;
}

export function toDocument(
  state: GridState,
  existing: readonly ExistingProduct[],
  organizationSkus: readonly string[],
): BuiltDocument {
  const skus = gridSkus(state.rows, organizationSkus);
  const pointers: Record<string, ProblemTarget> = {};
  const ingredients: CatalogImportIngredient[] = [];
  const sellables: CatalogImportProductFields[] = [];
  const plates: CatalogImportPlate[] = [];
  const openingStock: CatalogImportStockCount[] = [];
  const linkedExisting = new Set<string>();

  const link = (ingredientId: string): string => {
    if (ingredientId.startsWith("existing:")) linkedExisting.add(ingredientId);
    return refFor(ingredientId);
  };

  for (const row of state.rows) {
    const fields = productFields(row, skus[row.id]?.value ?? "");
    if (row.type === "INGREDIENT" || row.type === "BOTH") {
      pointers[`/ingredients/${ingredients.length}`] = { rowId: row.id };
      ingredients.push({ ...fields, product_type: row.type });
    } else if (row.type === "SELLABLE") {
      pointers[`/sellables/${sellables.length}`] = { rowId: row.id };
      sellables.push(fields);
    } else {
      const p = plates.length;
      pointers[`/plates/${p}`] = { rowId: row.id };
      const plate: CatalogImportPlate = { ...fields };
      if (row.recipe.length > 0) {
        plate.recipe = row.recipe.map((line, i) => {
          pointers[`/plates/${p}/recipe/${i}`] = { rowId: row.id, panel: { kind: "recipe", key: line.key } };
          return { ingredient: link(line.ingredientId), quantity: quantityOf(line.quantity) };
        });
      }
      if (row.slots.length > 0) {
        plate.slots = row.slots.map((slot, s) => {
          const target: ProblemTarget = { rowId: row.id, panel: { kind: "slot", key: slot.key } };
          pointers[`/plates/${p}/slots/${s}`] = target;
          return {
            name: slot.name.trim(),
            choices: slot.choices.map((choice, c) => {
              pointers[`/plates/${p}/slots/${s}/choices/${c}`] = target;
              return {
                name: choice.name.trim(),
                default: choice.isDefault,
                items: choice.items.map((item, i) => {
                  pointers[`/plates/${p}/slots/${s}/choices/${c}/items/${i}`] = target;
                  return { ingredient: link(item.ingredientProductId), quantity: quantityOf(item.quantity) };
                }),
              };
            }),
          };
        });
      }
      plates.push(plate);
    }

    if (cellApplies(row.type, "stock")) {
      const counted = parseCount(row.stock);
      if (counted !== null) {
        pointers[`/opening_stock/${openingStock.length}`] = { rowId: row.id, cell: "stock" };
        openingStock.push({ product: row.id, counted });
      }
    }
  }

  const existingEntries: CatalogImportExisting[] = [];
  const byId = new Map(existing.map((p) => [p.id, p]));
  linkedExisting.forEach((ref) => {
    const product = byId.get(ref.slice("existing:".length));
    if (!product) return;
    existingEntries.push(product.sku ? { ref, sku: product.sku } : { ref, name: product.name });
  });
  for (const u of state.unmatched) {
    const ref = `unmatched:${u.key}`;
    existingEntries.push(u.sku ? { ref, sku: u.sku } : { ref, name: u.name ?? "" });
  }

  const document: CatalogImportDocument = {};
  if (existingEntries.length > 0) document.existing = existingEntries;
  if (ingredients.length > 0) document.ingredients = ingredients;
  if (sellables.length > 0) document.sellables = sellables;
  if (plates.length > 0) document.plates = plates;
  if (openingStock.length > 0) document.opening_stock = openingStock;
  return { document, pointers };
}

const FIELD_CELLS: Record<string, Column> = {
  name: "name",
  category: "category",
  sku: "sku",
  unit_of_measure: "unit",
  total_price_with_taxes: "price",
  vat: "vat",
  ico: "ico",
  taxes_format: "ico",
  "prep.area": "area",
  prep: "area",
};

function cellFor(problem: CatalogImportProblem): Column {
  switch (problem.code) {
    case "name_conflict":
      return "name";
    case "sku_conflict":
      return "sku";
    case "invalid_unit":
      return "unit";
    case "missing_field":
    case "invalid_field": {
      const field = /^[a-z_]+(\.[a-z_]+)?/.exec(problem.message)?.[0] ?? "";
      return FIELD_CELLS[field] ?? "name";
    }
    default:
      return "name";
  }
}

// A path the map does not know (the server may point deeper than the grid wrote) falls back
// to its longest known prefix. Null means the problem belongs above the grid.
export function locateProblem(
  pointers: Record<string, ProblemTarget>,
  problem: CatalogImportProblem,
): ProblemTarget | null {
  const parts = problem.path.split("/");
  for (let n = parts.length; n > 1; n--) {
    const target = pointers[parts.slice(0, n).join("/")];
    if (!target) continue;
    if (target.panel) return { rowId: target.rowId, cell: "detail", panel: target.panel };
    if (target.cell) return target;
    return { rowId: target.rowId, cell: cellFor(problem) };
  }
  return null;
}
