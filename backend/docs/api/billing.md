# Billing API

Three groups of routes, all cloud only:

| Group | Routes | Who | Credential |
| ----- | ------ | --- | ---------- |
| [Setup](#endpoints) | `/api/billing/setup/**` | A new restaurant's admin, between signing up and the trial | Setup token, organization `pending` |
| [Subscription](#subscription) | `/api/billing`, `/api/billing/card-setup`, `/api/billing/card`, `/api/billing/cancel`, `/api/billing/resume` | An admin, from inside the app | Sign-in token with `billing:manage` |
| [Recovery](#recovery) | `/api/billing/recovery/**` | The admin of an organization stopped for an ended trial or a missed payment | Setup token, organization stopped |

How and when a subscription is charged is in [Charges](#charges). Wompi's events arrive at the [billing webhook](./wompi-billing-webhook.md).

# Billing Setup

What a new restaurant's admin does between signing up and the trial: read the setup state, choose plan and period, and leave a card. Leaving a card starts the trial and signs the person in.

- **Cloud only.** None of these routes exist on an edge box.
- **Auth: a setup token**, sent as `Authorization: Bearer <setup_token>`. Nothing else is accepted: a sign-in token, an organization API key and the platform key all get `401`.
- **Only while the organization is `pending`.** Once its trial started (by a saved card or by the operator), every route answers `409 not_pending`.
- **No card data reaches the cloud.** The browser sends the card to Wompi and the cloud receives Wompi's token for it.

## The setup token

A JWT with audience `billing-setup` that names the admin and the organization and carries no role or permission. It expires **60 minutes** after it is issued. It is returned in two places:

- the `201` of [`POST /api/signup/google`](./signup.md#sign-up-with-google), as `setup_token`;
- the `403 pending_approval`, `403 trial_expired` and `403 payment_required` of a sign-in that would otherwise have succeeded ([`POST /api/auth/signin`](./auth.md#sign-in), [`POST /api/auth/google`](./auth.md#sign-in-with-google)), as `setup_token`, **only for a user with the admin role**.

The same token opens two groups of routes, by the organization's state when the request arrives:

| Organization | `/api/billing/setup/**` | `/api/billing/recovery/**` |
| ------------ | ----------------------- | -------------------------- |
| `pending` | works | `409 not_stopped` |
| stopped (trial ended, or payment missed) | `409 not_pending` | works |
| working, or `suspended` | `409 not_pending` | `409 not_stopped` (see [the recovery state](#get-the-recovery-state) for the one exception) |

Every other route refuses a setup token with the same `401` as an invalid token.

## Card entry

Card entry is on only when the cloud is configured with the platform's own Wompi keys: `BILLING_WOMPI_PUBLIC_KEY` and `BILLING_WOMPI_PRIVATE_KEY`. Both must be set, and both of the same environment (`pub_test_`/`prv_test_` or `pub_prod_`/`prv_prod_`); one without the other, or a mixed pair, stops the cloud at startup. The Wompi address is `WOMPI_API_URL`. These are not an organization's [Wompi integration](./organization-integrations.md).

With card entry off, the state has `card_entry: false`, the card routes answer `404`, and the operator starts the trial with [`POST /api/platform/organizations/:id/start-trial`](./organizations.md#start-a-trial), which works with or without a saved card.

## Charging

Charging is on only when the cloud also has `BILLING_WOMPI_EVENTS_SECRET` and `BILLING_WOMPI_INTEGRITY_SECRET`, the two secrets of the same Wompi account (*Desarrolladores → Secretos para integración técnica*).

| Set | Card entry | Charging |
| --- | ---------- | -------- |
| Nothing | off | off |
| Public and private key | on | off: cards are saved and nothing is ever charged |
| All four | on | on |
| Any other combination, or secrets of another environment than the keys (`test_events_`/`test_integrity_` go with test keys, `prod_…` with production keys) | — | the cloud stops at startup |

With charging off there is no charge job, [`POST /api/billing/recovery/pay`](#pay) and the [billing webhook](./wompi-billing-webhook.md) answer `404`, and the recovery state has `card_entry: false`.

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

---

# Charges

A **subscription** is an organization with a saved card. Only a subscription that is not cancelled is ever charged. An organization without a card, or one the operator set to `active` by hand, is never charged and never stops for lack of payment.

- **Due date.** The trial's end while the organization is `trial`; the end of the paid period (`paid_until`) once it has paid.
- **Amount.** The [price](./plans.md) of the plan and period at the moment of the charge, in pesos, no tax.
- **First charge.** Within an hour of the trial's end. Approved: the organization becomes `active`, paid from the trial's end for one month or one year.
- **Renewal.** On the paid period's last instant. Approved: the period extends one month or one year **from the previous due date**, however late the approval.
- **Retries.** A declined, voided or failed charge is tried again one day and three days after the due date: three charges at most. No charge is created while another is `pending`.
- **Grace.** The organization keeps working for three days after the due date while its subscription is not cancelled, then stops: `trial_expired` for a trial, `payment_required` for a paid one (see [Organization access refusals](./auth.md#organization-access-refusals)). A cancelled subscription stops at the due date, with no grace.
- **Never twice.** At most one approved and one pending charge per due date, whichever replica runs the job and whatever crashes.
- **Settlement.** A charge is `pending` until Wompi reports `approved`, `declined`, `voided` or `error`, by [event](./wompi-billing-webhook.md) or by lookup. An approval is applied only when Wompi's amount and currency equal the charge's. A charge still pending 10 minutes after its creation is looked up at Wompi every 5 minutes; one still pending after 24 hours is logged as an error and counts as failed.

### Charge statuses

| Status | Meaning |
| ------ | ------- |
| `pending` | Sent to Wompi, no final answer yet |
| `approved` | Paid; `covers_from` and `covers_until` say the period it paid |
| `declined` | The bank or Wompi refused it |
| `voided` | Annulled at Wompi |
| `error` | Wompi failed to process it, never took it, or gave no final answer in 24 hours |

---

# Subscription

What an admin sees and does about the subscription from inside the app.

- **Auth:** a sign-in token of a user with the `billing:manage` permission, which only the **admin** role holds. Anyone else gets `403`.
- Core routes: they work on every plan.

| Method | Endpoint                  | Description |
| ------ | ------------------------- | ----------- |
| GET    | `/api/billing`            | The subscription and its charges |
| GET    | `/api/billing/card-setup` | What the browser needs to send a card to Wompi |
| PUT    | `/api/billing/card`       | Replace the saved card. Exists only when card entry is on |
| POST   | `/api/billing/cancel`     | Cancel the subscription |
| POST   | `/api/billing/resume`     | Remove the cancellation |

### Common Errors

**401 Unauthorized** — No or invalid sign-in token; a setup token is not one

**403 Forbidden** — The user lacks `billing:manage`

```json
{
  "error": "Insufficient permissions",
  "required": "billing:manage"
}
```

Also the [organization access refusals](./auth.md#organization-access-refusals).

---

## Get the Subscription

`GET /api/billing`

### Response Fields

| Field                | Type            | Description |
| -------------------- | --------------- | ----------- |
| `plan`               | string          | `starter`, `einvoicing` or `full` |
| `period`             | string          | `monthly` or `annual` |
| `status`             | string          | The organization's status: `trial` or `active` |
| `trial_ends_at`      | string \| null  | RFC 3339 |
| `paid_until`         | string \| null  | End of the paid period; `null` until the first approved charge |
| `due_date`           | string \| null  | The trial's end during the trial, `paid_until` after. `null` for an organization activated by hand |
| `next_charge_amount` | number \| null  | Pesos to be charged on `due_date`. `null` when nothing will be charged: no card, cancelled, or activated by hand |
| `currency`           | string          | `COP` |
| `cancelled`          | boolean         | The subscription is cancelled; it works until `due_date` |
| `payment_failed`     | boolean         | The latest charge for the current due date failed and the organization still works |
| `access_ends_at`     | string \| null  | When the organization stops unless a charge is approved: `due_date` plus three days, or `due_date` itself when cancelled or without a card. `null` when it never stops |
| `card`               | object \| null  | `brand` and `last_four` of the saved card |
| `latest_charge`      | object \| null  | The newest charge, same shape as `charges[]` |
| `charges`            | array           | Charges made, newest first |
| `charges[].id`       | string          | Charge id |
| `charges[].date`     | string          | When the charge was created |
| `charges[].amount`   | number          | Pesos |
| `charges[].currency` | string          | `COP` |
| `charges[].status`   | string          | See [charge statuses](#charge-statuses) |
| `charges[].covers_from`, `charges[].covers_until` | string \| null | The period an approved charge paid |
| `card_entry`         | boolean         | Whether a card can be saved or changed |

The payment source's identifier is never returned.

### Example Response (200 OK) — during the trial

```json
{
  "plan": "einvoicing",
  "period": "monthly",
  "status": "trial",
  "trial_ends_at": "2026-12-15T14:00:00Z",
  "paid_until": null,
  "due_date": "2026-12-15T14:00:00Z",
  "next_charge_amount": 40000,
  "currency": "COP",
  "cancelled": false,
  "payment_failed": false,
  "access_ends_at": "2026-12-18T14:00:00Z",
  "card": { "brand": "VISA", "last_four": "4242" },
  "latest_charge": null,
  "charges": [],
  "card_entry": true
}
```

### Example Response (200 OK) — paid, with a renewal declined yesterday

```json
{
  "plan": "einvoicing",
  "period": "monthly",
  "status": "active",
  "trial_ends_at": "2026-12-15T14:00:00Z",
  "paid_until": "2027-01-15T14:00:00Z",
  "due_date": "2027-01-15T14:00:00Z",
  "next_charge_amount": 40000,
  "currency": "COP",
  "cancelled": false,
  "payment_failed": true,
  "access_ends_at": "2027-01-18T14:00:00Z",
  "card": { "brand": "VISA", "last_four": "4242" },
  "latest_charge": {
    "id": "019b1f2e-7c1a-7b3e-9d55-2f8f0a6c1e21",
    "date": "2027-01-15T14:03:10Z",
    "amount": 40000,
    "currency": "COP",
    "status": "declined",
    "covers_from": null,
    "covers_until": null
  },
  "charges": [
    {
      "id": "019b1f2e-7c1a-7b3e-9d55-2f8f0a6c1e21",
      "date": "2027-01-15T14:03:10Z",
      "amount": 40000,
      "currency": "COP",
      "status": "declined",
      "covers_from": null,
      "covers_until": null
    },
    {
      "id": "019a7d10-4a52-7e0c-8a11-6f1d2c3b4a59",
      "date": "2026-12-15T14:02:41Z",
      "amount": 40000,
      "currency": "COP",
      "status": "approved",
      "covers_from": "2026-12-15T14:00:00Z",
      "covers_until": "2027-01-15T14:00:00Z"
    }
  ],
  "card_entry": true
}
```

### Example Response (200 OK) — no saved card

The same body with no card, no next charge and no charges.

```json
{
  "plan": "full",
  "period": "monthly",
  "status": "active",
  "trial_ends_at": null,
  "paid_until": null,
  "due_date": null,
  "next_charge_amount": null,
  "currency": "COP",
  "cancelled": false,
  "payment_failed": false,
  "access_ends_at": null,
  "card": null,
  "latest_charge": null,
  "charges": [],
  "card_entry": true
}
```

---

## Get the Card Setup

`GET /api/billing/card-setup`

What the browser needs to send a card to Wompi, as the [setup state](#get-the-setup-state) gives it. The acceptance tokens are fresh on every read and single use.

### Example Response (200 OK)

```json
{
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

With card entry off: `{"card_entry": false}`.

### Error Responses

**502 Bad Gateway** — `payment_provider_unavailable`: Wompi did not answer

---

## Change the Card

`PUT /api/billing/card`

Creates a new payment source and replaces the saved one, under the rules of [saving the first card](#save-a-card): the same body, the same failures, and the **same limit of 5 attempts per organization per hour, counted together** with `POST /api/billing/setup/card` and `POST /api/billing/recovery/pay`. It starts no trial.

When the due date has passed and no charge was approved for it, **one charge is created at once** with the new card, outside the three scheduled attempts. If a charge is already pending, none is added.

### Request Body

```json
{
  "card_token": "<card token from Wompi>",
  "acceptance_token": "<acceptance token>",
  "personal_data_acceptance_token": "<personal data acceptance token>"
}
```

### Response (200 OK)

The same body as [`GET /api/billing`](#get-the-subscription), with the new card and, when one was created, the new charge as `latest_charge`.

### Error Responses

The previous card stays the saved one on every failure.

**400 Bad Request** — A missing token, named in `field`

**402 Payment Required** — `card_refused`: Wompi refused the new card

```json
{
  "error": "Card refused",
  "code": "card_refused"
}
```

**404 Not Found** — Card entry is off

**429 Too Many Requests** — A sixth card attempt within an hour, with `Retry-After`

**502 Bad Gateway** — `payment_provider_unavailable`

---

## Cancel the Subscription

`POST /api/billing/cancel`

No body. Marks the subscription cancelled: no charge is created from now on, the organization works until its `due_date` and then stops with no grace, and the saved card is kept. Cancelling twice changes nothing.

### Response (200 OK)

The same body as [`GET /api/billing`](#get-the-subscription), with `cancelled: true`, `next_charge_amount: null` and `access_ends_at` equal to `due_date`.

### Error Responses

**409 Conflict** — The organization has no saved card

```json
{
  "error": "There is no saved card",
  "code": "no_subscription"
}
```

---

## Resume the Subscription

`POST /api/billing/resume`

No body. Removes the cancellation while the due date has not passed; the next charge is then made on the due date as if it had never been cancelled. Resuming a subscription that is not cancelled changes nothing.

### Response (200 OK)

The same body as [`GET /api/billing`](#get-the-subscription), with `cancelled: false`.

### Error Responses

**409 Conflict** — The due date has passed. The organization is stopped: it pays its way back through [recovery](#recovery)

```json
{
  "error": "The subscription has lapsed",
  "code": "subscription_lapsed"
}
```

**409 Conflict** — `no_subscription`: the organization has no saved card

---

# Recovery

How the admin of a stopped organization pays its way back. A sign-in refused with `trial_expired` or `payment_required` gives an admin a [setup token](#the-setup-token); with it, and with nothing else, these routes work.

| Method | Endpoint                          | Description |
| ------ | --------------------------------- | ----------- |
| GET    | `/api/billing/recovery`           | What is owed; after the payment is approved, the sign-in |
| PUT    | `/api/billing/recovery/selection` | Change plan and period |
| POST   | `/api/billing/recovery/pay`       | Pay with the saved card or a new one. Exists only when [charging](#charging) is on |

### Common Errors

**401 Unauthorized** — No bearer, an invalid or expired setup token, or any other credential

**409 Conflict** — The organization is working, `pending` or `suspended`

```json
{
  "error": "Organization is not stopped",
  "code": "not_stopped"
}
```

---

## Get the Recovery State

`GET /api/billing/recovery`

While the organization is stopped it answers what is owed. The page polls it while a charge is pending.

**Once the recovery charge is approved** the organization works again, and this route answers, **once**, the same body as a successful [`POST /api/auth/signin`](./auth.md#sign-in) instead of the state: that is what signs the person in. It does so only for a payment approved within the last hour. Every later call, and any call for an organization that works for another reason, answers `409 not_stopped`: send the person to sign in.

### Response Fields — stopped

| Field               | Type           | Description |
| ------------------- | -------------- | ----------- |
| `organization_name` | string         | The restaurant's name |
| `reason`            | string         | `trial_expired` or `payment_required` |
| `plan`              | string         | `starter`, `einvoicing` or `full` |
| `period`            | string         | `monthly` or `annual` |
| `amount_due`        | number         | Pesos, from the [price table](./plans.md) |
| `currency`          | string         | `COP` |
| `card`              | object \| null | `brand` and `last_four` of the saved card |
| `latest_charge`     | object \| null | The newest charge (see [charges](#get-the-subscription)); `pending` while Wompi has not answered |
| `card_entry`        | boolean        | `true` only when [charging](#charging) is on. When `false`, show the reason and the support contact only |
| `wompi`             | object         | Present when `card_entry` is `true`; as in the [setup state](#get-the-setup-state) |

### Example Response (200 OK) — stopped, with a card on file

```json
{
  "organization_name": "Restaurante Doña Rosa",
  "reason": "payment_required",
  "plan": "einvoicing",
  "period": "monthly",
  "amount_due": 40000,
  "currency": "COP",
  "card": { "brand": "VISA", "last_four": "4242" },
  "latest_charge": {
    "id": "019b1f2e-7c1a-7b3e-9d55-2f8f0a6c1e21",
    "date": "2027-01-18T14:04:02Z",
    "amount": 40000,
    "currency": "COP",
    "status": "declined",
    "covers_from": null,
    "covers_until": null
  },
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

### Example Response (200 OK) — the payment was approved

```json
{
  "token": "<sign-in token>",
  "username": "rosa@example.com",
  "roles": [{ "id": 2, "name": "admin" }],
  "permissions": ["orders:read", "billing:manage"]
}
```

Tell the two apart by `token`.

### Error Responses

**409 Conflict** — `not_stopped`

**502 Bad Gateway** — `payment_provider_unavailable`

---

## Change Plan and Period (Recovery)

`PUT /api/billing/recovery/selection`

The same request, response and `400` as [`PUT /api/billing/setup/selection`](#choose-plan-and-period), for an organization that is stopped. The next charge uses the new plan and period.

### Error Responses

**400 Bad Request** — An invalid `plan` or `period`, named in `field`

**409 Conflict** — `not_stopped`

---

## Pay

`POST /api/billing/recovery/pay`

Creates one charge at once for the plan and period. Counted in the shared limit of 5 card attempts per organization per hour.

### Request Body

Empty, or `{}`, to pay with the saved card. To pay with a new card, the three fields of [saving a card](#save-a-card); the card is saved first and replaces the previous one.

```json
{
  "card_token": "<card token from Wompi>",
  "acceptance_token": "<acceptance token>",
  "personal_data_acceptance_token": "<personal data acceptance token>"
}
```

### Example Response (202 Accepted)

The charge as Wompi first answered it, usually `pending`. Poll [the recovery state](#get-the-recovery-state) for the outcome.

```json
{
  "charge": {
    "id": "019b2a44-0d1e-7c2b-b0a1-5e6f7a8b9c0d",
    "date": "2027-01-25T16:20:11Z",
    "amount": 40000,
    "currency": "COP",
    "status": "pending",
    "covers_from": null,
    "covers_until": null
  }
}
```

When the charge is approved the organization becomes `active`, paid for one month or one year **from the approval**, and its cancellation, if any, is removed. When it is declined the organization stays stopped and the state shows the declined charge; pay again with another card.

### Error Responses

**400 Bad Request** — No saved card and no new one

```json
{
  "error": "invalid card_token: is required: there is no saved card",
  "field": "card_token"
}
```

**402 Payment Required** — `card_refused`: Wompi refused the new card. Nothing is charged

**404 Not Found** — Charging is off

**409 Conflict** — A charge is still pending

```json
{
  "error": "A charge is pending",
  "code": "charge_pending"
}
```

**409 Conflict** — `not_stopped`

**429 Too Many Requests** — A sixth card attempt within an hour, with `Retry-After`

**502 Bad Gateway** — `payment_provider_unavailable`
