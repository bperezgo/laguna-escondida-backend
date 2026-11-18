import { test } from "node:test";
import assert from "node:assert/strict";
import type { PlateOptionSlot } from "../../types/product.ts";
import {
  choicesPayload,
  selectChoice,
  selectedChoiceId,
} from "./lineChoices.ts";

function slot(id: string, choiceIds: string[]): PlateOptionSlot {
  return {
    id,
    composite_product_id: "plate",
    name: id,
    position: 0,
    choices: choiceIds.map((c, i) => ({ id: c, name: c, position: i, is_default: i === 0, items: [] })),
  };
}

const slots = [slot("arroz", ["blanco", "coco"]), slot("acomp", ["canastas", "papas", "patacon"])];

test("a slot without a pick shows its default", () => {
  assert.equal(selectedChoiceId(slots[0], []), "blanco");
  assert.equal(selectedChoiceId(slots[1], ["papas"]), "papas");
});

test("picking a choice replaces the slot's previous pick and keeps the others", () => {
  let choices = selectChoice(slots, [], "acomp", "papas");
  choices = selectChoice(slots, choices, "arroz", "coco");
  choices = selectChoice(slots, choices, "acomp", "patacon");
  assert.deepEqual(choices.sort(), ["coco", "patacon"]);
  assert.deepEqual(selectChoice(slots, choices, "unknown", "x"), choices);
});

test("the payload sends one known choice per slot", () => {
  assert.deepEqual(choicesPayload(slots, []), ["blanco", "canastas"]);
  assert.deepEqual(choicesPayload(slots, ["papas", "coco"]), ["coco", "papas"]);
});

test("a choice removed from the menu falls back to the default", () => {
  assert.deepEqual(choicesPayload(slots, ["gone"]), ["blanco", "canastas"]);
  assert.deepEqual(choicesPayload([], ["papas"]), []);
});
