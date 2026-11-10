import type { PlateOptionSlot } from "../../types/product.ts";

// A line's choices are kept as choice ids, at most one per slot; a slot without one shows its default.

export function selectedChoiceId(
  slot: PlateOptionSlot,
  choices: string[],
): string | undefined {
  const picked = slot.choices.find((c) => choices.includes(c.id));
  return (picked ?? slot.choices.find((c) => c.is_default))?.id;
}

export function selectChoice(
  slots: PlateOptionSlot[],
  choices: string[],
  slotId: string,
  choiceId: string,
): string[] {
  const slot = slots.find((s) => s.id === slotId);
  if (!slot) return choices;
  const inSlot = new Set(slot.choices.map((c) => c.id));
  return [...choices.filter((id) => !inSlot.has(id)), choiceId];
}

// One id per slot, resolved against the slots as loaded now: a choice removed from the menu since
// the line was taken would reject the whole order, so that slot falls back to its default.
export function choicesPayload(
  slots: PlateOptionSlot[],
  choices: string[],
): string[] {
  return slots
    .map((slot) => selectedChoiceId(slot, choices))
    .filter((id): id is string => !!id);
}
