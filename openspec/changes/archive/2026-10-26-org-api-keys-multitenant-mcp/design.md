## Context

See proposal.md (Why). Facts about the current code that shape the approach:

- `JWTAuthMiddleware` (`internal/platform/handler/middleware.go`) trusts the token's claims, including the roles, sets `user_id`, `org_id` and `role_ids` on the Gin context, and puts the org on the request context with `tenant.WithOrg`. Everything downstream reads those values.
- Order ownership for agents is `createdByID != user.ID` (`ensureAgentMayChange` in `order_service.go`), so whatever acts as the agent must be a user.
- A lookup that names its own organization already runs without the org scope: `UserRepository.FindByEmail`. RLS is not enabled yet (multi-tenancy plan, step 4).
- The MCP `Client` holds one email or username and password plus one cached JWT, and re-signs in on a `401`. `cmd/mcp-server/main.go` guards `/mcp` with `requireBearer(MCP_AUTH_TOKEN)`. The customer tools already read headers from `req.Extra.Header`.
- marketing-app stores, per organization, a tool server URL plus an encrypted `token`, and sends it as `Authorization: Bearer <token>` together with `X-Customer-Phone` and `X-Customer-Location`.

## Goals / Non-Goals

**Goals:**
- One customer MCP deployment serves any number of organizations, with nothing per org in its configuration.
- The backend's existing authorization (permissions, agent-only limits, tenant scoping) applies to key requests unchanged.

**Non-Goals:**
- A general machine-identity model: keys that aren't tied to a user, scopes narrower than the user's roles, or keys for staff roles.
- Self-service key management by organization admins.
- Rate limiting per key.

## Decisions

**D1. A key is issued for a user, and authenticates as that user.**
The key row references `organization_id` and `user_id`. A key request builds the same principal a token of that user would. `created_by`, `ensureAgentMayChange`, `IsAgentOnly` and order history need no change.
- *Alternative:* a key as its own principal (org plus role). This would need a second kind of actor in `created_by` and in every ownership check. Rejected.
- Keys are limited for now to users whose only role is `agent`, so the worst a leaked key can do is what the agent can already do.

**D2. Roles and status are read fresh on every key request.**
A JWT's roles are frozen at sign-in. A key lives for months, so the lookup joins the user (active, not deleted, current roles) and the organization (status `active`) on every request. That is what makes deactivating a user or suspending an organization take effect immediately.
- *Cost:* one indexed query per request. It's acceptable at WhatsApp order volume. A short in-process cache can come later without changing the spec.

**D3. Format and storage.**
- The plaintext is `lgk_` followed by 32 random bytes, base64url-encoded.
- The stored hash is `sha256(plaintext)`. A random 256-bit key doesn't need a slow hash, and a fast lookup by `hash` (unique index) avoids comparing in a loop.
- The first 8 characters after the prefix are stored separately, for recognizing a key in the list.
- The table is `api_keys(id uuid v7, organization_id, user_id, name, prefix, hash, created_at, last_used_at, revoked_at)` in a **cloud-only** migration, so edge boxes never have the table and the sync pull has nothing to leave out.

**D4. Lookup by hash runs without the org scope.**
This follows `FindByEmail`: the key is what names the organization. When RLS is enabled (plan step 4), this one lookup needs the same treatment as the email lookup, either by keeping `api_keys` out of RLS like `organizations` or with a narrow `SECURITY DEFINER` function. That decision is recorded here so step 4 doesn't miss the table.

**D5. One middleware path, chosen by prefix.**
`JWTAuthMiddleware` keeps its signature and every route keeps its wiring. A bearer starting with `lgk_` goes to the API-key service. Anything else goes to JWT validation as today. Both paths set the same Gin keys and `tenant.WithOrg`. On the edge, the API-key service isn't wired, so an `lgk_` bearer is refused with the generic 401.
- Bearer was chosen over `X-API-Key`: it doesn't collide with the admin key, and marketing-app already sends a Bearer.
- `last_used_at` is updated in the background, at most once a minute per key, so authentication never waits on a write.

**D6. The customer MCP forwards the caller's bearer, and that's all.**
- In customer mode, `/mcp` is wrapped by a guard that requires `Bearer lgk_…`, so a pasted staff JWT doesn't get the customer tools with staff power. The guard stores the bearer on the request context.
- `Client` gains a per-request credential: when the context carries a bearer, it's sent and the cached service-account token isn't used.
- A `401` from the backend becomes a tool error, with no sign-in and no retry.
- `NewConfig` stops requiring the auth token and the service account when the toolset is `customer`.
- The full toolset keeps today's sign-in path untouched.
- *Alternative:* the MCP validates keys itself, or maps keys to orgs. That duplicates backend auth and needs per-org configuration. Rejected: the backend is the single place keys are checked.

**D7. Hard cutover.**
Laguna's WhatsApp orders are not live yet, so the customer mode loses `MCP_AUTH_TOKEN` and the service account outright. There's no compatibility setting.

## Risks / Trade-offs

- **[A leaked key works outside the MCP]** A key can call the API directly and list all of its organization's agent orders, including phones and addresses, because the phone check lives in the toolset. → This is the same exposure as the agent user's password today, and it stays within one org. Revoke the key from the list endpoint. A backend phone filter for agent principals is a later option.
- **[Keys without expiry]** → Revocation, `last_used_at` for spotting stale keys, and the immediate effect of deactivating a user or suspending an org. Expiry can be added as a column later.
- **[RLS lands later and breaks the lookup]** → D4 records it, and the task list adds a test that fails if the key lookup is scoped.
- **[One shared MCP process for every org]** A bug that mixed credentials would leak across orgs. → The bearer is per request, with no shared mutable token in customer mode, and a two-org test drives the same server with both keys concurrently.

## Migration Plan

1. Deploy the backend with the cloud migration. The routes and middleware path are additive, and existing JWT clients are unaffected.
2. Issue Laguna's key for its `agent` user (`POST /api/platform/organizations/:id/api-keys`).
3. Deploy the MCP server in customer mode without `MCP_AUTH_TOKEN` or the service account.
4. In the marketing-app console, point Laguna's orders tool server at the MCP URL and set its token to the key. The console's server status should show `connected`.
5. Remove Laguna's old agent password from the MCP deployment's secrets.

**Rollback:** redeploy the previous MCP image with its old environment. The backend change can stay, since it's additive.
