import { test } from "node:test";
import assert from "node:assert/strict";
import { OrganizationIntegrationError, organizationIntegrationsApi } from "./organizationIntegrations.ts";

function stubFetch(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  globalThis.fetch = (async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return new Response(status === 204 ? null : JSON.stringify(body), { status });
  }) as typeof fetch;
  return calls;
}

test("generating the marketing-app secret POSTs to the proxy and returns the secret once", async () => {
  const calls = stubFetch(200, { provider: "marketing_app", signing_secret_hint: "Q7xk", signing_secret: "s".repeat(43) });

  const generated = await organizationIntegrationsApi.generateMarketingAppSecret();

  assert.equal(calls[0].url, "/api/organization/integrations/marketing_app/secret");
  assert.equal(calls[0].init?.method, "POST");
  assert.equal(calls[0].init?.body, undefined);
  assert.equal(generated.signing_secret.length, 43);
});

test("generating without an encryption key keeps the backend's code", async () => {
  stubFetch(503, { error: "Integrations are not configured for this deployment", code: "INTEGRATIONS_UNAVAILABLE" });

  await assert.rejects(organizationIntegrationsApi.generateMarketingAppSecret(), (err: unknown) => {
    assert.ok(err instanceof OrganizationIntegrationError);
    assert.equal(err.status, 503);
    assert.equal(err.code, "INTEGRATIONS_UNAVAILABLE");
    return true;
  });
});

test("removing the marketing-app secret reuses the provider delete", async () => {
  const calls = stubFetch(204, null);

  await organizationIntegrationsApi.remove("marketing_app");

  assert.equal(calls[0].url, "/api/organization/integrations/marketing_app");
  assert.equal(calls[0].init?.method, "DELETE");
});
