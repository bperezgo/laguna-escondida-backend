import { test } from "node:test";
import assert from "node:assert/strict";
import { forwardCatalogImport } from "./proxy.ts";

function recordingFetch(status: number, body: string, contentType = "application/json") {
  const calls: { url: string; init: RequestInit }[] = [];
  const fetchFn = async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return new Response(body, { status, headers: { "Content-Type": contentType } });
  };
  return { calls, fetchFn };
}

test("the body and the token are forwarded unchanged", async () => {
  const { calls, fetchFn } = recordingFetch(200, '{"valid":true}');
  const body = '{"sellables":[{"ref":"a","name":"Coca"}]}';
  const got = await forwardCatalogImport(fetchFn, "http://api/catalog/import/validate", "tok", body);

  assert.equal(calls.length, 1);
  assert.equal(calls[0].url, "http://api/catalog/import/validate");
  assert.equal(calls[0].init.method, "POST");
  assert.equal(calls[0].init.body, body);
  assert.equal((calls[0].init.headers as Record<string, string>).Authorization, "Bearer tok");
  assert.deepEqual(got, { status: 200, body: '{"valid":true}', contentType: "application/json" });
});

test("no token means no Authorization header", async () => {
  const { calls, fetchFn } = recordingFetch(401, '{"error":"unauthorized"}');
  await forwardCatalogImport(fetchFn, "http://api/catalog/import", undefined, "{}");
  assert.equal("Authorization" in (calls[0].init.headers as Record<string, string>), false);
});

test("a 413 that is not JSON comes back with its status and body", async () => {
  const { fetchFn } = recordingFetch(413, "http: request body too large", "text/plain");
  const got = await forwardCatalogImport(fetchFn, "http://api/catalog/import", "tok", "{}");
  assert.deepEqual(got, { status: 413, body: "http: request body too large", contentType: "text/plain" });
});

test("a 403 comes back unchanged", async () => {
  const { fetchFn } = recordingFetch(403, '{"error":"forbidden"}');
  const got = await forwardCatalogImport(fetchFn, "http://api/catalog/import/validate", "tok", "{}");
  assert.equal(got.status, 403);
  assert.equal(got.body, '{"error":"forbidden"}');
});
