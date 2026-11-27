import assert from "node:assert/strict";
import { test } from "node:test";

import { annualPrice, plans } from "../src/config/plans.ts";
import { comparePrices, parsePlanPrices } from "./compare-prices.ts";

const cloudAnswer = {
  currency: "COP",
  trial_days: 14,
  plans: [
    { id: "starter", monthly: 25000, annual: 250000 },
    { id: "einvoicing", monthly: 40000, annual: 400000 },
    { id: "full", monthly: 100000, annual: 1000000 },
  ],
};

const site = plans.map((plan) => ({ id: plan.id, monthly: plan.monthly, annual: annualPrice(plan) }));

test("the site's prices match what the cloud returns", () => {
  assert.deepEqual(comparePrices(site, parsePlanPrices(cloudAnswer)), []);
});

test("one wrong price fails and names the plan and the period", () => {
  const drifted = site.map((plan) => (plan.id === "einvoicing" ? { ...plan, monthly: 45000 } : plan));

  const problems = comparePrices(drifted, parsePlanPrices(cloudAnswer));

  assert.equal(problems.length, 1);
  assert.match(problems[0] ?? "", /"einvoicing" monthly/);
  assert.match(problems[0] ?? "", /45000/);
  assert.match(problems[0] ?? "", /40000/);
});

test("every plan and period that differs is named", () => {
  const drifted = site.map((plan) => (plan.id === "starter" ? plan : { ...plan, annual: plan.annual + 1 }));

  const problems = comparePrices(drifted, parsePlanPrices(cloudAnswer));

  assert.deepEqual(problems.map((line) => line.split(":")[0]), ['plan "einvoicing" annual', 'plan "full" annual']);
});

test("a plan missing on either side fails and is named", () => {
  const cloud = parsePlanPrices(cloudAnswer);

  assert.match(comparePrices(site, cloud.filter((plan) => plan.id !== "full")).join("\n"), /"full" is on the site but the cloud does not return it/);
  assert.match(comparePrices(site.filter((plan) => plan.id !== "starter"), cloud).join("\n"), /"starter" is returned by the cloud but is not on the site/);
});

test("an answer that is not the plans contract is refused", () => {
  assert.throws(() => parsePlanPrices({ error: "not found" }), /no `plans` list/);
  assert.throws(() => parsePlanPrices({ plans: [{ id: "starter", monthly: "25000", annual: 250000 }] }), /malformed/);
});
