import { test } from "node:test";
import assert from "node:assert/strict";
import {
  computeCashChange,
  parseReceivedAmount,
  suggestCashAmounts,
} from "./cashChange.ts";

test("suggestions for a typical total", () => {
  assert.deepEqual(suggestCashAmounts(47500), [50000, 60000, 100000]);
});

test("suggestions for a small total", () => {
  assert.deepEqual(suggestCashAmounts(12300), [15000, 20000, 50000]);
});

test("a round total suggests amounts strictly above it", () => {
  assert.deepEqual(suggestCashAmounts(50000), [55000, 60000, 100000]);
});

test("suggestions ignore float noise in the total", () => {
  assert.deepEqual(suggestCashAmounts(47499.99999), [50000, 60000, 100000]);
});

test("overpaying gives the change", () => {
  assert.deepEqual(computeCashChange(47500, 100000), {
    kind: "change",
    amount: 52500,
  });
});

test("the exact amount gives zero change", () => {
  assert.deepEqual(computeCashChange(47500, 47500), {
    kind: "change",
    amount: 0,
  });
});

test("underpaying gives the missing amount", () => {
  assert.deepEqual(computeCashChange(47500, 45000), {
    kind: "short",
    missing: 2500,
  });
});

test("no received amount gives an empty result", () => {
  assert.deepEqual(computeCashChange(47500, null), { kind: "empty" });
});

test("received input with thousands separators parses", () => {
  assert.equal(parseReceivedAmount("100.000"), 100000);
  assert.equal(parseReceivedAmount("$ 100.000"), 100000);
  assert.equal(parseReceivedAmount("100000"), 100000);
});

test("empty received input parses as nothing", () => {
  assert.equal(parseReceivedAmount(""), null);
  assert.equal(parseReceivedAmount("  "), null);
  assert.equal(parseReceivedAmount("abc"), null);
});
