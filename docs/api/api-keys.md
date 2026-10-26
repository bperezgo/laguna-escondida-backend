# Organization API Keys

Long-lived, revocable credentials for a system outside the POS, such as the WhatsApp agent, to act for one organization's agent user without holding a password. A key is sent as `Authorization: Bearer lgk_…` wherever a user's token is accepted, and the request acts as the key's user in the key's organization.

- **Cloud only.** The edge does not serve these routes (`404`), never receives keys, and refuses every `lgk_` bearer with `401`.
- **Admin API key.** Issuing, listing and revoking require `X-API-Key: <ADMIN_API_KEY>`. A user JWT, of any role, is refused with `401`. Organization admins cannot manage their own keys yet.
- **Agent users only.** A key can be issued only for an active user of the organization whose only role is `agent`, so a leaked key can do no more than the agent already can.
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

| Method | Endpoint                                              | Description                         |
| ------ | ----------------------------------------------------- | ----------------------------------- |
| POST   | `/api/platform/organizations/:id/api-keys`            | Issue a key for an agent user       |
| GET    | `/api/platform/organizations/:id/api-keys`            | List the organization's keys        |
| DELETE | `/api/platform/organizations/:id/api-keys/:keyId`     | Revoke a key (permanent)            |

### Common Errors

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

Revokes a key. Revocation is permanent and takes effect on the next request; revoking an already revoked key keeps its first revocation time. A suspended organization's keys can still be revoked.

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
