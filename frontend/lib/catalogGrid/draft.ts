import type { SlotDraft } from "../products/optionSlots.ts";
import type { RecipeLineDraft } from "../products/recipeLines.ts";
import { MAX_ROWS, newRow, PASTE_COLUMNS, type Column, type GridRow, type GridState, type UnmatchedExisting } from "./state.ts";
import { GRID_TYPES, GRID_UNITS, type GridType } from "./values.ts";

export const DRAFT_VERSION = 1;

export interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export function draftStorageKey(organizationId: string, userId: string): string {
  return `catalog-grid:v1:${organizationId}:${userId}`;
}

export function browserStorage(): StorageLike | null {
  try {
    const storage = globalThis.localStorage;
    return storage ?? null;
  } catch {
    return null;
  }
}

export interface LoadedDraft {
  state: GridState | null;
  available: boolean;
}

function asString(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function asArray(v: unknown): unknown[] {
  return Array.isArray(v) ? v : [];
}

function asRecord(v: unknown): Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
}

// The stored value is rebuilt field by field, so an edited or foreign value can't add fields
// or break the grid; anything unreadable falls back to an empty field.
function reviveRow(raw: unknown): GridRow | null {
  const o = asRecord(raw);
  const id = asString(o.id);
  const type = asString(o.type) as GridType;
  if (!id || !GRID_TYPES.some((t) => t.value === type)) return null;
  const row = newRow(type, id);
  row.name = asString(o.name);
  row.category = asString(o.category);
  row.area = asString(o.area);
  const unit = asString(o.unit);
  row.unit = GRID_UNITS.some((u) => u === unit) ? (unit as GridRow["unit"]) : "";
  row.price = asString(o.price);
  row.ico = asString(o.ico);
  row.vat = asString(o.vat);
  row.sku = asString(o.sku);
  row.skuTouched = o.skuTouched === true;
  row.stock = asString(o.stock);
  row.description = asString(o.description);
  row.priority = typeof o.priority === "number" && Number.isInteger(o.priority) ? o.priority : 0;
  row.recipe = asArray(o.recipe).map((l): RecipeLineDraft => {
    const line = asRecord(l);
    return { key: asString(line.key), ingredientId: asString(line.ingredientId), quantity: asString(line.quantity) };
  });
  row.slots = asArray(o.slots).map((s): SlotDraft => {
    const slot = asRecord(s);
    return {
      key: asString(slot.key),
      name: asString(slot.name),
      choices: asArray(slot.choices).map((c) => {
        const choice = asRecord(c);
        return {
          key: asString(choice.key),
          name: asString(choice.name),
          isDefault: choice.isDefault === true,
          items: asArray(choice.items).map((it) => {
            const item = asRecord(it);
            return {
              key: asString(item.key),
              ingredientProductId: asString(item.ingredientProductId),
              quantity: asString(item.quantity),
            };
          }),
        };
      }),
    };
  });
  const marks: Partial<Record<Column, true>> = {};
  const rawMarks = asRecord(o.marks);
  for (const column of Object.keys(rawMarks)) {
    const known = PASTE_COLUMNS.indexOf(column as Column) >= 0 || column === "detail";
    if (rawMarks[column] === true && known) marks[column as Column] = true;
  }
  row.marks = marks;
  return row;
}

function reviveState(raw: unknown): GridState | null {
  const o = asRecord(raw);
  if (o.version !== DRAFT_VERSION) return null;
  const state = asRecord(o.state);
  const rows = asArray(state.rows)
    .slice(0, MAX_ROWS)
    .map(reviveRow)
    .filter((r): r is GridRow => r !== null);
  const unmatched = asArray(state.unmatched).map((u): UnmatchedExisting => {
    const entry = asRecord(u);
    const out: UnmatchedExisting = { key: asString(entry.key) };
    if (typeof entry.sku === "string") out.sku = entry.sku;
    if (typeof entry.name === "string") out.name = entry.name;
    return out;
  });
  return { rows, unmatched };
}

export function loadDraft(storage: StorageLike | null, organizationId: string, userId: string): LoadedDraft {
  if (!storage) return { state: null, available: false };
  let text: string | null;
  try {
    text = storage.getItem(draftStorageKey(organizationId, userId));
  } catch {
    return { state: null, available: false };
  }
  if (text === null) return { state: null, available: true };
  try {
    return { state: reviveState(JSON.parse(text)), available: true };
  } catch {
    return { state: null, available: true };
  }
}

// Returns false when the browser refused to store it, so the page can say the draft is not saved.
export function saveDraft(
  storage: StorageLike | null,
  organizationId: string,
  userId: string,
  state: GridState,
): boolean {
  if (!storage) return false;
  try {
    const key = draftStorageKey(organizationId, userId);
    if (state.rows.length === 0 && state.unmatched.length === 0) storage.removeItem(key);
    else storage.setItem(key, JSON.stringify({ version: DRAFT_VERSION, state }));
    return true;
  } catch {
    return false;
  }
}

export function clearDraft(storage: StorageLike | null, organizationId: string, userId: string): void {
  if (!storage) return;
  try {
    storage.removeItem(draftStorageKey(organizationId, userId));
  } catch {
    // Nothing left to do: the next save overwrites it, and an unreadable key is ignored on load.
  }
}
