# Signup

A restaurant creates its own account with its Google account. There is no password signup and the cloud sends no email.

- **Cloud only**, and only while `SIGNUP_ENABLED=true` and `GOOGLE_CLIENT_ID` is set; otherwise `404`. `POST /api/signup` (password signup) does not exist.
- **No other authentication.** The Google ID token is the proof of the email.
- **Rate-limited** per client address: 10 per hour.

## Endpoints

| Method | Endpoint             | Auth            | Description                                   |
| ------ | -------------------- | --------------- | --------------------------------------------- |
| POST   | `/api/signup/google` | Google ID token | Create the organization, its admin and its box user, or sign in |

---

## Sign Up with Google

`POST /api/signup/google`

The ID token comes from Google Identity Services in the browser. It is accepted only when Google's signature is valid, its audience is the cloud's client id, it is unexpired and its email is verified by Google.

### Request Body

| Field             | Type   | Required | Description                               |
| ----------------- | ------ | -------- | ----------------------------------------- |
| `id_token`        | string | Yes      | Google ID token (JWT)                     |
| `restaurant_name` | string | Yes      | 1–255 characters after trimming           |
| `plan`            | string | Yes      | `starter`, `einvoicing` or `full` (see [Plans](./organizations.md#plans)) |

```json
{
  "id_token": "<ID token from Google>",
  "restaurant_name": "Restaurante Doña Rosa",
  "plan": "einvoicing"
}
```

### What it creates

All together, or nothing:

- **Organization**: `name` and `display_name` are the restaurant name; `slug` is derived from it (lowercase, accents removed, other characters joined by `-`, cut to 40, `-2`, `-3`… when taken); `status` `trial` with `trial_ends_at` 14 days ahead; the chosen `plan`; `America/Bogota`, `CO`, `es-CO`, `COP`. Business info, receipt text, branding and e-invoicing prefixes stay unset; the operator sets them.
- **Admin**: the Google name and email, the admin role, username = email. It has no usable password: it signs in with Google (or an admin sets a password with reset-password).
- **Box user**: name `Caja`, the manager role, no email. Username is the email's part before `@`, keeping letters, digits, `.`, `_` and `-` (`caja` when fewer than 3 remain). Its random password is never returned here; get one with [`POST /api/admin/users/:id/generate-password`](./users.md#generate-a-password).

### Example Response (201 Created)

A new account: the same body as `POST /api/auth/signin`, for the new admin.

```json
{
  "token": "<JWT>",
  "username": "rosa.perez@example.com",
  "roles": [{ "id": 2, "name": "admin", "created_at": "2024-01-01T00:00:00Z" }],
  "permissions": ["orders:read", "orders:create", "users:create", "edge:enroll"]
}
```

### Example Response (200 OK)

The Google email already belongs to a user: nothing is created and the response is exactly that of [`POST /api/auth/google`](./auth.md#sign-in-with-google), errors included.

### Error Responses

**400 Bad Request** — An invalid field, named in `field`

```json
{
  "error": "invalid plan: must be starter, einvoicing or full",
  "field": "plan"
}
```

```json
{
  "error": "invalid restaurant_name: must be 1-255 characters",
  "field": "restaurant_name"
}
```

**401 Unauthorized** — The ID token is invalid (signature, audience, expiry or unverified email)

```json
{
  "error": "Google sign-in failed"
}
```

**403 Forbidden** — The email belongs to a user whose organization's trial has ended (`code: "trial_expired"`)

**429 Too Many Requests** — Over the per-address limit, with `Retry-After`; nothing is created
