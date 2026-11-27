# Billing Setup API

What a new restaurant's admin does between signing up and the trial: read the setup state, choose plan and period, and leave a card. Leaving a card starts the trial and signs the person in.

- **Cloud only.** None of these routes exist on an edge box.
- **Auth: a setup token**, sent as `Authorization: Bearer <setup_token>`. Nothing else is accepted: a sign-in token, an organization API key and the platform key all get `401`.
- **Only while the organization is `pending`.** Once its trial started (by a saved card or by the operator), every route answers `409 not_pending`.
- **No card data reaches the cloud.** The browser sends the card to Wompi and the cloud receives Wompi's token for it.

## The setup token

A JWT with audience `billing-setup` that names the admin and the organization and carries no role or permission. It expires **60 minutes** after it is issued. It is returned in two places:

- the `201` of [`POST /api/signup/google`](./signup.md#sign-up-with-google), as `setup_token`;
- the `403 pending_approval` of a sign-in that would otherwise have succeeded ([`POST /api/auth/signin`](./auth.md#sign-in), [`POST /api/auth/google`](./auth.md#sign-in-with-google)), as `setup_token`, **only for a user with the admin role**.

Every other route refuses a setup token with the same `401` as an invalid token.

## Card entry

Card entry is on only when the cloud is configured with the platform's own Wompi keys: `BILLING_WOMPI_PUBLIC_KEY` and `BILLING_WOMPI_PRIVATE_KEY`. Both must be set, and both of the same environment (`pub_test_`/`prv_test_` or `pub_prod_`/`prv_prod_`); one without the other, or a mixed pair, stops the cloud at startup. The Wompi address is `WOMPI_API_URL`. These are not an organization's [Wompi integration](./organization-integrations.md).

With card entry off, the state has `card_entry: false`, the card route answers `404`, and the operator starts the trial with [`POST /api/platform/organizations/:id/start-trial`](./organizations.md#start-a-trial), which works with or without a saved card.

## Endpoints

| Method | Endpoint                       | Auth        | Description                                   |
| ------ | ------------------------------ | ----------- | --------------------------------------------- |
| GET    | `/api/billing/setup`           | Setup token | The plan, the period and what card entry needs |
| PUT    | `/api/billing/setup/selection` | Setup token | Choose plan and period                        |
| POST   | `/api/billing/setup/card`      | Setup token | Save a card, start the trial and sign in. Exists only when card entry is on |

### Common Errors

**401 Unauthorized** — No bearer, an invalid or expired setup token, or any other credential

```json
{
  "error": "Invalid or expired token"
}
```

**409 Conflict** — The organization is no longer `pending`

```json
{
  "error": "Organization is not pending approval",
  "code": "not_pending"
}
```

---

## Get the Setup State

`GET /api/billing/setup`

### Response Fields

| Field               | Type    | Description                                                        |
| ------------------- | ------- | ------------------------------------------------------------------ |
| `organization_name` | string  | The restaurant's name                                              |
| `plan`              | string  | `starter`, `einvoicing` or `full`                                  |
| `period`            | string  | `monthly` or `annual`; `monthly` until one is chosen               |
| `card_entry`        | boolean | `true` only when the cloud has its billing keys                    |
| `wompi`             | object  | Present only when `card_entry` is `true`                           |
| `wompi.public_key`  | string  | The platform's Wompi public key, for tokenizing the card           |
| `wompi.api_url`     | string  | Wompi's API address, e.g. `https://production.wompi.co/v1`         |
| `wompi.terms_url`   | string  | Wompi's terms and conditions                                       |
| `wompi.personal_data_url` | string | Wompi's personal-data authorization                           |
| `wompi.acceptance_token`  | string | Records the acceptance of the terms. Single use                |
| `wompi.personal_data_acceptance_token` | string | Records the personal-data authorization. Single use |

The two acceptance tokens are **fresh on every read and single use**: read the state again before each card attempt. The response never carries a private key.

### Example Response (200 OK) — card entry on

```json
{
  "organization_name": "Restaurante Doña Rosa",
  "plan": "einvoicing",
  "period": "monthly",
  "card_entry": true,
  "wompi": {
    "public_key": "<Wompi public key>",
    "api_url": "https://production.wompi.co/v1",
    "terms_url": "https://wompi.com/assets/files/terminos.pdf",
    "personal_data_url": "https://wompi.com/assets/files/autorizacion-datos.pdf",
    "acceptance_token": "<acceptance token>",
    "personal_data_acceptance_token": "<personal data acceptance token>"
  }
}
```

### Example Response (200 OK) — card entry off

```json
{
  "organization_name": "Restaurante Doña Rosa",
  "plan": "einvoicing",
  "period": "monthly",
  "card_entry": false
}
```

### Error Responses

**502 Bad Gateway** — Card entry is on and Wompi did not answer

```json
{
  "error": "Payment provider unavailable",
  "code": "payment_provider_unavailable"
}
```

Also the [common errors](#common-errors).

---

## Choose Plan and Period

`PUT /api/billing/setup/selection`

Stores both for the organization, which stays `pending`. The amount in the response comes from the [price table](./plans.md); an `amount` in the request is ignored.

### Request Body

| Field    | Type   | Required | Description                       |
| -------- | ------ | -------- | --------------------------------- |
| `plan`   | string | Yes      | `starter`, `einvoicing` or `full` |
| `period` | string | Yes      | `monthly` or `annual`             |

```json
{
  "plan": "einvoicing",
  "period": "annual"
}
```

### Example Response (200 OK)

```json
{
  "plan": "einvoicing",
  "period": "annual",
  "amount": 400000,
  "currency": "COP"
}
```

### Error Responses

**400 Bad Request** — An invalid field, named in `field`. Nothing is stored

```json
{
  "error": "invalid period: must be monthly or annual",
  "field": "period"
}
```

Also the [common errors](#common-errors).

---

## Save a Card

`POST /api/billing/setup/card`

Registered only when card entry is on; otherwise `404`. **Limited to 5 attempts per organization per hour**, refused cards included.

In order, the cloud:

1. Creates a Wompi payment source for the card, with the admin's email. **Nothing is charged.**
2. Stores, together or not at all: the payment source, the card's brand and last four digits, the moment of consent, and the start of the 14-day trial.
3. Answers with a sign-in.

### Request Body

| Field                            | Type   | Required | Description                                             |
| -------------------------------- | ------ | -------- | ------------------------------------------------------- |
| `card_token`                     | string | Yes      | The token Wompi returned for the card (`tok_...`)       |
| `acceptance_token`               | string | Yes      | From the setup state, once the person accepted the terms |
| `personal_data_acceptance_token` | string | Yes      | From the setup state, once the person authorized the data treatment |

```json
{
  "card_token": "<card token from Wompi>",
  "acceptance_token": "<acceptance token>",
  "personal_data_acceptance_token": "<personal data acceptance token>"
}
```

Never send a card number, security code or expiry date to the cloud.

### Example Response (200 OK)

The same body as a successful [`POST /api/auth/signin`](./auth.md#sign-in). The organization is now `trial` with `trial_ends_at` 14 days ahead, and the setup token stops working (`409 not_pending`).

```json
{
  "token": "<sign-in token>",
  "username": "rosa@example.com",
  "roles": [{ "id": 1, "name": "admin" }],
  "permissions": ["orders:read", "orders:create"]
}
```

### Error Responses

Every failure leaves the organization `pending` and stores no card.

**400 Bad Request** — A missing token, named in `field`

```json
{
  "error": "invalid card_token: is required",
  "field": "card_token"
}
```

**402 Payment Required** — Wompi refused the card token, or the payment source is not available

```json
{
  "error": "Card refused",
  "code": "card_refused"
}
```

**404 Not Found** — Card entry is off

**409 Conflict** — `not_pending`: the trial already started, including the loser of two saves sent at the same moment

**429 Too Many Requests** — A sixth attempt within an hour, with `Retry-After` in seconds. Wompi is not contacted

```json
{
  "error": "Too many requests"
}
```

**502 Bad Gateway** — Wompi did not answer

```json
{
  "error": "Payment provider unavailable",
  "code": "payment_provider_unavailable"
}
```

Also the [common errors](#common-errors).
