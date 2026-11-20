import { test } from "node:test";
import assert from "node:assert/strict";
import type { MarketingAppIntegration, WompiIntegration } from "../../types/organizationIntegrations.ts";
import {
  MARKETING_APP_PASTE_NOTE,
  MARKETING_APP_REGENERATE_WARNING,
  marketingAppActions,
  marketingAppCardState,
  marketingAppDismissed,
  marketingAppErrorMessage,
  marketingAppGenerated,
  maskedHint,
  wompiCardState,
} from "./integrations.ts";

const marketingApp: MarketingAppIntegration = {
  provider: "marketing_app",
  signing_secret_hint: "Q7xk",
  created_at: "2026-11-19T15:04:05Z",
  updated_at: "2026-11-19T15:04:05Z",
};

const wompi: WompiIntegration = {
  provider: "wompi",
  environment: "sandbox",
  private_key_hint: "wxyz",
  events_secret_hint: "9876",
  webhook_path: "/api/webhooks/wompi/019a1c2e-5f00-7a4b-8c3d-2e1f0a9b8c7d",
  created_at: "2026-11-19T15:04:05Z",
  updated_at: "2026-11-19T15:04:05Z",
};

const generatedSecret = "mJ3vN0b9cRk2LhT8yWq1ZeA5sUo4XgPd6FiCnVlQ7xk"; // gitleaks:allow (test fixture)

test("a server without an encryption key shows the card unavailable", () => {
  assert.deepEqual(marketingAppCardState({ status: 503, code: "INTEGRATIONS_UNAVAILABLE" }), { kind: "unavailable" });
});

test("first secret: not configured offers only Generar to a manager of integrations", () => {
  const state = marketingAppCardState({ status: 200, integrations: [wompi] });
  assert.deepEqual(state, { kind: "not_configured" });
  assert.deepEqual(marketingAppActions(state, true), ["generate"]);
});

test("first secret: generating shows the full secret and the paste-on-every-channel note", () => {
  const state = marketingAppGenerated({ ...marketingApp, signing_secret: generatedSecret });
  assert.equal(state.kind, "generated");
  assert.equal(state.kind === "generated" && state.secret, generatedSecret);
  assert.match(MARKETING_APP_PASTE_NOTE, /cada canal/);
  assert.deepEqual(marketingAppActions(state, true), [], "only Listo while the secret is shown");
});

test("shown only once: dismissing keeps the hint and drops the secret", () => {
  const dismissed = marketingAppDismissed(marketingAppGenerated({ ...marketingApp, signing_secret: generatedSecret }));
  assert.deepEqual(dismissed, { kind: "configured", integration: marketingApp });
  assert.ok(!JSON.stringify(dismissed).includes(generatedSecret));
});

test("shown only once: a reload reads only the hint and the update time", () => {
  const state = marketingAppCardState({ status: 200, integrations: [wompi, marketingApp] });
  assert.deepEqual(state, { kind: "configured", integration: marketingApp });
  assert.equal(maskedHint(marketingApp.signing_secret_hint), "••••Q7xk");
});

test("regenerating asks first: a configured secret offers Regenerar and Quitar, and the warning names every channel", () => {
  const state = marketingAppCardState({ status: 200, integrations: [marketingApp] });
  assert.deepEqual(marketingAppActions(state, true), ["regenerate", "remove"]);
  assert.match(MARKETING_APP_REGENERATE_WARNING, /todos los canales/);
});

test("read-only access offers no action in any state", () => {
  for (const state of [
    marketingAppCardState({ status: 200, integrations: [] }),
    marketingAppCardState({ status: 200, integrations: [marketingApp] }),
  ]) {
    assert.deepEqual(marketingAppActions(state, false), []);
  }
});

test("the two cards read their own provider from the same list", () => {
  const load = { status: 200, integrations: [wompi, marketingApp] };
  assert.deepEqual(wompiCardState(load), { kind: "configured", integration: wompi });
  assert.deepEqual(marketingAppCardState(load), { kind: "configured", integration: marketingApp });
  assert.deepEqual(wompiCardState({ status: 200, integrations: [marketingApp] }), { kind: "not_configured" });
});

test("failures are explained in Spanish", () => {
  assert.match(marketingAppErrorMessage(503, "x"), /no están habilitadas/);
  assert.match(marketingAppErrorMessage(403, "x"), /permiso/);
  assert.equal(marketingAppErrorMessage(500, "boom"), "boom");
  assert.equal(marketingAppCardState({ status: 403 }).kind, "error");
});
