# Users

User management endpoints. Creating users is an **admin-only** operation guarded by an API key rather than a JWT.

**Cloud only: user writes.** The box (edge) serves sign-in, `/api/auth/me`, `GET /api/admin/users`, `GET /api/admin/users/:id` and `GET /api/admin/roles`, and gets its users from the cloud's pull. These routes exist only in the cloud; on the box they return `404`:

- `POST /api/users` (admin-key bootstrap)
- `POST /api/admin/users`, `PUT /api/admin/users/:id`, `POST /api/admin/users/:id/reset-password`, `DELETE /api/admin/users/:id`

People who use the cloud app can also be brought in with an [invitation link](invitations.md).

For authentication (sign in) and reading the current user, see [Authentication](auth.md).

## Endpoints

| Method | Endpoint      | Description         | Auth      |
| ------ | ------------- | ------------------- | --------- |
| POST   | `/api/users`  | Create a new user   | API Key   |

---

## Create User

`POST /api/users`

Creates a new user in an organization, hashes the password, and assigns the given roles. A user belongs to exactly one organization; usernames are unique within it, and the same username may exist in another organization.

This is the bootstrap path, authenticated by the admin API key alone, so it names the organization. A signed-in manager creating users through `POST /api/admin/users` always creates them in their own organization; any `organization` in that body is ignored.

### Headers

| Header        | Value             | Required | Description                          |
| ------------- | ----------------- | -------- | ------------------------------------ |
| X-API-Key     | `<admin_api_key>` | Yes      | Admin API key. **Not** a JWT bearer token. |
| Content-Type  | `application/json`| Yes      |                                      |

### Request Body

| Field        | Type     | Required             | Description                                                        |
| ------------ | -------- | -------------------- | ------------------------------------------------------------------ |
| organization | string   | Cloud: Yes; Edge: No | Slug of the organization the user belongs to. On an edge box it may be omitted (the box's organization is used); when sent it must be the box's. |
| username     | string   | Yes                  | Username, unique within the organization (3–255 chars)            |
| name         | string   | Yes                  | Display name of the user (1–255 chars)                             |
| email        | string   | Admin, accountant, agent, courier | Up to 255 chars. Required when `role_ids` holds admin, accountant, agent or courier; optional for a manager (a box-only manager, who signs in only on the box) and for floor staff. Stored trimmed and lowercased, and unique across **all** organizations, case-insensitively. Cloud users sign in with it |
| password     | string   | Yes                  | Plain-text password (min 6 chars). Hashed before storage           |
| role_ids     | number[] | Yes                  | At least one valid role ID. Every ID must exist (see Roles below)  |

### Roles

`role_ids` must reference existing roles. The seeded roles are:

| ID | Name      |
| -- | --------- |
| 1  | server    |
| 2  | admin     |
| 3  | manager   |
| 4  | cooker    |
| 5  | accountant |
| 6  | agent     |
| 7  | courier   |

Cloud roles: admin, manager, accountant, agent, courier. Floor roles: server, cooker. A courier ("Repartidor") signs in to the cloud to claim and report deliveries (see [deliveries.md](deliveries.md)); it holds only `deliveries:carry`. Couriers are pulled to the box like every user but hold no permission there.

### Example Request

```json
{
  "organization": "laguna-escondida",
  "username": "john_doe",
  "name": "John Doe",
  "email": "John.Doe@Example.com",
  "password": "securepassword123",
  "role_ids": [2, 3]
}
```

### Example Response (201 Created)

The created user (without the password) together with its assigned roles.

```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "john_doe",
    "name": "John Doe",
    "email": "john.doe@example.com",
    "active": true,
    "created_at": "2026-06-23T15:30:00Z",
    "updated_at": "2026-06-23T15:30:00Z"
  },
  "roles": [
    {
      "id": 2,
      "name": "admin",
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": 3,
      "name": "manager",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

> The `password` field is never returned in any response. `email` is `null` for a user without one.
>
> Emails stay in the cloud: an edge box's copy of a user (from the pull) never carries it.

### Error Responses

**400 Bad Request** — Malformed JSON body

```json
{
  "error": "Invalid request body"
}
```

**400 Bad Request** — No `organization` in the cloud

```json
{
  "error": "organization is required"
}
```

**400 Bad Request** — On an edge box, an `organization` other than the box's

```json
{
  "error": "organization does not match this box's"
}
```

**400 Bad Request** — Admin, accountant, agent or courier without an `email`

```json
{
  "error": "email is required for an admin, accountant, agent or courier"
}
```

**400 Bad Request** — `email` is not a valid address (a bare address only, no display name) or longer than 255 chars

```json
{
  "error": "email is not a valid address",
  "code": "USER_INVALID_EMAIL"
}
```

**400 Bad Request** — One or more `role_ids` are not valid roles

```json
{
  "error": "Invalid role IDs provided"
}
```

**401 Unauthorized** — Missing API key

```json
{
  "error": "API key is required"
}
```

**401 Unauthorized** — Wrong API key

```json
{
  "error": "Invalid API key"
}
```

**404 Not Found** — No organization has that slug

```json
{
  "error": "Organization not found"
}
```

**404 Not Found** — One or more roles do not exist

```json
{
  "error": "One or more roles not found"
}
```

**409 Conflict** — Username already taken in this organization

```json
{
  "error": "User already exists"
}
```

**409 Conflict** — Email already used by another user, in any organization (compared case-insensitively)

```json
{
  "error": "Email is already in use"
}
```

**500 Internal Server Error** — User could not be created

```json
{
  "error": "Failed to create user"
}
```

> Note: empty required fields (`username`, `name`, or `password`) are rejected during creation but currently surface as a generic `500 Internal Server Error` (`{"error": "Internal server error"}`), because no request-body validator is wired into this endpoint. Always send all required fields.

---

## Email on Update

`PUT /api/admin/users/:id` (JWT with `users:update`, cloud only) accepts an optional `email`, alongside `name`, `role_ids` and `active`:

```json
{
  "role_ids": [3],
  "email": "cajero1@lagunaescondida.co"
}
```

- Omitted: the email is unchanged. `""`: the email is removed.
- The rule is checked on the resulting user: if, after the update, the user holds admin, accountant, agent or courier and has no email, the request is rejected with `400` `{"error": "email is required for an admin, accountant, agent or courier"}` and nothing is stored. A manager may have no email; without one they sign in only on the box.
- A taken email returns `409` `{"error": "Email is already in use"}`; an invalid one returns `400` with `"code": "USER_INVALID_EMAIL"`, as on create.

---

## Reset a Password

`POST /api/admin/users/:id/reset-password` (JWT with `users:update`, cloud only)

Sets the password the admin types for a user of the caller's organization, such as a user without an email who signs in only on the box. The box gets it on its next pull. The password is never returned.

### Request Body

```json
{
  "password": "NuevaClave123"
}
```

`password` is required, minimum 6 characters.

### Responses

- **204 No Content**: the password changed.
- **400 Bad Request**: invalid body.
- **403 Forbidden**: the caller lacks `users:update`.
- **404 Not Found**: no such user in the caller's organization (another organization's user is also `404`), or the user is managed by an API key.
