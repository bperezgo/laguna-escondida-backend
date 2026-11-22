## Context

See proposal.md for why. Today:

- `APIKeyService` (`internal/domain/service/api_key_service.go`) has `Issue(orgID, {user_id, name})`, `List(orgID)`, `Revoke(orgID, keyID)` and `Authenticate`. `Issue` checks the user is active, in the organization and agent-only. It stores the SHA-256 hash and an 8-character prefix, and returns the plaintext once.
- The only routes are `/api/platform/organizations/:id/api-keys`, behind `AdminAPIKeyMiddleware` (`internal/bootstrap/routes_cloud.go`).
- Integrations already went the same way. The platform routes have org-scoped twins at `/api/organization/integrations` (`RegisterOrganizationIntegrationRoutes`), which take the organization from the JWT and are gated by `integrations:read/manage`, which only admin holds. The frontend reaches them through a Next proxy (`app/api/organization/integrations`, `serverApiRequest`). This change copies that pattern.
- `users` lives in the frozen `shared/` migrations. Changes to it go in both `cloud/` and `edge/`, and `TestEdgeCloudSharedSchemaParity` enforces that. Users are pulled to edge boxes (`SyncReferenceRepository.FindChangedUsers`).
- `UserService.CreateUser` requires an email for cloud roles (agent included), so the key's user cannot go through it.

## Goals / Non-Goals

**Goals:** self-service issue, list and revoke for org admins; one hidden agent user per key; a card in Ajustes.

**Non-Goals:**
- Expiry or scopes on keys.
- Renaming keys.
- Migrating the existing `whatsapp_agent` keys to dedicated users.
- Letting managers issue keys.
- Showing a key's user in the users screen.

## Decisions

### D1. Org-scoped routes mirror the integration routes
`RegisterOrganizationAPIKeyRoutes` adds `/api/organization/api-keys` (POST, GET, DELETE `/:keyId`) with `JWTAuthMiddleware` plus `RequirePermission(permissions.APIKeysManage)`. It is cloud-only and registered next to the integration routes. The organization comes from the token's tenant, never from the path or the body. New handler methods on `APIKeyHandler` (`OwnIssueHandler`, `OwnListHandler`, `OwnRevokeHandler`) read it the way `OrganizationIntegrationHandler`'s `Own*` handlers do.

*Alternative:* reuse `integrations:manage`. Rejected: a separate permission costs one constant, and it keeps the audit question "who may mint credentials" answerable on its own.

### D2. A request bearing an API key cannot manage keys
The permission alone would already stop it, since the agent role lacks `api_keys:manage`. The handlers also refuse when the principal came from an API key, so a later role change can't open a path from a key to more keys. *Alternative:* rely only on the permission. Rejected: it is a one-line guard on a credential-minting path.

### D3. Issue creates the user and the key in one unit of work
`APIKeyService.IssueForOrganization(ctx, orgID, name)` runs inside `ports.UnitOfWork.Do`:
1. Validate the name (same rules as `Issue`) and check that the organization is active.
2. Create the user through `UserRepository`. Its id is a UUID v7, its username is `apikey-<user id>` (unique without a lookup), its name is the key name, email is null, it is active, and `api_key_managed = true`. Its password is the bcrypt hash of 32 random bytes that are thrown away.
3. Assign only the `agent` role through `UserRoleRepository`.
4. Issue the key with the same code path as `Issue` (extract the plaintext, hash and prefix logic into a helper).

`APIKeyService` gains a `UnitOfWork` dependency. The existing operator `Issue` is unchanged.

*Alternative:* a separate "agent users" table. Rejected: orders' `created_by`, permissions and `Authenticate` all key off `users`, so a second table would ripple everywhere.

