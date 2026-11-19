import assert from "node:assert/strict";
import { test } from "node:test";

import { buildRequest, mapResponse, submitLead, validate, type FormValues } from "./lead-form.ts";

const filled: FormValues = { name: " Ana Ruiz ", restaurant_name: "El Fogón", phone: "300 123 4567", consent: true, website: "" };
const attribution = { utm_source: "facebook", fbclid: "abc", landing_url: "https://kuenta.com/?utm_source=facebook", referrer: "" };

test("a filled form with consent has no errors", () => {
  assert.deepEqual(validate(filled), {});
});

test("each missing field is named", () => {
  assert.deepEqual(Object.keys(validate({ ...filled, name: " " })), ["name"]);
  assert.deepEqual(Object.keys(validate({ ...filled, restaurant_name: "" })), ["restaurant_name"]);
  assert.deepEqual(Object.keys(validate({ ...filled, phone: "12" })), ["phone"]);
});

test("without consent the form asks for consent and nothing else", () => {
  const errors = validate({ ...filled, consent: false });
  assert.deepEqual(Object.keys(errors), ["consent"]);
  assert.match(errors.consent ?? "", /política de privacidad/);
});

test("the request carries the trimmed fields, consent, honeypot, event id and attribution", () => {
  assert.deepEqual(buildRequest(filled, attribution, "0b9c1c3e-6a1f-4c1e"), {
    name: "Ana Ruiz",
    restaurant_name: "El Fogón",
    phone: "300 123 4567",
    consent: true,
    website: "",
    event_id: "0b9c1c3e-6a1f-4c1e",
    utm_source: "facebook",
    fbclid: "abc",
    landing_url: "https://kuenta.com/?utm_source=facebook",
    referrer: "",
  });
});

test("201 is success", () => {
  assert.deepEqual(mapResponse(201, { status: "received" }), { kind: "success" });
});

test("a 400 naming a field points at that field", () => {
  const result = mapResponse(400, { error: "invalid phone", field: "phone" });
  assert.equal(result.kind, "field");
  assert.equal(result.kind === "field" && result.field, "phone");
});

test("a 400 naming an unknown field, 429, 413 and 500 are the general error", () => {
  assert.deepEqual(mapResponse(400, { field: "utm_source" }), { kind: "error" });
  assert.deepEqual(mapResponse(400, undefined), { kind: "error" });
  for (const status of [429, 413, 500, 404]) assert.deepEqual(mapResponse(status, {}), { kind: "error" });
});

test("submitLead posts JSON to the backend and maps the answer", async () => {
  let seen: { url: string; init: RequestInit } | undefined;
  const fakeFetch = (async (url: string, init: RequestInit) => {
    seen = { url, init };
    return new Response(JSON.stringify({ status: "received" }), { status: 201 });
  }) as unknown as typeof fetch;

  const result = await submitLead("https://api.example.com", buildRequest(filled, attribution, "evt-12345678"), fakeFetch);

  assert.deepEqual(result, { kind: "success" });
  assert.equal(seen?.url, "https://api.example.com/api/public/leads");
  assert.equal(seen?.init.method, "POST");
  assert.equal(JSON.parse(String(seen?.init.body)).event_id, "evt-12345678");
});

test("a network failure is the general error and does not throw", async () => {
  const failing = (async () => {
    throw new TypeError("fetch failed");
  }) as unknown as typeof fetch;

  assert.deepEqual(await submitLead("https://api.example.com", buildRequest(filled, attribution, "evt-12345678"), failing), { kind: "error" });
});

test("a non JSON answer is handled", async () => {
  const html = (async () => new Response("<html>bad gateway</html>", { status: 502 })) as unknown as typeof fetch;

  assert.deepEqual(await submitLead("https://api.example.com", buildRequest(filled, attribution, "evt-12345678"), html), { kind: "error" });
});
