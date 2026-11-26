# Organization API Keys

> **Plan:** in the cloud these routes need the `full` plan; otherwise `403 plan_required`. A suspended organization or an ended trial is refused first. See [Organization access refusals](./auth.md#organization-access-refusals).

Long-lived, revocable credentials for a system outside the POS, such as the WhatsApp agent, to act for one organization's agent user without holding a password. A key is sent as `Authorization: Bearer lgk_…` wherever a user's token is accepted, and the request acts as the key's user in the key's organization.

- **Cloud only.** The edge does not serve these routes (`404`), never receives keys, and refuses every `lgk_` bearer with `401`.
- **Two ways in.** The platform operator manages any organization's keys with `X-API-Key: <ADMIN_API_KEY>` (`/api/platform/organizations/:id/api-keys`). An organization's admin manages their own organization's keys with their sign-in token (`/api/organization/api-keys`, see [Organization routes](#organization-routes)).
- **Agent users only.** A key acts as a user whose only role is `agent`, so a leaked key can do no more than the agent already can. The operator names an existing agent user; the organization route creates a new one for each key.
- **Shown once.** The plaintext is in the issue response only. The server stores a SHA-256 hash and the first 8 characters after `lgk_`, for recognizing the key in the list.
- **No expiry.** A key works until it is revoked, its user is deactivated or deleted, or its organization is suspended.

## API Key Fields

| Field             | Type           | Description                                                        |
| ----------------- | -------------- | ------------------------------------------------------------------ |
| `id`              | string (UUID)  | Key id (v7), used to revoke it                                     |
| `organization_id` | string (UUID)  | The organization the key acts for                                  |
| `user_id`         | string (UUID)  | The agent user the key authenticates as                            |
| `name`            | string         | A label, e.g. `whatsapp-agent`                                     |
| `prefix`          | string         | The first 8 characters after `lgk_`, to recognize the key          |
| `created_at`      | string         | RFC 3339                                                           |
| `last_used_at`    | string \| null | RFC 3339; updated at most once a minute, `null` if never used      |
| `revoked_at`      | string \| null | RFC 3339; `null` while the key works                               |
| `key`             | string         | **Issue response only.** The plaintext, `lgk_` + 43 characters     |

## Endpoints

| Method | Endpoint                                              | Auth          | Description                              |
| ------ | ----------------------------------------------------- | ------------- | ---------------------------------------- |
| POST   | `/api/platform/organizations/:id/api-keys`            | Admin API key | Issue a key for an agent user            |
| GET    | `/api/platform/organizations/:id/api-keys`            | Admin API key | List the organization's keys             |
| DELETE | `/api/platform/organizations/:id/api-keys/:keyId`     | Admin API key | Revoke a key (permanent)                 |
| POST   | `/api/organization/api-keys`                          | Admin's token | Issue a key with its own agent user      |
| GET    | `/api/organization/api-keys`                          | Admin's token | List the token's organization's keys     |
| DELETE | `/api/organization/api-keys/:keyId`                   | Admin's token | Revoke a key (permanent)                 |

The platform routes are described first; the [organization routes](#organization-routes) share their fields and errors.

### Common Errors (platform routes)

**401 Unauthorized** - Missing or wrong `X-API-Key` (a JWT or an API key is not accepted)

```json
{
  "error": "API key is required"
}
```

**404 Not Found** - No organization has the id (or, when issuing, it is suspended)

```json
{
  "error": "Organization not found"
}
```

---

## Issue API Key

Creates a key for one of the organization's agent users. The response is the only time the plaintext is returned; store it in the calling system's secrets right away.

```
POST /api/platform/organizations/:id/api-keys
```

### Request Body

| Field     | Type          | Required | Description                                                     |
| --------- | ------------- | -------- | --------------------------------------------------------------- |
| `user_id` | string (UUID) | Yes      | An active user of this organization whose only role is `agent`  |
| `name`    | string        | Yes      | Not blank, at most 255 characters (trimmed)                     |

### Example Request

```json
{
  "user_id": "019a1c2e-7b40-7c3d-9a51-6f2d8e4b1a07",
  "name": "whatsapp-agent"
}
```

### Example Response (201 Created)

```json
{
  "id": "019a2f61-0c88-7e12-b4a9-3d5c7e9f2b10",
  "organization_id": "bb0cad3e-be13-4bad-9a37-bba9525bfece",
  "user_id": "019a1c2e-7b40-7c3d-9a51-6f2d8e4b1a07",
  "name": "whatsapp-agent",
  "prefix": "Xk9vQ2mL",
  "created_at": "2026-10-26T15:04:05.123456Z",
  "last_used_at": null,
  "revoked_at": null,
  "key": "lgk_Xk9vQ2mLxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

### Error Responses

**400 Bad Request** - The user is unknown, belongs to another organization, is inactive, or holds any role other than `agent`. No key is created.

```json
{
  "error": "an api key can only be issued for an active user of the organization whose only role is agent",
  "field": "user_id"
}
```

**400 Bad Request** - `name` is blank or too long

```json
{
  "error": "api key name is required",
  "field": "name"
}
```

**400 Bad Request** - Malformed JSON

```json
{
  "error": "Invalid request body"
}
```

**404 Not Found** - Unknown or suspended organization

```json
{
  "error": "Organization not found"
}
```

---

## List API Keys

Returns every key of the organization, newest first, including revoked ones. Neither the plaintext nor the hash is ever returned. A suspended organization's keys can still be listed.

```
GET /api/platform/organizations/:id/api-keys
```

### Example Response (200 OK)

```json
{
  "api_keys": [
    {
      "id": "019a2f61-0c88-7e12-b4a9-3d5c7e9f2b10",
      "organization_id": "bb0cad3e-be13-4bad-9a37-bba9525bfece",
      "user_id": "019a1c2e-7b40-7c3d-9a51-6f2d8e4b1a07",
      "name": "whatsapp-agent",
      "prefix": "Xk9vQ2mL",
      "created_at": "2026-10-26T15:04:05.123456Z",
      "last_used_at": "2026-10-26T18:22:41.902113Z",
      "revoked_at": null
    }
  ]
}
```

### Error Responses

**404 Not Found** - Unknown organization

```json
{
  "error": "Organization not found"
}
```

---

## Revoke API Key

Revokes a key. Revocation is permanent and takes effect on the next request; revoking an already revoked key keeps its first revocation time. A suspended organization's keys can still be revoked. When the key's user was created for that key (an [organization route](#organization-routes) issue), that user is deactivated too; an agent user that predates its keys stays active, and its other keys keep working.

```
DELETE /api/platform/organizations/:id/api-keys/:keyId
```

### Example Response (204 No Content)

No body.

### Error Responses

**404 Not Found** - The key is unknown or belongs to another organization. The other organization's key keeps working.

```json
{
  "error": "API key not found"
}
```

**404 Not Found** - Unknown organization

```json
{
  "error": "Organization not found"
}
```

---

## Organization routes

An organization's admin issues, lists and revokes their own organization's keys from Ajustes, with their sign-in token. The organization is always the token's, never the path's or the body's.

- **Admin only.** The routes need the `api_keys:manage` permission, which only the admin role holds. Any other role gets `403`, and so does a request whose bearer is an `lgk_` key, whatever its user's roles.
- **One user per key.** Issuing creates, in one transaction, a new active user of the organization whose only role is `agent`, and the key for it. Nothing is left behind if either fails. To rotate a key: issue a new one, switch the client to it, revoke the old one.
- **That user is hidden.** It is not in `GET /api/admin/users`; reading, updating, deleting or resetting its password through `/api/admin/users/:id` returns `404`; it cannot sign in; and it is never synced to an edge box. Orders created with its key still record it as `created_by`.
- **Revoking deactivates it.** Revoking the key, from either route, deactivates that user. Revoking a key the operator issued for an existing agent user (such as `whatsapp_agent`) revokes only the key.
- **Cloud only.** The edge does not serve these routes (`404`).

### Common Errors (organization routes)

**401 Unauthorized** - Missing, invalid or expired token

```json
{
  "error": "Invalid or expired token"
}
```

**403 Forbidden** - The token's role lacks `api_keys:manage`

```json
{
  "error": "Insufficient permissions",
  "required": "api_keys:manage"
}
```

**403 Forbidden** - The bearer is an organization API key

```json
{
  "error": "Insufficient permissions"
}
```

### Issue a Key

```
POST /api/organization/api-keys
```

| Field  | Type   | Required | Description                                 |
| ------ | ------ | -------- | ------------------------------------------- |
| `name` | string | Yes      | Not blank, at most 255 characters (trimmed) |

Any other field, `user_id` included, is ignored.

```json
{
  "name": "console-local"
}
```

**201 Created** - Same body as the platform issue. `user_id` is the user created for this key. The plaintext `key` is returned only here.

```json
{
  "id": "019a2f61-0c88-7e12-b4a9-3d5c7e9f2b10",
  "organization_id": "bb0cad3e-be13-4bad-9a37-bba9525bfece",
  "user_id": "019a2f61-0c80-7a01-8c3e-5b7d9f1a3c22",
  "name": "console-local",
  "prefix": "Xk9vQ2mL",
  "created_at": "2026-11-21T15:04:05.123456Z",
  "last_used_at": null,
  "revoked_at": null,
  "key": "lgk_Xk9vQ2mLxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

**400 Bad Request** - `name` is blank or longer than 255 characters (`"field": "name"`), or the JSON is malformed (`"Invalid request body"`). Nothing is created.

```json
{
  "error": "api key name is required",
  "field": "name"
}
```

**404 Not Found** - The organization is suspended

```json
{
  "error": "Organization not found"
}
```

### List Keys

```
GET /api/organization/api-keys
```

**200 OK** - Every key of the token's organization, newest first, including revoked keys and keys the operator issued. Same body as the platform list, never the plaintext or the hash.

### Revoke a Key

```
DELETE /api/organization/api-keys/:keyId
```

**204 No Content** - The key is revoked, and its user is deactivated if it was created for that key.

**404 Not Found** - The key is unknown or belongs to another organization. Nothing changes, and the other organization's key keeps working.

```json
{
  "error": "API key not found"
}
```

---

## Using a Key

Send the key as a bearer to any protected cloud endpoint:

```
Authorization: Bearer lgk_Xk9vQ2mLxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

The request acts as the key's user in the key's organization, with that user's **current** roles, exactly as a token of that user would. Orders it creates have `created_by` set to the key's user, the agent-only limits apply (for example `POST /api/orders/pay-order` returns `403`), and another organization's data is not found.

**401 Unauthorized** - The key is unknown, revoked, its user is deactivated or deleted, its organization is suspended, or the request reached an edge box. The body is the same in every case and the same as for an invalid token, so it reveals nothing about which keys, users or organizations exist. The specific reason is logged server-side, without the key.

```json
{
  "error": "Invalid or expired token"
}
```
