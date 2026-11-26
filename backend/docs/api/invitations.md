# Invitations

An admin brings the people who use the cloud app (managers, accountants, couriers, other admins) with a link they share themselves, for example on WhatsApp. The invitee sets a password or uses Google. The cloud sends no email. Box staff are not invited: create them with [`POST /api/admin/users`](./users.md).

- **Cloud only.** The edge serves none of these routes (`404`).
- **The link is shown once**, in the create and new-link responses (`Cache-Control: no-store`). Only a SHA-256 of its token is stored. It is valid for 7 days.
- **No limit** on the number of users or invitations of an organization.

## Endpoints

| Method | Endpoint                              | Auth                    | Description                    |
| ------ | ------------------------------------- | ----------------------- | ------------------------------ |
| POST   | `/api/admin/invitations`              | JWT (`users:create`)    | Invite a person, get the link  |
| GET    | `/api/admin/invitations`              | JWT (`users:create`)    | List pending invitations       |
| POST   | `/api/admin/invitations/:id/link`     | JWT (`users:create`)    | New link; the old one stops working |
| DELETE | `/api/admin/invitations/:id`          | JWT (`users:create`)    | Revoke                         |
| GET    | `/api/invitations/:token`             | None (rate-limited)     | What the acceptance page shows |
| POST   | `/api/invitations/:token/accept`      | None (rate-limited)     | Accept with a password or Google |

The two public routes allow 30 requests per client address in 10 minutes, then `429` with `Retry-After`.

---

## Invite

`POST /api/admin/invitations`

### Request Body

| Field      | Type     | Required | Description                                                         |
| ---------- | -------- | -------- | ------------------------------------------------------------------- |
| `email`    | string   | Yes      | Stored trimmed and lowercased                                       |
| `name`     | string   | Yes      | 1–255 characters                                                    |
| `role_ids` | number[] | Yes      | Only admin (2), manager (3), accountant (5) or courier (7). Admin only when the caller is an admin |

```json
{
  "email": "contadora@example.com",
  "name": "Marta Gómez",
  "role_ids": [5]
}
```

### Example Response (201 Created)

```json
{
  "invitation": {
    "id": "019a7c1e-3b2f-7d10-9c4e-5f6a7b8c9d0e",
    "email": "contadora@example.com",
    "name": "Marta Gómez",
    "role_ids": [5],
    "roles": [{ "id": 5, "name": "accountant", "created_at": "2024-01-01T00:00:00Z" }],
    "invited_by": "019a7c1e-0000-7000-8000-0000000000a1",
    "invited_by_name": "",
    "expires_at": "2026-12-02T15:00:00Z",
    "created_at": "2026-11-25T15:00:00Z"
  },
  "link": "https://app.kuenta.co/invitacion/q3L0Zx…43 characters"
}
```

`invited_by_name` is filled in the list, not in this response.

### Error Responses

**400 Bad Request** — An agent or floor role, no roles, no name or no email

```json
{ "error": "invitations are for admin, manager, accountant or courier roles only" }
```

**403 Forbidden** — A non-admin invites with the admin role

```json
{ "error": "Only an admin can invite an admin" }
```

**409 Conflict** — The email already belongs to a user in any organization

```json
{ "error": "Email is already in use" }
```

**409 Conflict** — The email has a pending invitation in this organization; get a new link for it instead

```json
{ "error": "This email already has a pending invitation; get a new link for it instead" }
```

---

## List Pending Invitations

`GET /api/admin/invitations`

Invitations neither accepted nor revoked, newest first. Expired ones are listed (get a new link for them). No link is returned.

```json
{
  "invitations": [
    {
      "id": "019a7c1e-3b2f-7d10-9c4e-5f6a7b8c9d0e",
      "email": "contadora@example.com",
      "name": "Marta Gómez",
      "role_ids": [5],
      "roles": [{ "id": 5, "name": "accountant", "created_at": "2024-01-01T00:00:00Z" }],
      "invited_by": "019a7c1e-0000-7000-8000-0000000000a1",
      "invited_by_name": "Rosa Pérez",
      "expires_at": "2026-12-02T15:00:00Z",
      "created_at": "2026-11-25T15:00:00Z"
    }
  ]
}
```

---

## New Link

`POST /api/admin/invitations/:id/link`

Returns the same shape as Invite (`200 OK`), with a fresh 7-day link. The previous link stops working.

**404 Not Found** — Unknown, accepted or revoked invitation, or one of another organization

---

## Revoke

`DELETE /api/admin/invitations/:id`

`204 No Content`. The link stops working. `404` as for New Link.

---

## Look Up an Invitation

`GET /api/invitations/:token`

### Example Response (200 OK)

```json
{
  "organization_name": "Restaurante Doña Rosa",
  "email": "contadora@example.com",
  "name": "Marta Gómez",
  "roles": [{ "id": 5, "name": "accountant", "created_at": "2024-01-01T00:00:00Z" }],
  "expires_at": "2026-12-02T15:00:00Z"
}
```

**400 Bad Request** — Unknown, expired, revoked or accepted link; one body for all

```json
{ "error": "Invalid or expired link", "code": "invalid_or_expired_link" }
```

---

## Accept

`POST /api/invitations/:token/accept`

Send **either** a password **or** a Google ID token.

```json
{ "password": "una-clave-segura" }
```

```json
{ "id_token": "<ID token from Google>" }
```

- A password needs at least 8 characters; the user's email is the invitation's, as typed by the admin.
- A Google token must be valid as for [Google sign-in](./auth.md#sign-in-with-google) and its email must equal the invitation's.

Creates the user in the inviting organization with the invitation's name, email and roles, username = email, and marks the invitation accepted, under a row lock so one link creates one user.

### Example Response (201 Created)

The same body as `POST /api/auth/signin`, for the new user.

### Error Responses

**400 Bad Request** — Unknown, expired, revoked or accepted link

```json
{ "error": "Invalid or expired link", "code": "invalid_or_expired_link" }
```

**400 Bad Request** — No credential, both, a short password, a Google token whose email isn't the invited one, or a Google token while the cloud has no Google client id

```json
{ "error": "The Google account's email is not the invited one" }
```

**403 Forbidden** — The organization is suspended or its trial has ended; nothing is created

```json
{ "error": "Organization is not active", "code": "organization_inactive" }
```

**409 Conflict** — The email was taken by another account since the invitation was created

```json
{ "error": "Email is already in use" }
```
