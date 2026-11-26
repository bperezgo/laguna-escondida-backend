import { test } from "node:test";
import assert from "node:assert/strict";
import type { IssuedOrganizationApiKey, OrganizationApiKey } from "../../types/organizationApiKeys.ts";
import {
  apiKeyErrorMessage,
  apiKeyIssued,
  apiKeyRevoked,
  apiKeysCardState,
  canRevokeApiKey,
  mcpPublicUrl,
  showApiKeysCard,
  validateApiKeyName,
} from "./apiKeys.ts";

const operatorKey: OrganizationApiKey = {
  id: "k-old",
  organization_id: "org",
  user_id: "agent",
  name: "whatsapp-agent",
  prefix: "AbCdEfGh",
  created_at: "2026-10-26T15:04:05Z",
  last_used_at: "2026-11-20T10:00:00Z",
  revoked_at: null,
};

const issued: IssuedOrganizationApiKey = {
  ...operatorKey,
  id: "k-new",
  user_id: "key-user",
  name: "console-local",
  prefix: "Xk9vQ2mL",
  last_used_at: null,
  key: "lgk_Xk9vQ2mL" + "x".repeat(35),
};

test("a loaded list becomes the card's keys", () => {
  assert.deepEqual(apiKeysCardState({ status: 200, keys: [operatorKey] }), { kind: "loaded", keys: [operatorKey] });
});

test("load failures read as messages", () => {
  assert.equal(apiKeysCardState({ status: 403 }).kind, "error");
  assert.equal(apiKeysCardState({ status: 500 }).kind, "error");
  assert.equal(apiKeysCardState({ status: 200 }).kind, "error", "a body without keys");
});

test("issuing shows the plaintext with the MCP URL and lists the key without it", () => {
  const { keys, shown } = apiKeyIssued([operatorKey], issued, "https://mcp.example.com/mcp");

  assert.deepEqual(shown, { name: "console-local", key: issued.key, mcpUrl: "https://mcp.example.com/mcp" });
  assert.equal(keys[0].id, "k-new", "newest first");
  assert.equal(keys.length, 2);
  assert.ok(!("key" in keys[0]), "the list never holds the plaintext");
  assert.ok(!JSON.stringify(keys).includes(issued.key));
});

test("issuing without an MCP URL shows the key alone", () => {
  assert.equal(apiKeyIssued([], issued, null).shown.mcpUrl, null);
});

test("the MCP URL is shown only when configured", () => {
  assert.equal(mcpPublicUrl(undefined), null);
  assert.equal(mcpPublicUrl("   "), null);
  assert.equal(mcpPublicUrl(" https://mcp.example.com/mcp "), "https://mcp.example.com/mcp");
});

test("a revoked key shows its revocation and offers no revoke", () => {
  assert.ok(canRevokeApiKey(operatorKey));

  const keys = apiKeyRevoked([operatorKey, issued], "k-old", "2026-11-21T12:00:00Z");

  assert.equal(keys[0].revoked_at, "2026-11-21T12:00:00Z");
  assert.ok(!canRevokeApiKey(keys[0]));
  assert.equal(keys[1].revoked_at, null, "other keys are untouched");
  assert.equal(apiKeyRevoked(keys, "k-old", "later")[0].revoked_at, "2026-11-21T12:00:00Z", "keeps the first time");
});

test("the name is required and at most 255 characters", () => {
  assert.notEqual(validateApiKeyName("   "), "");
  assert.notEqual(validateApiKeyName("ñ".repeat(256)), "");
  assert.equal(validateApiKeyName("ñ".repeat(255)), "");
  assert.equal(validateApiKeyName(" console-local "), "");
});

test("errors read in Spanish for the cases the backend names", () => {
  assert.match(apiKeyErrorMessage(403, "Insufficient permissions"), /permiso/);
  assert.match(apiKeyErrorMessage(404, "API key not found"), /ya no existe/);
  assert.equal(apiKeyErrorMessage(400, "api key name is required"), "api key name is required");
});

test("only a holder of api_keys:manage sees the card, so a manager does not", () => {
  const manager = ["integrations:read", "users:read", "orders:create"];
  const admin = manager.concat("integrations:manage", "api_keys:manage");

  assert.equal(showApiKeysCard((p) => manager.includes(p), "full", false), false);
  assert.equal(showApiKeysCard((p) => admin.includes(p), "full", false), true);
});
