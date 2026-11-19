import { test } from "node:test";
import assert from "node:assert/strict";
import type { OrganizationIntegration } from "../../types/organizationIntegrations.ts";
import {
  environmentLabel,
  hasWompiErrors,
  maskedHint,
  validateWompiDraft,
  wompiCardState,
  wompiSaveErrors,
} from "./integrations.ts";

const wompi: OrganizationIntegration = {
  provider: "wompi",
  environment: "sandbox",
  private_key_hint: "wxyz",
  events_secret_hint: "9876",
  webhook_path: "/api/webhooks/wompi/019a1c2e-5f00-7a4b-8c3d-2e1f0a9b8c7d",
  created_at: "2026-11-19T15:04:05Z",
  updated_at: "2026-11-19T15:04:05Z",
};

test("a server without an encryption key is unavailable", () => {
  assert.deepEqual(wompiCardState({ status: 503, code: "INTEGRATIONS_UNAVAILABLE" }), { kind: "unavailable" });
});

test("no wompi integration is not configured", () => {
  assert.deepEqual(wompiCardState({ status: 200, integrations: [] }), { kind: "not_configured" });
});

test("a wompi integration is configured", () => {
  assert.deepEqual(wompiCardState({ status: 200, integrations: [wompi] }), { kind: "configured", integration: wompi });
});

test("other load failures are errors", () => {
  assert.equal(wompiCardState({ status: 403 }).kind, "error");
  assert.equal(wompiCardState({ status: 404 }).kind, "error");
  assert.equal(wompiCardState({ status: 500 }).kind, "error");
  assert.equal(wompiCardState({ status: 503 }).kind, "error");
});

test("empty fields are refused before sending", () => {
  const errors = validateWompiDraft({ private_key: " ", events_secret: "" });
  assert.ok(errors.private_key);
  assert.ok(errors.events_secret);
  assert.equal(hasWompiErrors(errors), true);
  assert.equal(hasWompiErrors(validateWompiDraft({ private_key: "prv_test_a", events_secret: "test_events_b" })), false);
});

test("a refused field is shown under that field", () => {
  const errors = wompiSaveErrors(400, "invalid private_key: must start with prv_test_ or prv_prod_", "private_key");
  assert.deepEqual(Object.keys(errors), ["private_key"]);
  assert.match(errors.private_key ?? "", /prv_test_ o prv_prod_/);

  const missing = wompiSaveErrors(400, "invalid events_secret: is required", "events_secret");
  assert.deepEqual(Object.keys(missing), ["events_secret"]);
  assert.match(missing.events_secret ?? "", /obligatorio/);
});

test("production keys on the sandbox are the form's message", () => {
  const errors = wompiSaveErrors(400, "the keys are for production and this cloud uses Wompi's sandbox");
  assert.deepEqual(Object.keys(errors), ["form"]);
  assert.match(errors.form ?? "", /producción/);
  assert.match(errors.form ?? "", /sandbox/);
});

test("mixed environments are the form's message", () => {
  const errors = wompiSaveErrors(400, "the private key is for sandbox and the events secret for production");
  assert.deepEqual(Object.keys(errors), ["form"]);
  assert.match(errors.form ?? "", /mismo ambiente/);
});

test("other failures are the form's message", () => {
  assert.deepEqual(Object.keys(wompiSaveErrors(503, "x")), ["form"]);
  assert.deepEqual(Object.keys(wompiSaveErrors(403, "x")), ["form"]);
  assert.deepEqual(wompiSaveErrors(500, "boom"), { form: "boom" });
});

test("hints and environments are shown masked and in Spanish", () => {
  assert.equal(maskedHint("wxyz"), "••••wxyz");
  assert.equal(maskedHint(""), "••••");
  assert.equal(environmentLabel("sandbox"), "Sandbox");
  assert.equal(environmentLabel("production"), "Producción");
});
