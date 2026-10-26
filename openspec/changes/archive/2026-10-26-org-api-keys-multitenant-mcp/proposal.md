## Why

The customer MCP server gets its organization from its own environment: it signs in once as one service account, caches one JWT and accepts one shared `MCP_AUTH_TOKEN`. Each restaurant would therefore need its own deployment, which the multi-tenancy plan rejected. marketing-app already stores an MCP URL and a token per organization and sends that token as the Bearer, but the backend has no credential that a third system can hold per organization, and the MCP server has no way to act for the caller instead of for itself. Laguna's WhatsApp orders are not live yet, so this can ship as a hard cutover before they are.

## What Changes

- **Org API keys (cloud only).** A platform operator issues a key for an organization's agent user. The plaintext is shown once. Only its hash is stored, and the key can be listed and revoked. A request bearing `Authorization: Bearer lgk_…` authenticates as that user in that organization, exactly as the user's own token would, so order ownership, `created_by` and the agent-only limits are unchanged.
- A key stops working when it is revoked, when its user is deactivated or deleted, or when its organization is suspended. Every one of those, and an unknown key, returns the same `401`.
- Edge boxes never receive keys and refuse any `lgk_` bearer.
- **BREAKING (customer MCP).** With `MCP_TOOLSET=customer`, the MCP server no longer signs in or holds an identity of its own. It requires an `lgk_` bearer on `/mcp` and forwards that bearer on every backend call, so one deployment serves every organization. `MCP_AUTH_TOKEN`, `LAGUNA_EMAIL`, `LAGUNA_USERNAME` and `LAGUNA_PASSWORD` are no longer used in customer mode. `X-Customer-Phone` and `X-Customer-Location` are unchanged.
- The full (staff) toolset, the stdio transport and the local Claude Code / Claude Desktop setup are unchanged.

Out of scope: `platform_admin` (keys are issued behind `ADMIN_API_KEY` for now), organization admins managing their own keys, keys for users with staff roles, and a backend-side customer-phone filter for agent principals.

## Capabilities

### New Capabilities

- `org-api-keys`: issuing, listing and revoking an organization's API keys, and authenticating a request as the key's user in the key's organization.

### Modified Capabilities

- `customer-agent-access`: the customer toolset authenticates each MCP request with the caller's org API key and acts for that key's organization, instead of a shared secret and one service account.
- `tenant-isolation`: an org API key is a credential that names an organization, and edge boxes refuse it.

## Impact

- **Database:** a new cloud migration for `api_keys` (organization, user, name, hash, prefix, `last_used_at`, `revoked_at`). It is never synced to edge boxes.
- **Backend:**
  - an API-key service and port, with a repository whose lookup by hash runs without the org scope
  - the `JWTAuthMiddleware` path accepts `lgk_` bearers
  - platform routes under `/api/platform/organizations/:id/api-keys`
  - route golden files
- **MCP server:**
  - `internal/platform/mcpserver/{config.go,client.go,server.go}` and `cmd/mcp-server/main.go`: a per-request credential, the `lgk_` guard, and customer-mode config validation
- **Docs:** a new `docs/api/api-keys.md`, plus updates to `docs/mcp-server.md` and `docs/examples/curl-examples.md`.
- **marketing-app:** no code change. Each org's orders tool server points at the one MCP URL, with its `token` set to that org's key.
- **Rollout:** deploy the backend, then the MCP server. Issue Laguna's key, set it in the marketing-app console, then delete Laguna's old agent password from the MCP deployment.
