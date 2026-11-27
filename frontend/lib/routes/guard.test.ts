import { test } from "node:test";
import assert from "node:assert/strict";
import { guardRedirect } from "./guard.ts";

const sections = [
  "/home",
  "/orders",
  "/products",
  "/stock",
  "/invoices",
  "/users",
  "/delivery-zones",
  "/deliveries",
  "/kitchen",
  "/suppliers",
  "/purchase-entries",
  "/expenses",
  "/close",
  "/financial",
  "/support-documents",
];

test("every section sends a signed-out visitor to sign-in", () => {
  for (const path of sections) {
    assert.equal(
      guardRedirect(path, false),
      `/signin?redirect=${encodeURIComponent(path)}`,
      path
    );
  }
});

test("nested and unknown routes are protected too", () => {
  assert.equal(
    guardRedirect("/orders/123", false),
    "/signin?redirect=%2Forders%2F123"
  );
  assert.equal(
    guardRedirect("/some-future-section", false),
    "/signin?redirect=%2Fsome-future-section"
  );
  // A prefix of a public route is not public.
  assert.equal(
    guardRedirect("/signinx", false),
    "/signin?redirect=%2Fsigninx"
  );
});

test("a signed-in user passes through every section", () => {
  for (const path of sections) {
    assert.equal(guardRedirect(path, true), null, path);
  }
});

test("root redirects by session", () => {
  assert.equal(guardRedirect("/", true), "/home");
  assert.equal(guardRedirect("/", false), "/signin");
});

test("sign-in is public, and skipped when already signed in", () => {
  assert.equal(guardRedirect("/signin", false), null);
  assert.equal(guardRedirect("/signin", true), "/home");
});

test("signup is public, and skipped when already signed in", () => {
  assert.equal(guardRedirect("/registro", false), null);
  assert.equal(guardRedirect("/registro", true), "/home");
});

test("an invitation link opens with or without a session", () => {
  assert.equal(guardRedirect("/invitacion/q3L0Zx", false), null);
  assert.equal(guardRedirect("/invitacion/q3L0Zx", true), null);
  assert.equal(guardRedirect("/invitacionx", false), "/signin?redirect=%2Finvitacionx");
});

test("installing the box needs a session", () => {
  assert.equal(guardRedirect("/ajustes/instalar-caja", false), "/signin?redirect=%2Fajustes%2Finstalar-caja");
});

test("the signup steps before the trial are public, and the last one needs the session it created", () => {
  assert.equal(guardRedirect("/registro/pago", false), null);
  assert.equal(guardRedirect("/registro/pago", true), "/home");
  assert.equal(guardRedirect("/registro/listo", true), null);
  assert.equal(guardRedirect("/registro/listo", false), "/signin");
});

test("the recovery page is reached without a session, and a signed-in user is sent home from it", () => {
  assert.equal(guardRedirect("/recuperar", false), null);
  assert.equal(guardRedirect("/recuperar", true), "/home");
});
