## Why

Only the platform operator can issue an organization API key today, with the admin API key. Connecting a console or agent to the customer MCP server therefore needs someone with server access, even for an organization's own setup. An organization admin should be able to issue, see and revoke their organization's keys from Ajustes, and rotate one without asking anyone.

## What Changes

- New cloud-only, JWT-authenticated routes `POST/GET /api/organization/api-keys` and `DELETE /api/organization/api-keys/:keyId`. They act on the token's organization, and only the admin role may use them (a new `api_keys:manage` permission).
- Issuing from these routes takes only a `name`. In one transaction it creates a new agent user dedicated to that key, plus the key. The plaintext is returned once, as today. One user per key means a rotation is: issue a new key, switch the client, revoke the old key.
- Revoking a key also deactivates its dedicated user. Revoking a key whose user was not created this way, for example the existing `whatsapp_agent`, revokes only the key.
- Users created for a key are hidden. They are left out of the users list, cannot be read, edited, deleted or have their password reset through user management, cannot sign in, and are not synced to edge boxes.
- Frontend: an "Claves API" card on `/ajustes/integraciones` lists the keys (name, prefix, created, last used, revoked) with Issue and Revoke actions. After issuing, it shows the key once with a copy button, next to the MCP URL when one is configured (`MCP_PUBLIC_URL`).
- The platform operator routes (`/api/platform/organizations/:id/api-keys`) are unchanged.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `org-api-keys`: organization admins can issue, list and revoke their own organization's keys. Keys issued this way get their own hidden agent user, which revoking deactivates.

## Impact

- **Backend:** `APIKeyService` gets org-scoped issue (user and key in one unit of work) and revoke-with-deactivate. There is a new handler set and route group in `routes_cloud.go`, plus a new permission granted to admin. `users` gets a marker column (cloud migration). The user list, user CRUD, sign-in and the edge user pull skip marked users.
- **Frontend:** a Next proxy route `app/api/organization/api-keys`, an API client and types, and the new card on the integrations page. `MCP_PUBLIC_URL` is read on the server.
- **Docs:** `backend/docs/api/api-keys.md` (new section for the org routes), `backend/docs/examples/curl-examples.md`, and `backend/docs/mcp-server.md` ("Adding a restaurant" can now be done from Ajustes).
- **No breaking changes.** Existing keys, the `whatsapp_agent` user and the operator routes keep working.
