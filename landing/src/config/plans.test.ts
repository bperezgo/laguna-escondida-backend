import assert from "node:assert/strict";
import { test } from "node:test";

import { annualPrice, formatCop, plans } from "./plans.ts";

test("the three plans cost 25000, 40000 and 100000 a month, in that order", () => {
  assert.deepEqual(plans.map((p) => p.id), ["starter", "einvoicing", "full"]);
  assert.deepEqual(plans.map((p) => p.monthly), [25000, 40000, 100000]);
});

test("Facturación electrónica is the one recommended plan", () => {
  assert.deepEqual(plans.filter((p) => p.recommended).map((p) => p.id), ["einvoicing"]);
});

test("a year costs ten months", () => {
  assert.deepEqual(plans.map(annualPrice), [250000, 400000, 1000000]);
});

test("prices are written the Colombian way", () => {
  assert.equal(formatCop(25000), "$25.000");
  assert.equal(formatCop(1000000), "$1.000.000");
  assert.equal(formatCop(900), "$900");
});

test("only the Completo plan mentions the WhatsApp agent, as a connection contracted apart", () => {
  const mentions = plans.flatMap((p) => p.includes.filter((line) => /whatsapp|agente/i.test(line)).map((line) => ({ id: p.id, line })));

  assert.equal(mentions.length, 1);
  assert.equal(mentions[0]?.id, "full");
  assert.match(mentions[0]?.line ?? "", /^Conexión con el agente de WhatsApp/);
  assert.match(mentions[0]?.line ?? "", /se contrata aparte/);
});
