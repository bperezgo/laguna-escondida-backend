import type { UnitOfMeasure } from "../../types/product.ts";

export type GridType = "INGREDIENT" | "SELLABLE" | "COMPOSITE" | "BOTH";

export const GRID_TYPES: { value: GridType; label: string }[] = [
  { value: "INGREDIENT", label: "Ingrediente" },
  { value: "SELLABLE", label: "Producto" },
  { value: "COMPOSITE", label: "Plato" },
  { value: "BOTH", label: "Ambos" },
];

export const GRID_UNITS: UnitOfMeasure[] = ["unit", "kg", "g", "l", "ml"];

export const GRID_AREAS: { value: string; label: string }[] = [
  { value: "kitchen", label: "Cocina" },
  { value: "bar", label: "Bar" },
  { value: "grill", label: "Parrilla" },
  { value: "sushi", label: "Sushi" },
  { value: "desserts", label: "Postres" },
];

export function isSold(type: GridType): boolean {
  return type !== "INGREDIENT";
}

export function isIngredientType(type: GridType): boolean {
  return type === "INGREDIENT" || type === "BOTH";
}

function fold(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").trim().toLowerCase();
}

const TYPE_WORDS: Record<string, GridType> = {
  ingrediente: "INGREDIENT",
  ingredient: "INGREDIENT",
  insumo: "INGREDIENT",
  producto: "SELLABLE",
  vendible: "SELLABLE",
  sellable: "SELLABLE",
  plato: "COMPOSITE",
  compuesto: "COMPOSITE",
  composite: "COMPOSITE",
  ambos: "BOTH",
  both: "BOTH",
};

export function parseType(raw: string): GridType | null {
  return TYPE_WORDS[fold(raw)] ?? null;
}

const UNIT_WORDS: Record<string, UnitOfMeasure> = {
  unit: "unit",
  und: "unit",
  un: "unit",
  u: "unit",
  unidad: "unit",
  unidades: "unit",
  kg: "kg",
  kilo: "kg",
  kilos: "kg",
  kilogramo: "kg",
  kilogramos: "kg",
  g: "g",
  gr: "g",
  gramo: "g",
  gramos: "g",
  l: "l",
  lt: "l",
  litro: "l",
  litros: "l",
  ml: "ml",
  mililitro: "ml",
  mililitros: "ml",
};

export function parseUnit(raw: string): UnitOfMeasure | null {
  return UNIT_WORDS[fold(raw)] ?? null;
}

// A known area label maps to its value; any other text is a custom area, as the server allows.
export function parseArea(raw: string): string {
  const folded = fold(raw);
  const known = GRID_AREAS.find((a) => fold(a.label) === folded || a.value === folded);
  return known ? known.value : raw.trim();
}

export function areaLabel(value: string): string {
  return GRID_AREAS.find((a) => a.value === value)?.label ?? value;
}

// Colombian amounts: "." groups thousands and "," marks decimals, so 32.000 and $32.000 are
// both 32000. A lone "," followed by three digits is read as thousands too (1,500 from a
// spreadsheet in another locale), because prices in pesos rarely carry decimals.
export function parseAmount(raw: string): number | null {
  const s = raw.replace(/[$\s]/g, "").replace(/ /g, "");
  if (s === "") return null;
  let normalized: string;
  if (/^\d{1,3}(\.\d{3})+(,\d+)?$/.test(s)) normalized = s.replace(/\./g, "").replace(",", ".");
  else if (/^\d{1,3}(,\d{3})+(\.\d+)?$/.test(s)) normalized = s.replace(/,/g, "");
  else if (/^\d+(,\d+)?$/.test(s)) normalized = s.replace(",", ".");
  else if (/^\d+(\.\d+)?$/.test(s)) normalized = s;
  else return null;
  const n = Number(normalized);
  return Number.isFinite(n) ? n : null;
}

export function parsePercent(raw: string): number | null {
  const s = raw.replace(/[%\s]/g, "").replace(",", ".");
  if (!/^\d+(\.\d+)?$/.test(s)) return null;
  const n = Number(s);
  return n <= 100 ? n : null;
}

export function parseCount(raw: string): number | null {
  const n = parseAmount(raw);
  return n !== null && Number.isInteger(n) ? n : null;
}
