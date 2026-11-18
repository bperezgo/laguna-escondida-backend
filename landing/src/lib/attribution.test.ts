import assert from "node:assert/strict";
import { test } from "node:test";

import { capture, parseAttribution, read, type StorageLike } from "./attribution.ts";

function memoryStorage(): StorageLike & { data: Map<string, string> } {
  const data = new Map<string, string>();
  return { data, getItem: (k) => data.get(k) ?? null, setItem: (k, v) => void data.set(k, v) };
}

const refusingStorage: StorageLike = {
  getItem() {
    throw new Error("SecurityError");
  },
  setItem() {
    throw new Error("SecurityError");
  },
};

const ad = {
  url: "https://kuenta.com/?utm_source=facebook&utm_medium=paid&utm_campaign=lanzamiento&utm_term=pos&utm_content=video1&fbclid=abc&gclid=xyz",
  referrer: "https://l.facebook.com/",
};

test("all seven parameters, the landing URL and the referrer are read", () => {
  assert.deepEqual(parseAttribution(ad), {
    utm_source: "facebook",
    utm_medium: "paid",
    utm_campaign: "lanzamiento",
    utm_term: "pos",
    utm_content: "video1",
    fbclid: "abc",
    gclid: "xyz",
    landing_url: ad.url,
    referrer: "https://l.facebook.com/",
  });
});

test("absent and blank parameters are left out, unknown ones ignored", () => {
  const got = parseAttribution({ url: "https://kuenta.com/?utm_source=%20&foo=bar&utm_campaign=x", referrer: "" });
  assert.deepEqual(got, { utm_campaign: "x", landing_url: "https://kuenta.com/?utm_source=%20&foo=bar&utm_campaign=x", referrer: "" });
});

test("a direct visit still records the landing URL", () => {
  assert.deepEqual(parseAttribution({ url: "https://kuenta.com/", referrer: "" }), { landing_url: "https://kuenta.com/", referrer: "" });
});

test("an unparseable URL does not throw", () => {
  assert.deepEqual(parseAttribution({ url: "not a url", referrer: "" }), { landing_url: "not a url", referrer: "" });
});

test("first touch is not overwritten by a later page of the session", () => {
  const storage = memoryStorage();
  capture(ad, storage);
  capture({ url: "https://kuenta.com/privacidad", referrer: "https://kuenta.com/" }, storage);

  const got = read({ url: "https://kuenta.com/#demo", referrer: "" }, storage);

  assert.equal(got.utm_source, "facebook");
  assert.equal(got.fbclid, "abc");
  assert.equal(got.landing_url, ad.url);
});

test("arriving from an ad, reading the policy, then converting keeps the ad attribution", () => {
  const storage = memoryStorage();
  capture(ad, storage);
  capture({ url: "https://kuenta.com/privacidad", referrer: ad.url }, storage);
  capture({ url: "https://kuenta.com/", referrer: "https://kuenta.com/privacidad" }, storage);

  const got = read({ url: "https://kuenta.com/", referrer: "https://kuenta.com/privacidad" }, storage);

  assert.equal(got.utm_campaign, "lanzamiento");
  assert.equal(got.landing_url, ad.url);
});

test("a direct first page is the first touch too: a later ad parameter does not replace it", () => {
  const storage = memoryStorage();
  capture({ url: "https://kuenta.com/", referrer: "" }, storage);
  capture(ad, storage);

  assert.equal(read(ad, storage).utm_source, undefined);
});

test("storage that refuses falls back to the current page and never throws", () => {
  capture(ad, refusingStorage);

  const got = read(ad, refusingStorage);

  assert.equal(got.utm_source, "facebook");
  assert.equal(got.landing_url, ad.url);
});

test("no storage at all behaves like refusing storage", () => {
  capture(ad, undefined);
  assert.equal(read(ad, undefined).gclid, "xyz");
});

test("a corrupt stored value is ignored", () => {
  const storage = memoryStorage();
  storage.data.set("kuenta_attribution", "{not json");

  assert.equal(read(ad, storage).utm_source, "facebook");

  storage.data.set("kuenta_attribution", JSON.stringify({ utm_source: 5 }));
  assert.equal(read(ad, storage).utm_source, "facebook");
});
