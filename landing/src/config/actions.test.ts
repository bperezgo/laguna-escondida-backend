import assert from "node:assert/strict";
import { test } from "node:test";

import { siteActions } from "./actions.ts";
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
