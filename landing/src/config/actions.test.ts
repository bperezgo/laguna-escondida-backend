import assert from "node:assert/strict";
import { test } from "node:test";

import { demoAction, planAction, siteActions } from "./actions.ts";
import { buildSiteConfig } from "./build-site-config.ts";

const base = { PUBLIC_API_BASE_URL: "https://api.example.com" };

test("with a signup address the trial is primary, the demo secondary, and sign-in is on the same origin", () => {
  const actions = siteActions(buildSiteConfig({ ...base, PUBLIC_SIGNUP_URL: "https://app.kuenta.com/signup" }));

  assert.deepEqual(actions.primary, { label: "Prueba 14 días", href: "https://app.kuenta.com/signup", kind: "trial" });
  assert.deepEqual(actions.secondary, { label: "Pide una demo", href: "/#demo", kind: "demo" });
  assert.deepEqual(actions.signIn, { label: "Iniciar sesión", href: "https://app.kuenta.com/signin", kind: "signin" });
});

test("without a signup address the demo is primary and there is no trial or sign-in", () => {
  const actions = siteActions(buildSiteConfig(base));

  assert.deepEqual(actions.primary, { label: "Pide una demo", href: "/#demo", kind: "demo" });
  assert.equal(actions.secondary, undefined);
  assert.equal(actions.signIn, undefined);
});

test("a signup address with a path and a trailing slash still gives the right sign-in address", () => {
  const actions = siteActions(buildSiteConfig({ ...base, PUBLIC_SIGNUP_URL: "https://app.kuenta.com/cuenta/signup/" }));

  assert.equal(actions.signIn?.href, "https://app.kuenta.com/signin");
});

test("a plan's trial action opens signup with that plan, and the site-wide trial action carries none", () => {
  const { primary } = siteActions(buildSiteConfig({ ...base, PUBLIC_SIGNUP_URL: "https://app.kuenta.com/signup" }));

  assert.deepEqual(planAction(primary, "full"), { label: "Prueba 14 días", href: "https://app.kuenta.com/signup?plan=full", kind: "trial" });
  assert.equal(primary.href, "https://app.kuenta.com/signup");
});

test("a signup address that already has a query keeps it and gains the plan", () => {
  const { primary } = siteActions(buildSiteConfig({ ...base, PUBLIC_SIGNUP_URL: "https://app.kuenta.com/signup?ref=landing" }));

  assert.equal(planAction(primary, "einvoicing").href, "https://app.kuenta.com/signup?ref=landing&plan=einvoicing");
});

test("when the demo is the primary action a plan changes nothing", () => {
  const { primary } = siteActions(buildSiteConfig(base));

  assert.deepEqual(planAction(primary, "starter"), demoAction);
});
