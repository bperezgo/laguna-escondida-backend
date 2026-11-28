export interface RecipeLineDraft {
  key: string;
  ingredientId: string;
  quantity: string;
}

export interface IngredientCandidate {
  id: string;
  name: string;
  unit: string;
}

export type RecipeChange =
  | { kind: "add"; line: RecipeLineDraft }
  | { kind: "update"; line: RecipeLineDraft }
  | { kind: "remove"; key: string };

let seq = 0;
function lineKey(): string {
  seq += 1;
  return `line-${seq}`;
}

// The product page accepts 0 (an ingredient reached only through a slot choice); the catalog
// import wants a quantity above 0.
export function validRecipeQuantity(value: string, allowZero: boolean): boolean {
  if (value.trim() === "") return false;
  const qty = Number(value);
  if (!Number.isFinite(qty)) return false;
  return allowZero ? qty >= 0 : qty > 0;
}

export function quantityMessage(allowZero: boolean): string {
  return allowZero
    ? "La cantidad debe ser un número igual o mayor a 0"
    : "La cantidad debe ser un número mayor a 0";
}

export function availableCandidates(
  lines: readonly RecipeLineDraft[],
  candidates: readonly IngredientCandidate[],
): IngredientCandidate[] {
  const used = new Set(lines.map((l) => l.ingredientId));
  return candidates.filter((c) => !used.has(c.id));
}

export function addLine(
  lines: readonly RecipeLineDraft[],
  ingredientId: string,
  quantity: string,
): RecipeLineDraft[] {
  return [...lines, { key: lineKey(), ingredientId, quantity: quantity.trim() }];
}

export function setLineQuantity(
  lines: readonly RecipeLineDraft[],
  key: string,
  quantity: string,
): RecipeLineDraft[] {
  return lines.map((l) => (l.key === key ? { ...l, quantity: quantity.trim() } : l));
}

export function removeLine(lines: readonly RecipeLineDraft[], key: string): RecipeLineDraft[] {
  return lines.filter((l) => l.key !== key);
}

// The editor changes one line per action, so the API-bound wrapper turns it back into one call.
export function recipeChange(
  prev: readonly RecipeLineDraft[],
  next: readonly RecipeLineDraft[],
): RecipeChange | null {
  const prevByKey = new Map(prev.map((l) => [l.key, l]));
  const nextKeys = new Set(next.map((l) => l.key));
  for (const line of next) {
    const before = prevByKey.get(line.key);
    if (!before) return { kind: "add", line };
    if (before.quantity !== line.quantity) return { kind: "update", line };
  }
  for (const line of prev) {
    if (!nextKeys.has(line.key)) return { kind: "remove", key: line.key };
  }
  return null;
}
