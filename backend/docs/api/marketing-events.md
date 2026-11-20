# Marketing Events (backend → marketing-app)

This backend tells marketing-app what happens to the orders and reservations its WhatsApp agent took, so marketing-app can report conversions to Meta and message guests. This backend never sends WhatsApp messages itself.

This is an **outbound** contract: marketing-app serves these endpoints and this backend calls them. It is cloud-only; the edge box never records or sends events.

## Delivery

- Each event is written to an outbox **in the same transaction** as the change it reports, so it exists exactly when the change committed.
- An event is recorded only when its order or reservation has a `channel_id` (it came through marketing-app), `MARKETING_APP_EVENTS_URL` is set, and the organization has a signing secret (its [`marketing_app` integration](organization-integrations.md#generate-the-marketing-app-secret)). Otherwise nothing is recorded and the change still succeeds.
- The secret is read when an event is recorded and again on each delivery run, so a secret put, generated or removed takes effect on the next event without a restart. While an organization has no secret, its pending events are left pending (not sent) and expire after 72 hours.
- A sender polls the outbox every 5 seconds and POSTs each due event. Delivery is **at least once**; the same `event_id` and the same bytes are sent on every retry. Events are **not** ordered: deduplicate on `event_id`.

| Response               | What the sender does                                                       |
| ---------------------- | -------------------------------------------------------------------------- |
| `2xx`                  | Delivered; never sent again                                                |
| `400`                  | Rejected as malformed; not retried; logged as an error                     |
| `401`                  | Retried at most hourly; logged as an error (the secret differs on each side, e.g. regenerated and not yet pasted) |
| `404`, `5xx`, timeout, connection error | Retried after 10s, doubling per attempt, capped at 30 minutes |
| Not delivered in 72 hours | Expired; not retried; logged as an error                               |

Each attempt times out after 10 seconds.

## Signature

Every request carries

```
Content-Type: application/json
X-Signature-256: sha256=<hex HMAC-SHA256 of the exact request body>
X-Channel-Id: <the body's channel_id>
```

The signature is keyed by the organization's signing secret at the time of the attempt. `X-Channel-Id` is **not** signed: it only tells marketing-app which secret to verify with. Every row in the outbox gets it, including retries of events recorded before the header existed (it is read from the stored body).

An organization has **one** secret: paste the same value on each of its channels in marketing-app. To verify, marketing-app:

1. with `X-Channel-Id`, loads that channel's secret (unknown channel or no secret → `401`), verifies the HMAC over the raw bytes with a constant-time compare, then parses the body and requires `channel_id` to equal the header, else `401`. A forged header can only make verification fail;
2. without the header, tries each channel's secret as before, so either side can deploy first.

**Rotation.** Generating the secret again ([`POST …/integrations/marketing_app/secret`](organization-integrations.md#generate-the-marketing-app-secret)) signs every later attempt with the new value. Until it is pasted on every channel, marketing-app answers `401`; those events are retried hourly and delivered once it matches, within 72 hours. There is no grace period with two valid secrets.

## Which change produces which event

| Event                   | When                                                                  | Path                      |
| ----------------------- | --------------------------------------------------------------------- | ------------------------- |
| `order.paid`            | An agent order is recorded as paid in the cloud: through its payment link, at the till in the cloud, or applied from the box | `/backend/events` **and** `/backend/notifications` |
| `order.cancelled`       | An agent order is voided, including when it expires unpaid             | `/backend/events`        |
| `order.expired`         | An agent order is voided because nobody paid its link in time          | `/backend/notifications` |
| `reservation.confirmed` | Staff confirm a WhatsApp reservation before its `starts_at`             | `/backend/notifications` |
| `reservation.declined`  | Staff decline it before its `starts_at`, with an optional `reason`      | `/backend/notifications` |
| `reservation.cancelled` | The guest (through the agent) or staff cancel it before its `starts_at`; `cancelled_by` says which | `/backend/notifications` |
| `feedback.requested`    | The morning-after job opens a feedback request                         | `/backend/notifications` |

A request, a visit and a no-show produce no event. A payment records two events, and an expiry two (`order.cancelled` and `order.expired`), each with its own `event_id`.

## `POST <events URL>/backend/events`

marketing-app's existing ad-conversions contract. Unknown orders (ones the agent did not place) should be answered `200` and ignored.

| Field            | Type   | Description                                                      |
| ---------------- | ------ | ---------------------------------------------------------------- |
| `event_id`       | string | UUID, stable across retries                                      |
| `type`           | string | `order.paid` or `order.cancelled`                                |
| `occurred_at`    | string | RFC 3339 with the organization's offset: when it was paid or voided |
| `channel_id`     | string | The channel the order came through                               |
| `order_id`       | string | The id the customer toolset returns                              |
| `customer_phone` | string | E.164 digits without `+`                                         |
| `amount`         | string | Decimal string of the amount paid. `order.paid` only             |
| `currency`       | string | The organization's ISO 4217 code. `order.paid` only              |

`order.paid`:

```json
{
  "event_id": "019a5f10-7c3e-7a21-9b40-3d2e1f0a5b61",
  "type": "order.paid",
  "occurred_at": "2026-11-08T20:30:00-05:00",
  "channel_id": "ch_1",
  "order_id": "019a0000-0000-7000-8000-0000000000d1",
  "customer_phone": "573005551234",
  "amount": "45000",
  "currency": "COP"
}
```

`order.cancelled`:

```json
{
  "event_id": "019a5f10-7c3e-7a21-9b40-3d2e1f0a5b62",
  "type": "order.cancelled",
  "occurred_at": "2026-11-08T20:30:00-05:00",
  "channel_id": "ch_1",
  "order_id": "019a0000-0000-7000-8000-0000000000d1",
  "customer_phone": "573005551234"
}
```

## `POST <events URL>/backend/notifications`

New in this contract. Each event is a message marketing-app sends the guest, so unlike `/backend/events` an unknown `type` should **not** be silently dropped. Until marketing-app serves the path, the backend's events get `404` and are retried until they expire.

| Field             | Type           | Description                                     |
| ----------------- | -------------- | ----------------------------------------------- |
| `event_id`        | string         | UUID, stable across retries                     |
| `type`            | string         | See the table above                             |
| `occurred_at`     | string         | RFC 3339 with the organization's offset         |
| `channel_id`      | string         | The channel to answer on                        |
| `conversation_id` | string \| null | The marketing-app conversation, when known      |
| `customer_phone`  | string         | E.164 digits without `+`                        |
| `data`            | object         | Per type, below                                 |

`data` for `order.paid`: `order_id`, `amount` (decimal string of the amount paid) and `currency` (the organization's ISO 4217 code). For `order.expired`: `order_id`.

`order.paid`:

```json
{
  "event_id": "019a5f13-1a2b-7c3d-8e4f-5a6b7c8d9e01",
  "type": "order.paid",
  "occurred_at": "2026-11-08T20:30:00-05:00",
  "channel_id": "ch_1",
  "conversation_id": "conv_9",
  "customer_phone": "573005551234",
  "data": {
    "order_id": "019a0000-0000-7000-8000-0000000000d1",
    "amount": "45000",
    "currency": "COP"
  }
}
```

`order.expired`:

```json
{
  "event_id": "019a5f13-1a2b-7c3d-8e4f-5a6b7c8d9e02",
  "type": "order.expired",
  "occurred_at": "2026-11-08T20:30:00-05:00",
  "channel_id": "ch_1",
  "conversation_id": "conv_9",
  "customer_phone": "573005551234",
  "data": {
    "order_id": "019a0000-0000-7000-8000-0000000000d1"
  }
}
```

`data` for `reservation.*`: `reservation_id`, `name`, `party_size`, `starts_at`, `status`; plus `reason` when declined with one, and `cancelled_by` (`customer` or `staff`) when cancelled.

`reservation.confirmed`:

```json
{
  "event_id": "019a5f11-0a2b-7c44-8d13-6e5f4a3b2c71",
  "type": "reservation.confirmed",
  "occurred_at": "2026-11-08T15:00:00-05:00",
  "channel_id": "ch_1",
  "conversation_id": "conv_9",
  "customer_phone": "573005551234",
  "data": {
    "reservation_id": "r1",
    "name": "Ana",
    "party_size": 6,
    "starts_at": "2026-11-09T20:00:00-05:00",
    "status": "confirmed"
  }
}
```

`reservation.declined`:

```json
{
  "event_id": "019a5f11-0a2b-7c44-8d13-6e5f4a3b2c72",
  "type": "reservation.declined",
  "occurred_at": "2026-11-08T15:00:00-05:00",
  "channel_id": "ch_1",
  "conversation_id": "conv_9",
  "customer_phone": "573005551234",
  "data": {
    "reservation_id": "r1",
    "name": "Ana",
    "party_size": 6,
    "starts_at": "2026-11-09T20:00:00-05:00",
    "status": "declined",
    "reason": "Fully booked"
  }
}
```

`reservation.cancelled` (by the guest):

```json
{
  "event_id": "019a5f11-0a2b-7c44-8d13-6e5f4a3b2c73",
  "type": "reservation.cancelled",
  "occurred_at": "2026-11-08T15:00:00-05:00",
  "channel_id": "ch_1",
  "conversation_id": null,
  "customer_phone": "573005551234",
  "data": {
    "reservation_id": "r1",
    "name": "Ana",
    "party_size": 6,
    "starts_at": "2026-11-09T20:00:00-05:00",
    "status": "cancelled",
    "cancelled_by": "customer"
  }
}
```

`data` for `feedback.requested`: `feedback_request_id`, `reservation_id`, `name`, `starts_at` of the visit, and `expires_at` of the request (7 days later). The guest answers through the agent's `record_my_feedback` tool.

```json
{
  "event_id": "019a5f12-5d6e-7f01-a2b3-4c5d6e7f8091",
  "type": "feedback.requested",
  "occurred_at": "2026-11-09T11:05:00-05:00",
  "channel_id": "ch_1",
  "conversation_id": "conv_9",
  "customer_phone": "573005551234",
  "data": {
    "feedback_request_id": "fr1",
    "reservation_id": "r1",
    "name": "Ana",
    "starts_at": "2026-11-08T20:00:00-05:00",
    "expires_at": "2026-11-16T11:05:00-05:00"
  }
}
```

## Responses marketing-app should give

| Status | Meaning                                                     |
| ------ | ----------------------------------------------------------- |
| `200`  | Done, or deliberately ignored (including a duplicate `event_id`) |
| `400`  | The body is malformed; it will not be sent again            |
| `401`  | The signature did not match                                 |
| `5xx`  | Try again later                                             |

## Configuration (cloud)

| Env var                              | Required | Description |
| ------------------------------------ | -------- | ----------- |
| `MARKETING_APP_EVENTS_URL`           | No       | marketing-app's base URL, e.g. `https://marketing.example.com`. Unset: nothing is recorded or sent |
| `INTEGRATION_SECRETS_KEY`            | Yes, to send events | Encrypts the organizations' integrations, where each signing secret lives. Unset: no organization records events |
| `MARKETING_APP_EVENTS_SECRET_<SLUG>` | —        | **Ignored.** Startup warns once per variable still set, by name, never the value. The secret is the organization's `marketing_app` integration |
| `RESERVATION_VISITED_CRON`           | No       | When confirmed reservations are marked visited; default `*/15 * * * *` |
| `FEEDBACK_REQUEST_CRON`              | No       | When feedback requests are opened; default `5 * * * *` |

At startup, while the URL is set, a warning names each active organization without a secret; its agent orders and reservations record no events until its admin generates one (Ajustes → Integraciones) or the operator puts one.

**Rollout (moving a secret out of the environment).**

1. Deploy the backend with `INTEGRATION_SECRETS_KEY` set. Startup warns that `MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA` is ignored and that Laguna has no secret. Until step 2, Laguna records no events, and those are lost.
2. Right away, `PUT /api/platform/organizations/<laguna>/integrations/marketing_app` with the current value (at least 32 characters; if it is shorter, generate a new one and paste it in marketing-app instead). Confirm the next agent event is delivered (`2xx`).
3. Remove `MARKETING_APP_EVENTS_SECRET_*` from the deployment.
4. Deploy marketing-app's `X-Channel-Id` verification, any time. It is required before onboarding an organization with more than one channel.
5. Optional: regenerate from Ajustes and paste the new secret on Laguna's channel, to prove rotation.

**Rollback:** revert the backend and restore the environment variable; integration rows are ignored by the old code. To stop sending altogether, unset the URL; the outbox keeps its rows.
