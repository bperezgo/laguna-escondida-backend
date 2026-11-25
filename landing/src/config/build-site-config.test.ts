import assert from "node:assert/strict";
import { test } from "node:test";

import { buildSiteConfig, installerDownloadUrl, whatsappLink } from "./build-site-config.ts";

const base = { PUBLIC_API_BASE_URL: "https://api.example.com/" };

test("a build without the backend address fails with a clear message", () => {
  assert.throws(() => buildSiteConfig({}), /PUBLIC_API_BASE_URL is not set/);
  assert.throws(() => buildSiteConfig({ PUBLIC_API_BASE_URL: "  " }), /PUBLIC_API_BASE_URL is not set/);
});

test("a build with only the backend address works, with every optional tool off", () => {
  const config = buildSiteConfig(base);
  assert.equal(config.apiBaseUrl, "https://api.example.com");
  assert.equal(config.productName, "Kuenta");
  assert.equal(config.siteUrl, "https://kuenta.com");
  assert.equal(config.gaMeasurementId, undefined);
  assert.equal(config.metaPixelId, undefined);
  assert.equal(config.whatsappNumber, undefined);
  assert.equal(config.legalName, undefined);
  assert.equal(config.privacyEmail, undefined);
});

test("the privacy contact is passed through and a malformed email fails the build", () => {
  const config = buildSiteConfig({ ...base, PUBLIC_LEGAL_NAME: "Empresa S.A.S.", PUBLIC_PRIVACY_EMAIL: "datos@kuenta.com" });
  assert.equal(config.legalName, "Empresa S.A.S.");
  assert.equal(config.privacyEmail, "datos@kuenta.com");
  assert.throws(() => buildSiteConfig({ ...base, PUBLIC_PRIVACY_EMAIL: "datos" }), /PUBLIC_PRIVACY_EMAIL is malformed/);
});

test("blank optional values count as unset", () => {
  const config = buildSiteConfig({ ...base, PUBLIC_GA_MEASUREMENT_ID: "", PUBLIC_META_PIXEL_ID: " ", PUBLIC_WHATSAPP_NUMBER: "" });
  assert.equal(config.gaMeasurementId, undefined);
  assert.equal(config.metaPixelId, undefined);
  assert.equal(config.whatsappNumber, undefined);
});

test("configured ids are passed through", () => {
  const config = buildSiteConfig({
    ...base,
    PUBLIC_GA_MEASUREMENT_ID: "G-ABC123XYZ9",
    PUBLIC_META_PIXEL_ID: "123456789012345",
    PUBLIC_SITE_URL: "https://staging.kuenta.com/",
  });
  assert.equal(config.gaMeasurementId, "G-ABC123XYZ9");
  assert.equal(config.metaPixelId, "123456789012345");
  assert.equal(config.siteUrl, "https://staging.kuenta.com");
});

test("a malformed value fails the build instead of silently breaking tracking", () => {
  assert.throws(() => buildSiteConfig({ ...base, PUBLIC_GA_MEASUREMENT_ID: "UA-1" }), /PUBLIC_GA_MEASUREMENT_ID is malformed/);
  assert.throws(() => buildSiteConfig({ ...base, PUBLIC_META_PIXEL_ID: "abc" }), /PUBLIC_META_PIXEL_ID is malformed/);
  assert.throws(() => buildSiteConfig({ PUBLIC_API_BASE_URL: "api.example.com" }), /absolute http\(s\) URL/);
  assert.throws(() => buildSiteConfig({ PUBLIC_API_BASE_URL: "javascript:alert(1)" }), /absolute http\(s\) URL/);
});

test("the WhatsApp number is cleaned and builds a prefilled link", () => {
  const config = buildSiteConfig({ ...base, PUBLIC_WHATSAPP_NUMBER: "+57 300 123 4567" });
  assert.equal(config.whatsappNumber, "573001234567");
  assert.equal(whatsappLink(config, "Hola, quiero una demo."), "https://wa.me/573001234567?text=Hola%2C%20quiero%20una%20demo.");
});

test("without a number there is no WhatsApp link", () => {
  assert.equal(whatsappLink(buildSiteConfig(base), "Hola"), undefined);
});

test("a malformed number fails the build", () => {
  assert.throws(() => buildSiteConfig({ ...base, PUBLIC_WHATSAPP_NUMBER: "12" }), /PUBLIC_WHATSAPP_NUMBER is malformed/);
});

test("the installer link is the configured backend's download endpoint, with or without a trailing slash", () => {
  for (const api of ["https://api.example.com", "https://api.example.com/", "https://api.example.com//"]) {
    assert.equal(installerDownloadUrl(buildSiteConfig({ PUBLIC_API_BASE_URL: api })), "https://api.example.com/api/public/edge/installer");
  }
  assert.equal(
    installerDownloadUrl(buildSiteConfig({ PUBLIC_API_BASE_URL: "https://staging-api.example.com/" })),
    "https://staging-api.example.com/api/public/edge/installer",
  );
});
