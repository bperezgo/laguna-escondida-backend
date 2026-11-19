import { test } from "node:test";
import assert from "node:assert/strict";
import { effectivePermissions } from "./edge.ts";

const admin = [
  "products:read",
  "products:create",
  "products:update",
  "products:delete",
  "users:read",
  "users:create",
  "users:update",
  "users:delete",
  "orders:create",
];

test("the cloud keeps every permission", () => {
  assert.deepEqual(effectivePermissions(admin, false), admin);
});

test("the edge drops the catalog and user writes, and keeps the reads", () => {
  assert.deepEqual(effectivePermissions(admin, true), ["products:read", "users:read", "orders:create"]);
});

test("the edge drops both integrations permissions", () => {
  const withIntegrations = [...admin, "integrations:read", "integrations:manage"];
  assert.deepEqual(effectivePermissions(withIntegrations, false), withIntegrations);
  assert.deepEqual(effectivePermissions(withIntegrations, true), ["products:read", "users:read", "orders:create"]);
});
