import { test } from "node:test";
import assert from "node:assert/strict";
import { clearDraft, draftStorageKey, loadDraft, saveDraft, type StorageLike } from "./draft.ts";
import { newRow, rowRef, type GridState } from "./state.ts";

function memoryStorage(): StorageLike & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return {
    data,
    getItem: (k) => data.get(k) ?? null,
    setItem: (k, v) => void data.set(k, v),
    removeItem: (k) => void data.delete(k),
  };
}

function grid(n: number): GridState {
  const rows = Array.from({ length: n }, (_, i) => ({ ...newRow("SELLABLE", `r${i}`), name: `Producto ${i}`, price: "1000" }));
  rows[0] = {
    ...newRow("COMPOSITE", "r0"),
    name: "Bandeja",
    recipe: [{ key: "l1", ingredientId: rowRef("r1"), quantity: "200" }],
    slots: [{ key: "s", name: "Arroz", choices: [{ key: "c", name: "Blanco", isDefault: true, items: [] }] }],
  };
  return { rows, unmatched: [{ key: "u1", name: "Yuca" }] };
}

test("Reload keeps work", () => {
  const storage = memoryStorage();
  const before = grid(40);
  assert.equal(saveDraft(storage, "org-a", "user-1", before), true);
  const loaded = loadDraft(storage, "org-a", "user-1");
  assert.equal(loaded.available, true);
  assert.deepEqual(loaded.state, before);
});

test("Another organization's draft not shown", () => {
  const storage = memoryStorage();
  saveDraft(storage, "org-a", "user-1", grid(3));
  assert.deepEqual(loadDraft(storage, "org-b", "user-2"), { state: null, available: true });
  assert.deepEqual(loadDraft(storage, "org-b", "user-1"), { state: null, available: true });
  assert.deepEqual(loadDraft(storage, "org-a", "user-2"), { state: null, available: true });
  assert.deepEqual(Array.from(storage.data.keys()), [draftStorageKey("org-a", "user-1")]);
});

test("Storage unavailable", () => {
  const throwing: StorageLike = {
    getItem: () => {
      throw new Error("SecurityError");
    },
    setItem: () => {
      throw new Error("QuotaExceededError");
    },
    removeItem: () => {
      throw new Error("SecurityError");
    },
  };
  assert.deepEqual(loadDraft(throwing, "org-a", "user-1"), { state: null, available: false });
  assert.equal(saveDraft(throwing, "org-a", "user-1", grid(1)), false);
  assert.doesNotThrow(() => clearDraft(throwing, "org-a", "user-1"));
  assert.deepEqual(loadDraft(null, "org-a", "user-1"), { state: null, available: false });
  assert.equal(saveDraft(null, "org-a", "user-1", grid(1)), false);
});

test("the draft is removed after create", () => {
  const storage = memoryStorage();
  saveDraft(storage, "org-a", "user-1", grid(2));
  clearDraft(storage, "org-a", "user-1");
  assert.equal(storage.data.size, 0);
});

test("an emptied grid removes the draft instead of saving it", () => {
  const storage = memoryStorage();
  saveDraft(storage, "org-a", "user-1", grid(2));
  saveDraft(storage, "org-a", "user-1", { rows: [], unmatched: [] });
  assert.equal(storage.data.size, 0);
});

test("an unknown version or an unreadable value is ignored", () => {
  const storage = memoryStorage();
  const key = draftStorageKey("org-a", "user-1");
  storage.setItem(key, JSON.stringify({ version: 2, state: grid(1) }));
  assert.equal(loadDraft(storage, "org-a", "user-1").state, null);
  storage.setItem(key, "{oops");
  assert.equal(loadDraft(storage, "org-a", "user-1").state, null);
});

test("a tampered row is rebuilt from known fields only", () => {
  const storage = memoryStorage();
  const key = draftStorageKey("org-a", "user-1");
  storage.setItem(
    key,
    JSON.stringify({
      version: 1,
      state: {
        rows: [
          { id: "a", type: "SELLABLE", name: 7, extra: "x", marks: { price: true, bogus: true, constructor: true, toString: true } },
          { id: "b", type: "WEIRD" },
        ],
        unmatched: "nope",
      },
    }),
  );
  const state = loadDraft(storage, "org-a", "user-1").state!;
  assert.equal(state.rows.length, 1);
  assert.equal(state.rows[0].name, "");
  assert.equal("extra" in state.rows[0], false);
  assert.deepEqual(state.rows[0].marks, { price: true });
  assert.deepEqual(state.unmatched, []);
});
