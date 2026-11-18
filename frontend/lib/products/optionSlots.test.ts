import { test } from "node:test";
import assert from "node:assert/strict";
import type { PlateOptionSlot } from "../../types/product.ts";
import {
  draftsFromSaved,
  draftsToRequest,
  move,
  newChoice,
  newSlot,
  removeChoice,
  setDefaultChoice,
  validateDrafts,
  type SlotDraft,
} from "./optionSlots.ts";

const PLATE = "plate-1";

const saved: PlateOptionSlot[] = [
  {
    id: "slot-1",
    composite_product_id: PLATE,
    name: "Acompañante",
    position: 0,
    choices: [
      {
        id: "c-canastas",
        name: "Canastas",
        position: 0,
        is_default: true,
        items: [{ id: "i-1", ingredient_product_id: "canasta", ingredient_name: "Canasta", quantity: 2 }],
      },
      { id: "c-sin", name: "Sin acompañante", position: 1, is_default: false, items: [] },
    ],
  },
];

function slot(overrides: Partial<SlotDraft> = {}): SlotDraft {
  return {
    key: "s",
    name: "Arroz",
    choices: [{ key: "a", name: "Blanco", isDefault: true, items: [] }],
    ...overrides,
  };
}

test("a saved list round-trips to a request that keeps every id", () => {
  const req = draftsToRequest(draftsFromSaved(saved));
  assert.deepEqual(req, {
    slots: [
      {
        id: "slot-1",
        name: "Acompañante",
        choices: [
          { id: "c-canastas", name: "Canastas", is_default: true, items: [{ ingredient_product_id: "canasta", quantity: 2 }] },
          { id: "c-sin", name: "Sin acompañante", is_default: false, items: [] },
        ],
      },
    ],
  });
});

test("new slots and choices are sent without ids and with trimmed names", () => {
  const s = newSlot();
  s.name = "  Arroz ";
  s.choices[0].name = "Blanco ";
  const req = draftsToRequest([s]);
  assert.deepEqual(req.slots[0], {
    name: "Arroz",
    choices: [{ name: "Blanco", is_default: true, items: [] }],
  });
});

test("a valid list passes, and an empty list clears every slot", () => {
  assert.equal(validateDrafts(draftsFromSaved(saved), PLATE), null);
  assert.equal(validateDrafts([], PLATE), null);
});

test("validation mirrors the backend rules", () => {
  const cases: [string, SlotDraft[], RegExp][] = [
    ["slot without name", [slot({ name: " " })], /necesita un nombre/],
    ["duplicate slot names ignore case", [slot(), slot({ key: "s2", name: "arroz" })], /dos grupos/],
    ["no choices", [slot({ choices: [] })], /al menos una opción/],
    [
      "two defaults",
      [slot({ choices: [{ key: "a", name: "A", isDefault: true, items: [] }, { key: "b", name: "B", isDefault: true, items: [] }] })],
      /exactamente una/,
    ],
    ["no default", [slot({ choices: [{ key: "a", name: "A", isDefault: false, items: [] }] })], /exactamente una/],
    [
      "duplicate choice names",
      [slot({ choices: [{ key: "a", name: "A", isDefault: true, items: [] }, { key: "b", name: "a", isDefault: false, items: [] }] })],
      /dos opciones/,
    ],
    [
      "ingredient not picked",
      [slot({ choices: [{ key: "a", name: "A", isDefault: true, items: [{ key: "i", ingredientProductId: "", quantity: "1" }] }] })],
      /Selecciona el ingrediente/,
    ],
    [
      "the plate itself",
      [slot({ choices: [{ key: "a", name: "A", isDefault: true, items: [{ key: "i", ingredientProductId: PLATE, quantity: "1" }] }] })],
      /mismo plato/,
    ],
    [
      "repeated ingredient",
      [
        slot({
          choices: [
            {
              key: "a",
              name: "A",
              isDefault: true,
              items: [
                { key: "i", ingredientProductId: "x", quantity: "1" },
                { key: "j", ingredientProductId: "x", quantity: "2" },
              ],
            },
          ],
        }),
      ],
      /repite/,
    ],
  ];
  for (const [name, drafts, want] of cases) {
    assert.match(validateDrafts(drafts, PLATE) ?? "", want, name);
  }
  for (const qty of ["0", "1.5", "", "-1"]) {
    const drafts = [slot({ choices: [{ key: "a", name: "A", isDefault: true, items: [{ key: "i", ingredientProductId: "x", quantity: qty }] }] })];
    assert.match(validateDrafts(drafts, PLATE) ?? "", /enteros desde 1/, `quantity ${JSON.stringify(qty)}`);
  }
});

test("picking a default clears the previous one", () => {
  const s = slot({ choices: [newChoice(true), newChoice(false)] });
  const next = setDefaultChoice(s, s.choices[1].key);
  assert.deepEqual(next.choices.map((c) => c.isDefault), [false, true]);
});

test("removing the default moves it to the first remaining choice", () => {
  const s = slot({ choices: [newChoice(true), newChoice(false), newChoice(false)] });
  const next = removeChoice(s, s.choices[0].key);
  assert.deepEqual(next.choices.map((c) => c.isDefault), [true, false]);
  assert.equal(removeChoice(next, next.choices[1].key).choices[0].isDefault, true);
});

test("move swaps neighbours and ignores the edges", () => {
  assert.deepEqual(move(["a", "b", "c"], 1, -1), ["b", "a", "c"]);
  assert.deepEqual(move(["a", "b", "c"], 2, 1), ["a", "b", "c"]);
  assert.deepEqual(move(["a", "b", "c"], 0, -1), ["a", "b", "c"]);
});
