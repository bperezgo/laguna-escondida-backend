import { test } from "node:test";
import assert from "node:assert/strict";
import { featureVisible, pageVisible, planRequiredNotice, PLAN_CARDS, PLANS } from "./plans.ts";

const gated = [
  "/invoices",
  "/support-documents",
  "/reservations",
  "/feedback",
  "/deliveries",
  "/delivery-zones",
  "/my-deliveries",
  "/ajustes/notificaciones",
  "/ajustes/integraciones",
];
const core = ["/orders", "/products", "/stock", "/users", "/expenses", "/ajustes", "/ajustes/instalar-caja"];

test("starter hides every gated page and keeps core", () => {
  for (const path of gated) assert.equal(pageVisible(path, "starter", false), false, path);
  for (const path of core) assert.equal(pageVisible(path, "starter", false), true, path);
  assert.equal(featureVisible("agents", "starter", false), false);
});

test("einvoicing adds invoices and support documents only", () => {
  assert.equal(pageVisible("/invoices", "einvoicing", false), true);
  assert.equal(pageVisible("/support-documents", "einvoicing", false), true);
  assert.equal(pageVisible("/reservations", "einvoicing", false), false);
  assert.equal(pageVisible("/ajustes/integraciones", "einvoicing", false), false);
  assert.equal(featureVisible("agents", "einvoicing", false), false);
});

test("full shows everything", () => {
  for (const path of [...gated, ...core]) assert.equal(pageVisible(path, "full", false), true, path);
  assert.equal(featureVisible("agents", "full", false), true);
});

test("the edge and an unknown plan hide nothing", () => {
  for (const path of gated) {
    assert.equal(pageVisible(path, "starter", true), true, path);
    assert.equal(pageVisible(path, undefined, false), true, path);
  }
});

test("nested pages follow their prefix, and a lookalike prefix does not", () => {
  assert.equal(pageVisible("/reservations/ajustes", "starter", false), false);
  assert.equal(pageVisible("/invoicesx", "starter", false), true);
});

test("there is a card for every plan", () => {
  assert.deepEqual(PLAN_CARDS.map((c) => c.plan), [...PLANS]);
});

test("the upgrade notice names the plan and links support only when set", () => {
  const withEmail = planRequiredNotice("einvoicing", "soporte@kuenta.co");
  assert.match(withEmail.message, /Facturación electrónica/);
  assert.match(withEmail.message, /soporte@kuenta\.co/);
  assert.ok(withEmail.mailto?.startsWith("mailto:soporte@kuenta.co"));
  const without = planRequiredNotice("full", null);
  assert.match(without.message, /Completo/);
  assert.equal(without.mailto, null);
  assert.doesNotMatch(planRequiredNotice(undefined, null).message, /plan undefined/);
});
