import { test } from "node:test";
import assert from "node:assert/strict";
import { featureVisible, pageVisible, planRequiredNotice, PLAN_CARDS, PLANS, RECOMMENDED_PLAN, formatPesos, parsePlanPrices, planFromQuery, planPrice } from "./plans.ts";

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

test("the plan named by the landing is selected, and the recommended one otherwise", () => {
  assert.equal(planFromQuery("full"), "full");
  assert.equal(planFromQuery("starter"), "starter");
  assert.equal(planFromQuery("gold"), "einvoicing");
  assert.equal(planFromQuery(undefined), "einvoicing");
  assert.equal(planFromQuery(["full", "starter"]), "full");
  assert.equal(RECOMMENDED_PLAN, "einvoicing");
});

test("pesos are written with a dot every three digits and no decimals", () => {
  assert.equal(formatPesos(400000), "$400.000");
  assert.equal(formatPesos(1000000), "$1.000.000");
  assert.equal(formatPesos(25000), "$25.000");
  assert.equal(formatPesos(0), "$0");
  assert.equal(formatPesos(999), "$999");
});

const priceList = {
  currency: "COP",
  trial_days: 14,
  plans: [
    { id: "starter", monthly: 25000, annual: 250000 },
    { id: "einvoicing", monthly: 40000, annual: 400000 },
    { id: "full", monthly: 100000, annual: 1000000 },
  ],
};

test("the price list is read as the backend sends it", () => {
  const prices = parsePlanPrices(priceList);
  assert.equal(prices?.trialDays, 14);
  assert.equal(planPrice(prices, "einvoicing", "annual"), 400000);
  assert.equal(planPrice(prices, "full", "monthly"), 100000);
});

test("an incomplete or unreadable price list means no prices, not wrong ones", () => {
  assert.equal(parsePlanPrices(null), null);
  assert.equal(parsePlanPrices({ trial_days: 14, plans: priceList.plans.slice(0, 2) }), null);
  assert.equal(parsePlanPrices({ ...priceList, plans: [{ id: "starter", monthly: "25000", annual: 250000 }] }), null);
  assert.equal(planPrice(null, "full", "monthly"), null);
});