### D4. A boolean `users.api_key_managed` marks the dedicated user
The column is `NOT NULL DEFAULT false`, added by a migration in both `cloud/` and `edge/` (next free numbers at implementation time: today that's cloud 000093 and edge 000083, but in-flight changes may take them). The edge column always stays false. Readers:
- `UserRepository.FindAll` filters `api_key_managed = false`.
- `UserService.GetUser`, `UpdateUser`, `DeleteUser` and `ResetPassword` return `ErrUserNotFound` for a marked user, so admin CRUD and the 404 behavior come from one check.
- Both sign-in paths refuse a marked user with the existing invalid-credentials error. The random password already makes sign-in impossible; this makes it explicit.
- `FindChangedUsers` filters out marked users, so they never reach a box.
- `Authenticate` (key path) does not look at the flag.

*Alternative:* infer "managed" from "agent-only with a key". Rejected: the existing `whatsapp_agent` matches that shape and must stay visible and editable.

### D5. Revoke deactivates the dedicated user in the same unit of work
`APIKeyService.RevokeForOrganization(ctx, orgID, keyID)` revokes the key. If the key's user is marked, it also sets that user to inactive, all in one `UnitOfWork.Do`. An unmarked user is left alone. The operator `Revoke` gets the same deactivation, so both paths leave the same state. Deactivating instead of deleting keeps `created_by` on past orders valid.

### D6. The frontend gets the MCP URL from server-side config
`MCP_PUBLIC_URL` is read in the integrations page's server component and passed to the client card as a prop. Unset means no URL is shown. No backend change. *Alternative:* expose it from the backend's organization settings. Rejected: it is a deployment value of the frontend's environment, the same for every org, and the backend never needs it.

### D7. The card follows the existing integration cards
`ApiKeysCard.tsx` goes beside `MarketingAppIntegrationCard.tsx`, rendered by `IntegrationsPageClient.tsx` only when the user has `api_keys:manage`. After issuing, a dialog shows the plaintext with a copy button and the URL, and closing it drops the plaintext from state. Revoking asks for confirmation in an in-app dialog, not `window.confirm`.

## Risks / Trade-offs

- **An org admin can mint credentials without the operator knowing.** → Keys are limited to the agent role and stay cloud-only. The operator listing still shows every key, with `last_used_at`.
- **Users created for keys pile up as inactive rows after rotations.** → They are hidden and small. Purging them is a later concern.
- **The migration numbers collide with in-flight changes.** → Take the next free number when implementing. The parity test catches a cloud/edge mismatch.
- **A marked user is edited directly in the DB** (for example given a staff role). → `Authenticate` already uses the user's current roles. The operator `Issue` refuses non-agent users, but the existing key would gain those roles. This is accepted, since DB access is already full access.

## Migration Plan

1. Deploy the backend: the migration adds a column with a default, and the routes are additive. Old frontends are unaffected.
2. Deploy the frontend and set `MCP_PUBLIC_URL` in the cloud frontend's environment.
3. Rollback: redeploy the previous images. The column can stay, since nothing old reads it.

## Implementation map

Backend (`backend/`):
- `internal/domain/permissions/permissions.go`, `role_permissions.go` (+ `role_permissions_test.go`): `APIKeysManage`, admin only
- `internal/domain/dto/user.go`: `APIKeyManaged` on `User`
- `internal/domain/dto/api_key.go`: org issue request (`name` only)
- `internal/domain/service/api_key_service.go` (+ `_test.go`): `IssueForOrganization`, `RevokeForOrganization`, operator `Revoke` deactivation
- `internal/domain/service/user_service.go` (+ `_test.go`): hide marked users in get/update/delete/reset-password/sign-in
- `internal/platform/postgres/repository/user_repository.go`, `sync_reference_repository.go`, `api_key_repository.go`
- `internal/platform/postgres/migrations/cloud/`, `edge/`: `add_users_api_key_managed`
- `internal/platform/handler/api_key_handler.go` (+ `_test.go`)
- `internal/bootstrap/routes_cloud.go`, `cloud.go`: route registration, `UnitOfWork` injection
- `docs/api/api-keys.md`, `docs/examples/curl-examples.md`, `docs/mcp-server.md`

Frontend (`frontend/`):
- `app/api/organization/api-keys/route.ts`, `app/api/organization/api-keys/[keyId]/route.ts`
- `lib/api/organizationApiKeys.ts` (+ `.test.ts`), `types/organizationApiKeys.ts`
- `components/settings/ApiKeysCard.tsx`, `components/settings/IntegrationsPageClient.tsx`
- `app/(main)/ajustes/integraciones/page.tsx`: read `MCP_PUBLIC_URL`

## Open Questions

None.
