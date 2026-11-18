import { test } from "node:test";
import assert from "node:assert/strict";
import {
  createTabAttention,
  formatTabTitle,
  type TabAttentionState,
} from "./tabAttention.ts";

test("formatTabTitle prefixes the count only when there is one", () => {
  assert.equal(formatTabTitle(0, "Laguna Escondida"), "Laguna Escondida");
  assert.equal(formatTabTitle(2, "Laguna Escondida"), "(2) Laguna Escondida");
});

test("each alert adds one and reset goes back to zero", () => {
  const rendered: TabAttentionState[] = [];
  const attention = createTabAttention((s) => rendered.push(s));
  attention.setBaseTitle("Laguna Escondida");
  attention.add();
  attention.add();
  assert.equal(attention.getState().count, 2);
  attention.reset();
  assert.deepEqual(rendered.at(-1), { count: 0, baseTitle: "Laguna Escondida" });
});

test("setting the base title on a page change keeps the count", () => {
  const rendered: TabAttentionState[] = [];
  const attention = createTabAttention((s) => rendered.push(s));
  attention.setBaseTitle("Laguna Escondida");
  attention.add();
  attention.setBaseTitle("Laguna Escondida");
  const last = rendered.at(-1)!;
  assert.equal(formatTabTitle(last.count, last.baseTitle!), "(1) Laguna Escondida");
});

test("reset at zero does not re-render", () => {
  let renders = 0;
  const attention = createTabAttention(() => renders++);
  attention.reset();
  assert.equal(renders, 0);
});
