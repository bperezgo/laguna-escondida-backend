import type {
  PlateOptionSlot,
  ReplacePlateOptionSlotsRequest,
} from "../../types/product.ts";

// Editor drafts carry a local key so React rows stay stable before the backend assigns ids.
export interface ItemDraft {
  key: string;
  ingredientProductId: string;
  quantity: string;
}

export interface ChoiceDraft {
  key: string;
  id?: string;
  name: string;
  isDefault: boolean;
  items: ItemDraft[];
}

export interface SlotDraft {
  key: string;
  id?: string;
  name: string;
  choices: ChoiceDraft[];
}

let seq = 0;
export function draftKey(): string {
  seq += 1;
  return `draft-${seq}`;
}

export function newChoice(isDefault: boolean): ChoiceDraft {
  return { key: draftKey(), name: "", isDefault, items: [] };
}

export function newSlot(): SlotDraft {
  return { key: draftKey(), name: "", choices: [newChoice(true)] };
}

export function draftsFromSaved(slots: PlateOptionSlot[]): SlotDraft[] {
  return slots.map((s) => ({
    key: s.id,
    id: s.id,
    name: s.name,
    choices: s.choices.map((c) => ({
      key: c.id,
      id: c.id,
      name: c.name,
      isDefault: c.is_default,
      items: c.items.map((it) => ({
        key: it.id,
        ingredientProductId: it.ingredient_product_id,
        quantity: String(it.quantity),
      })),
    })),
  }));
}

// Ids go back unchanged so open orders and cached choice ids keep pointing at the same choice.
export function draftsToRequest(
  drafts: SlotDraft[],
): ReplacePlateOptionSlotsRequest {
  return {
    slots: drafts.map((s) => ({
      ...(s.id ? { id: s.id } : {}),
      name: s.name.trim(),
      choices: s.choices.map((c) => ({
        ...(c.id ? { id: c.id } : {}),
        name: c.name.trim(),
        is_default: c.isDefault,
        items: c.items.map((it) => ({
          ingredient_product_id: it.ingredientProductId,
          quantity: Number(it.quantity),
        })),
      })),
    })),
  };
}

function slotLabel(s: SlotDraft, index: number): string {
  return s.name.trim() || `#${index + 1}`;
}

// Mirrors the backend rules so the admin sees the problem before saving; the backend stays the authority.
export function validateDrafts(drafts: SlotDraft[], plateId: string): string | null {
  const slotNames = new Set<string>();
  for (let i = 0; i < drafts.length; i++) {
    const slot = drafts[i];
    const label = slotLabel(slot, i);
    const name = slot.name.trim().toLowerCase();
    if (!name) return `El grupo ${label} necesita un nombre`;
    if (slotNames.has(name)) return `Hay dos grupos llamados "${slot.name.trim()}"`;
    slotNames.add(name);

    if (slot.choices.length === 0) return `El grupo "${label}" necesita al menos una opción`;
    const defaults = slot.choices.filter((c) => c.isDefault).length;
    if (defaults !== 1) return `El grupo "${label}" debe tener exactamente una opción predeterminada`;

    const choiceNames = new Set<string>();
    for (const choice of slot.choices) {
      const cname = choice.name.trim().toLowerCase();
      if (!cname) return `Una opción del grupo "${label}" no tiene nombre`;
      if (choiceNames.has(cname)) return `El grupo "${label}" tiene dos opciones "${choice.name.trim()}"`;
      choiceNames.add(cname);

      const ingredients = new Set<string>();
      for (const item of choice.items) {
        if (!item.ingredientProductId) return `Selecciona el ingrediente en "${choice.name.trim()}"`;
        if (item.ingredientProductId === plateId) return `"${choice.name.trim()}" no puede incluir el mismo plato`;
        if (ingredients.has(item.ingredientProductId)) {
          return `"${choice.name.trim()}" repite un ingrediente`;
        }
        ingredients.add(item.ingredientProductId);
        const qty = Number(item.quantity);
        if (item.quantity.trim() === "" || !Number.isInteger(qty) || qty < 1) {
          return `Las cantidades de "${choice.name.trim()}" deben ser números enteros desde 1`;
        }
      }
    }
  }
  return null;
}

export function setDefaultChoice(slot: SlotDraft, choiceKey: string): SlotDraft {
  return {
    ...slot,
    choices: slot.choices.map((c) => ({ ...c, isDefault: c.key === choiceKey })),
  };
}

// Removing the default hands it to the first remaining choice so the slot never has none.
export function removeChoice(slot: SlotDraft, choiceKey: string): SlotDraft {
  const choices = slot.choices.filter((c) => c.key !== choiceKey);
  if (choices.length > 0 && !choices.some((c) => c.isDefault)) {
    choices[0] = { ...choices[0], isDefault: true };
  }
  return { ...slot, choices };
}

export function move<T>(list: T[], index: number, delta: -1 | 1): T[] {
  const target = index + delta;
  if (target < 0 || target >= list.length) return list;
  const next = [...list];
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}
