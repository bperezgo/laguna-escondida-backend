import { test } from "node:test";
import assert from "node:assert/strict";
import type { Organization } from "@/lib/permissions/types";
import {
  brandingCss,
  organizationDisplayName,
  organizationLogoUrl,
  receiptFooterLines,
} from "./organization.ts";

// An organization that has set none of its optional settings.
const bare: Organization = {
  id: "bb0cad3e-be13-4bad-9a37-bba9525bfece",
  slug: "laguna-escondida",
  name: "Laguna Escondida",
  timezone: "America/Bogota",
  country: "CO",
  locale: "es-CO",
  currency: "COP",
  legal_name: null,
  nit: null,
  address: null,
  phone: null,
  ticket_footer: null,
  ticket_legal_notice: null,
  display_name: null,
  logo_url: null,
  primary_color: null,
  secondary_color: null,
};

test("display name falls back to the name", () => {
  assert.equal(organizationDisplayName(bare), "Laguna Escondida");
  assert.equal(organizationDisplayName({ ...bare, display_name: "  " }), "Laguna Escondida");
  assert.equal(organizationDisplayName({ ...bare, display_name: "El Muelle" }), "El Muelle");
});

test("no colors keeps the stylesheet defaults", () => {
  assert.equal(brandingCss(bare), null);
});

test("a color overrides its variable and derived shades, light and dark", () => {
  const css = brandingCss({ ...bare, primary_color: "#1D4ED8" });
  assert.ok(css);
  assert.match(css, /:root \{ --color-primary: #1d4ed8;/);
  // 85% of the color mixed with black, as globals.css steps teal-500 → 600.
  assert.match(css, /--color-primary-hover: #1942b8;/);
  assert.match(css, /--color-primary-light: rgba\(29, 78, 216, 0.12\);/);
  assert.match(css, /\[data-theme='dark'\] \{ --color-primary: #1d4ed8;/);
  assert.doesNotMatch(css, /secondary/);
});

test("Laguna's own colors derive shades close to today's stylesheet", () => {
  const css = brandingCss({ ...bare, primary_color: "#14B8A6" });
  // globals.css: hover #0d9488, -light rgba(20, 184, 166, 0.12), -50 #f0fdfa
  assert.match(css ?? "", /--color-primary-hover: #119c8d;/);
  assert.match(css ?? "", /--color-primary-light: rgba\(20, 184, 166, 0.12\);/);
  assert.match(css ?? "", /--color-primary-50: #f1fbfa;/);
});

test("anything but #RRGGBB never reaches the stylesheet", () => {
  for (const color of ["red", "#fff", "#123456; } body { display: none", "url(x)"]) {
    assert.equal(brandingCss({ ...bare, primary_color: color }), null, color);
  }
});

test("logo must be https", () => {
  assert.equal(organizationLogoUrl(bare), null);
  assert.equal(organizationLogoUrl({ ...bare, logo_url: "javascript:alert(1)" }), null);
  assert.equal(organizationLogoUrl({ ...bare, logo_url: "http://x/logo.png" }), null);
  assert.equal(
    organizationLogoUrl({ ...bare, logo_url: "https://cdn.example/logo.png" }),
    "https://cdn.example/logo.png"
  );
});

test("receipt footer is today's Laguna text without settings", () => {
  const today = ["¡Gracias por su visita!", "Laguna Escondida"];
  assert.deepEqual(receiptFooterLines(null), today);
  assert.deepEqual(receiptFooterLines(bare), today);
});

test("receipt footer carries the organization's receipt settings", () => {
  assert.deepEqual(
    receiptFooterLines({
      ...bare,
      display_name: "El Muelle",
      nit: "901234567-8",
      address: "Cra 7 # 12-34\nBogotá",
      ticket_footer: "Vuelva pronto",
      ticket_legal_notice: "Régimen común",
    }),
    ["Vuelva pronto", "El Muelle", "NIT: 901234567-8", "Cra 7 # 12-34", "Bogotá", "Régimen común"]
  );
});
