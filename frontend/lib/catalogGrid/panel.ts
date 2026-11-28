import { findDraftProblem, type SlotDraft } from "../products/optionSlots.ts";
import { validRecipeQuantity, type RecipeLineDraft } from "../products/recipeLines.ts";
import { rowRef } from "./state.ts";

export type PanelProblem =
  | { kind: "recipe"; key: string; message: string }
  | { kind: "slot"; key: string; message: string };

// Checked before the plate panel closes; the import still validates everything on Revisar.
export function panelProblem(plateId: string, recipe: RecipeLineDraft[], slots: SlotDraft[]): PanelProblem | null {
  for (const line of recipe) {
    if (!validRecipeQuantity(line.quantity, false)) {
      return { kind: "recipe", key: line.key, message: "Cada ingrediente de la receta necesita una cantidad mayor a 0" };
    }
  }
  const slot = findDraftProblem(slots, rowRef(plateId));
  return slot ? { kind: "slot", key: slot.slotKey, message: slot.message } : null;
}
