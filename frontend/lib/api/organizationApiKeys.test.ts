import { test } from "node:test";
import assert from "node:assert/strict";
import { OrganizationApiKeyError, organizationApiKeysApi } from "./organizationApiKeys.ts";

function stubFetch(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  globalThis.fetch = (async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return new Response(status === 204 ? null : JSON.stringify(body), { status });
  }) as typeof fetch;
  return calls;
}

test("listing GETs the proxy", async () => {
  const calls = stubFetch(200, { api_keys: [{ id: "k1", name: "console-local", prefix: "AbCdEfGh" }] });

  const list = await organizationApiKeysApi.list();

  assert.equal(calls[0].url, "/api/organization/api-keys");
  assert.equal(calls[0].init?.method, undefined);
  assert.equal(list.api_keys[0].prefix, "AbCdEfGh");
});

test("issuing POSTs only the name and returns the plaintext", async () => {
  const calls = stubFetch(201, { id: "k1", name: "console-local", key: "lgk_" + "x".repeat(43) });

  const issued = await organizationApiKeysApi.issue("console-local");

  assert.equal(calls[0].url, "/api/organization/api-keys");
  assert.equal(calls[0].init?.method, "POST");
  assert.deepEqual(JSON.parse(String(calls[0].init?.body)), { name: "console-local" });
  assert.ok(issued.key.startsWith("lgk_"));
});

test("a refused name keeps the backend's field", async () => {
  stubFetch(400, { error: "api key name is required", field: "name" });

  await assert.rejects(organizationApiKeysApi.issue(" "), (err: unknown) => {
    assert.ok(err instanceof OrganizationApiKeyError);
    assert.equal(err.status, 400);
    assert.equal(err.field, "name");
    assert.equal(err.message, "api key name is required");
    return true;
  });
});

test("a manager's 403 reads as a permission error", async () => {
  stubFetch(403, { error: "Insufficient permissions", required: "api_keys:manage" });

  await assert.rejects(organizationApiKeysApi.list(), (err: unknown) => {
    assert.ok(err instanceof OrganizationApiKeyError);
    assert.equal(err.status, 403);
    assert.match(err.message, /permiso/);
    return true;
  });
});

test("revoking DELETEs the key by id", async () => {
  const calls = stubFetch(204, null);

  await organizationApiKeysApi.revoke("019a2f61-0c88-7e12-b4a9-3d5c7e9f2b10");

  assert.equal(calls[0].url, "/api/organization/api-keys/019a2f61-0c88-7e12-b4a9-3d5c7e9f2b10");
  assert.equal(calls[0].init?.method, "DELETE");
});

test("revoking another organization's key surfaces the 404", async () => {
  stubFetch(404, { error: "API key not found" });

  await assert.rejects(organizationApiKeysApi.revoke("k2"), (err: unknown) => {
    assert.ok(err instanceof OrganizationApiKeyError);
    assert.equal(err.status, 404);
    return true;
  });
});
