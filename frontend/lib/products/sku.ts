// Same rule as backend/internal/domain/aggregate/product/sku_generate.go; both are checked
// against the example table in openspec specs/product-sku.

const MAX_NAME_PARTS = 3;

const AREA_PREFIXES: Record<string, string> = {
  kitchen: "COC",
  bar: "BAR",
  grill: "PAR",
  sushi: "SUS",
  desserts: "POS",
};

const FILLER_WORDS = new Set(["A", "DE", "DEL", "CON", "LA", "EL", "LOS", "LAS", "Y", "EN", "AL"]);

export function takenSkus(skus: readonly string[]): Set<string> {
  return new Set(skus.map((sku) => sku.toUpperCase()));
}

export function generateSku(
  name: string,
  productType: string,
  area: string,
  taken: ReadonlySet<string>,
): string {
  const base = [prefix(productType, area), ...nameParts(name)].join("-");
  const isTaken = (sku: string) => taken.has(sku.toUpperCase());
  if (!isTaken(base)) return base;
  for (let n = 2; ; n++) {
    const candidate = `${base}-${n}`;
    if (!isTaken(candidate)) return candidate;
  }
}

function prefix(productType: string, area: string): string {
  const trimmed = area.trim();
  const known = AREA_PREFIXES[trimmed.toLowerCase()];
  if (known) return known;
  const folded = words(trimmed).join("");
  if (folded) return folded.slice(0, 3);
  return productType === "INGREDIENT" ? "ING" : "PRD";
}

function nameParts(name: string): string[] {
  const parts: string[] = [];
  for (const word of words(name)) {
    if (FILLER_WORDS.has(word)) continue;
    parts.push(wordPart(word));
    if (parts.length === MAX_NAME_PARTS) break;
  }
  return parts;
}

function wordPart(word: string): string {
  const digits = /^[0-9]+/.exec(word);
  return digits ? digits[0] : word.slice(0, 3);
}

function words(s: string): string[] {
  const stripped = s.normalize("NFD").replace(/[\u0300-\u036f]/g, "");
  return (stripped.match(/[A-Za-z0-9]+/g) ?? []).map((w) => w.toUpperCase());
}

export interface SkuField {
  editing: boolean;
  touched: boolean;
  typed: string;
}

export interface SkuInputs {
  name: string;
  productType: string;
  area: string;
}

export function initialSkuField(existingSku?: string): SkuField {
  const editing = existingSku !== undefined;
  return { editing, touched: editing, typed: existingSku ?? "" };
}

export function typeSku(field: SkuField, value: string): SkuField {
  return { ...field, typed: value, touched: field.editing || value !== "" };
}

export function skuFieldValue(
  field: SkuField,
  inputs: SkuInputs,
  taken: ReadonlySet<string>,
): string {
  if (field.touched) return field.typed;
  if (!inputs.name.trim()) return "";
  return generateSku(inputs.name, inputs.productType, inputs.area, taken);
}
