# Signup

A restaurant creates its own account with its Google account. There is no password signup and the cloud sends no email.

- **Cloud only**, and only while `SIGNUP_ENABLED=true` and `GOOGLE_CLIENT_ID` is set; otherwise `404`. `POST /api/signup` (password signup) does not exist.
- **No other authentication.** The Google ID token is the proof of the email.
- **Rate-limited** per client address: 10 per hour.

## Endpoints

| Method | Endpoint             | Auth            | Description                                   |
| ------ | -------------------- | --------------- | --------------------------------------------- |
| POST   | `/api/signup/google` | Google ID token | Create a pending organization, its admin and its box user, or sign in |

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

- **Organization**: `name` and `display_name` are the restaurant name; `slug` is derived from it (lowercase, accents removed, other characters joined by `-`, cut to 40, `-2`, `-3`… when taken); `status` `pending` with no `trial_ends_at`; the chosen `plan`, which is the plan its trial starts on; `America/Bogota`, `CO`, `es-CO`, `COP`. Business info, receipt text, branding and e-invoicing prefixes stay unset; the operator sets them.
- **Admin**: the Google name and email, the admin role, username = email. It has no usable password: it signs in with Google (or an admin sets a password with reset-password).
- **No box user**: signup creates only the admin. To sign in on the box, the admin creates a user without an email in Usuarios and sets its password ([`POST /api/admin/users/:id/reset-password`](./users.md#reset-a-password)); the box signs in by username.

### Approval

A new account waits as `pending` and gets **no sign-in token**, only a [setup token](./billing.md#the-setup-token) for its admin, which opens the [billing setup routes](./billing.md) and nothing else. When the cloud has its billing keys, the admin leaves a card there and the trial starts at once. Otherwise its users are refused with `403 pending_approval` until the operator starts its trial with [`POST /api/platform/organizations/:id/start-trial`](./organizations.md#start-a-trial) (14 days from that moment) or activates it with `PATCH {"status": "active"}`. Nobody is notified; the operator finds waiting signups with `GET /api/platform/organizations?status=pending`.

### Example Response (201 Created)

A new account, waiting for its trial. No sign-in token is returned and no session should be set. `setup_token` lasts 60 minutes and is accepted only by the [billing setup routes](./billing.md).

```json
{
  "status": "pending",
  "organization_name": "Restaurante Doña Rosa",
  "plan": "einvoicing",
  "setup_token": "<setup token>"
}
```

### Example Response (200 OK)

The Google email already belongs to a user: nothing is created and the response is exactly that of [`POST /api/auth/google`](./auth.md#sign-in-with-google), errors included. Signing up again before the trial starts therefore answers `403 pending_approval`, with a new `setup_token` for the admin.

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

**403 Forbidden** — The email belongs to a user whose organization is waiting for its trial, including the same person signing up again. Nothing is created. `setup_token` is present only when that user is an admin.

```json
{
  "error": "Approval pending",
  "code": "pending_approval",
  "setup_token": "<setup token>"
}
```

**403 Forbidden** — The email belongs to a user whose organization's trial has ended (`code: "trial_expired"`)

**429 Too Many Requests** — Over the per-address limit, with `Retry-After`; nothing is created
