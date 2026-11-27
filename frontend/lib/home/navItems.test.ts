import { test } from "node:test";
import assert from "node:assert/strict";
import { NAV_ITEMS, visibleNavItems } from "./navItems.ts";
import { visibleSettingsSections } from "../settings/hub.ts";
import { showApiKeysCard } from "../settings/apiKeys.ts";

const everything = () => true;
const hrefs = (items: { href: string }[]) => items.map((i) => i.href);
const gated = ["/invoices", "/support-documents", "/reservations", "/feedback", "/deliveries", "/my-deliveries"];

test("a starter admin sees no invoices, support documents, reservations, feedback or deliveries", () => {
  const shown = hrefs(visibleNavItems({ hasPermission: everything, plan: "starter", isEdge: false, isAdmin: true }));
  for (const href of gated) assert.ok(!shown.includes(href), href);
  for (const href of ["/orders", "/products", "/stock", "/users", "/ajustes"]) assert.ok(shown.includes(href), href);
});

test("a starter admin's settings hold only the box installation and the subscription, with no API keys", () => {
  const ctx = { hasPermission: everything, plan: "starter", isEdge: false };
  assert.deepEqual(hrefs(visibleSettingsSections(ctx)), ["/ajustes/instalar-caja", "/ajustes/suscripcion"]);
  assert.equal(showApiKeysCard(everything, "starter", false), false);
});

test("einvoicing adds invoices and support documents", () => {
  const shown = hrefs(visibleNavItems({ hasPermission: everything, plan: "einvoicing", isEdge: false, isAdmin: true }));
  assert.ok(shown.includes("/invoices"));
  assert.ok(shown.includes("/support-documents"));
  assert.ok(!shown.includes("/reservations"));
  assert.equal(showApiKeysCard(everything, "einvoicing", false), false);
});

test("a full admin sees every entry, every settings section and the API keys card", () => {
  const shown = hrefs(visibleNavItems({ hasPermission: everything, plan: "full", isEdge: false, isAdmin: true }));
  assert.deepEqual(shown, hrefs([...NAV_ITEMS]));
  assert.deepEqual(hrefs(visibleSettingsSections({ hasPermission: everything, plan: "full", isEdge: false })), [
    "/ajustes/integraciones",
    "/ajustes/notificaciones",
    "/ajustes/instalar-caja",
    "/ajustes/suscripcion",
  ]);
  assert.equal(showApiKeysCard(everything, "full", false), true);
});

test("the edge ignores the plan", () => {
  const shown = hrefs(visibleNavItems({ hasPermission: everything, plan: "starter", isEdge: true, isAdmin: true }));
  assert.ok(shown.includes("/products"));
  assert.ok(shown.includes("/kitchen"));
  assert.equal(showApiKeysCard(everything, "starter", true), true);
});

test("permissions still gate each entry", () => {
  const shown = hrefs(
    visibleNavItems({ hasPermission: (p) => p === "products:read", plan: "full", isEdge: false, isAdmin: false })
  );
  assert.deepEqual(shown, ["/products"]);
});

test("a manager on starter with only notification access gets no settings card", () => {
  const shown = hrefs(
    visibleNavItems({
      hasPermission: (p) => p === "notification-channels:read",
      plan: "starter",
      isEdge: false,
      isAdmin: false,
    })
  );
  assert.ok(!shown.includes("/ajustes"));
});

test("the subscription section is for whoever manages billing, on every plan and never on a box", () => {
  const admin = (p: string) => p === "billing:manage";
  for (const plan of ["starter", "einvoicing", "full"]) {
    assert.deepEqual(hrefs(visibleSettingsSections({ hasPermission: admin, plan, isEdge: false })), ["/ajustes/suscripcion"]);
  }
  const manager = (p: string) => p !== "billing:manage";
  assert.ok(!hrefs(visibleSettingsSections({ hasPermission: manager, plan: "full", isEdge: false })).includes("/ajustes/suscripcion"));
  assert.ok(!hrefs(visibleNavItems({ hasPermission: everything, plan: "full", isEdge: true, isAdmin: true })).includes("/ajustes"));
});
