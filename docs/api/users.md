# Users

User management endpoints. Creating users is an **admin-only** operation guarded by an API key rather than a JWT.

**Cloud only: user writes.** The box (edge) serves sign-in, `/api/auth/me`, `GET /api/admin/users`, `GET /api/admin/users/:id` and `GET /api/admin/roles`, and gets its users from the cloud's pull. These routes exist only in the cloud; on the box they return `404`:

- `POST /api/users` (admin-key bootstrap)
- `POST /api/admin/users`, `PUT /api/admin/users/:id`, `POST /api/admin/users/:id/reset-password`, `DELETE /api/admin/users/:id`

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

### Example Request

```json
{
  "organization": "laguna-escondida",
  "username": "john_doe",
  "name": "John Doe",
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

> The `password` field is never returned in any response.

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

**500 Internal Server Error** — User could not be created

```json
{
  "error": "Failed to create user"
}
```

> Note: empty required fields (`username`, `name`, or `password`) are rejected during creation but currently surface as a generic `500 Internal Server Error` (`{"error": "Internal server error"}`), because no request-body validator is wired into this endpoint. Always send all required fields.
