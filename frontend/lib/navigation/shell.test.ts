import { test } from "node:test";
import assert from "node:assert/strict";
import type { NavContext } from "../home/navItems.ts";
import {
  boxNavigation,
  cloudEntries,
  cloudNavigation,
  currentBoxTab,
  currentEntry,
  firstScreen,
} from "./shell.ts";

const everything = () => true;
const only = (...granted: string[]) => (p: string) => granted.includes(p);
const hrefs = (entries: { href: string }[]) => entries.map((e) => e.href);

const cloud = (over: Partial<NavContext> = {}): NavContext => ({
  hasPermission: everything,
  plan: "full",
  isEdge: false,
  isAdmin: true,
  ...over,
});
const box = (over: Partial<NavContext> = {}): NavContext => cloud({ isEdge: true, isAdmin: false, ...over });

const SERVER = only("orders:read", "orders:create", "orders:update", "products:read", "commands:read", "commands:update");
const COOK = only("orders:read", "orders:complete-product", "products:read", "commands:read", "commands:update");
const MANAGER = (p: string) => p !== "financial:read" && p !== "edge:enroll" && p !== "integrations:read";
const ACCOUNTANT = only("invoices:read", "suppliers:read", "purchase-entries:read", "expenses:read", "ordering:read");

test("a full-plan admin gets Inicio and the six groups in order", () => {
  const nav = cloudNavigation(cloud());
  assert.equal(nav.home?.href, "/home");
  assert.deepEqual(
    nav.groups.map((g) => [g.title, hrefs(g.entries)]),
    [
      ["Vender", ["/orders", "/kitchen", "/deliveries", "/my-deliveries", "/reservations"]],
      ["Menú", ["/products"]],
      ["Inventario", ["/stock", "/suppliers", "/purchase-entries"]],
      ["Finanzas", ["/financial", "/close", "/expenses", "/invoices", "/support-documents"]],
      ["Clientes", ["/feedback"]],
      ["Ajustes", ["/users", "/ajustes/integraciones", "/ajustes/notificaciones", "/ajustes/instalar-caja"]],
    ]
  );
});

test("a starter admin gets no customer channels, no e-invoicing and no Clientes group", () => {
  const nav = cloudNavigation(cloud({ plan: "starter" }));
  const shown = hrefs(cloudEntries(nav));
  for (const href of ["/deliveries", "/reservations", "/feedback", "/invoices", "/support-documents"]) {
    assert.ok(!shown.includes(href), href);
  }
  assert.ok(!nav.groups.some((g) => g.id === "clientes"));
  assert.deepEqual(hrefs(nav.groups.at(-1)!.entries), ["/users", "/ajustes/instalar-caja"]);
});

test("a manager in the cloud gets no Órdenes, Cocina or Inicio", () => {
  const nav = cloudNavigation(cloud({ hasPermission: MANAGER, isAdmin: false }));
  const shown = hrefs(cloudEntries(nav));
  assert.ok(!shown.includes("/orders"));
  assert.ok(!shown.includes("/kitchen"));
  assert.equal(nav.home, null);
});

test("a courier gets only Vender with Mis entregas", () => {
  const nav = cloudNavigation(cloud({ hasPermission: only("deliveries:carry"), isAdmin: false }));
  assert.equal(nav.home, null);
  assert.deepEqual(
    nav.groups.map((g) => [g.id, hrefs(g.entries)]),
    [["vender", ["/my-deliveries"]]]
  );
});

test("the box shows tabs, and Más only when it holds something", () => {
  const server = boxNavigation(box({ hasPermission: SERVER }));
  assert.deepEqual(hrefs(server.tabs), ["/orders", "/kitchen"]);
  assert.deepEqual(hrefs(server.more), ["/products"]);

  const manager = boxNavigation(box({ hasPermission: MANAGER }));
  assert.deepEqual(hrefs(manager.tabs), ["/orders", "/kitchen"]);
  assert.deepEqual(hrefs(manager.more), ["/products", "/stock", "/users"]);

  const floorOnly = boxNavigation(box({ hasPermission: only("orders:read", "commands:read") }));
  assert.deepEqual(floorOnly.more, []);
});

test("the box ignores the plan", () => {
  assert.deepEqual(boxNavigation(box({ plan: "starter" })), boxNavigation(box({ plan: "full" })));
});

test("the current entry is the longest matching prefix", () => {
  const entries = cloudEntries(cloudNavigation(cloud()));
  assert.equal(currentEntry("/delivery-zones", entries)?.href, "/deliveries");
  assert.equal(currentEntry("/reservations/ajustes", entries)?.href, "/reservations");
  assert.equal(currentEntry("/expenses", entries)?.href, "/expenses");
  assert.equal(currentEntry("/ajustes/instalar-caja", entries)?.href, "/ajustes/instalar-caja");
  assert.equal(currentEntry("/home", entries)?.href, "/home");
  assert.equal(currentEntry("/ajustes", entries), null);
});

test("a screen under Más marks Más on the box", () => {
  const nav = boxNavigation(box({ hasPermission: MANAGER }));
  assert.equal(currentBoxTab("/stock", nav), "more");
  assert.equal(currentBoxTab("/orders", nav), "/orders");
  assert.equal(currentBoxTab("/kitchen", nav), "/kitchen");
  assert.equal(currentBoxTab("/home", nav), null);
});

test("each user starts on their own first screen", () => {
  assert.equal(firstScreen(cloud()), "/home");
  assert.equal(firstScreen(cloud({ hasPermission: only("deliveries:carry"), isAdmin: false })), "/my-deliveries");
  // An accountant reads the ordering hours, which live under Domicilios.
  assert.equal(firstScreen(cloud({ hasPermission: ACCOUNTANT, isAdmin: false })), "/deliveries");
  assert.equal(firstScreen(cloud({ hasPermission: ACCOUNTANT, isAdmin: false, plan: "starter" })), "/suppliers");
  assert.equal(firstScreen(cloud({ hasPermission: MANAGER, isAdmin: false })), "/deliveries");
  assert.equal(firstScreen(box({ hasPermission: SERVER })), "/orders");
  assert.equal(firstScreen(box({ hasPermission: COOK })), "/kitchen");
  assert.equal(firstScreen(box({ hasPermission: only("products:read") })), "/products");
  assert.equal(firstScreen(cloud({ hasPermission: () => false, isAdmin: false })), null);
  assert.equal(firstScreen(box({ hasPermission: () => false })), null);
});
