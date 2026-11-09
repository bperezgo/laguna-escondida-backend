# Marketing Events (backend → marketing-app)

This backend tells marketing-app what happens to the orders and reservations its WhatsApp agent took, so marketing-app can report conversions to Meta and message guests. This backend never sends WhatsApp messages itself.

This is an **outbound** contract: marketing-app serves these endpoints and this backend calls them. It is cloud-only; the edge box never records or sends events.

## Delivery

- Each event is written to an outbox **in the same transaction** as the change it reports, so it exists exactly when the change committed.
- An event is recorded only when its order or reservation has a `channel_id` (it came through marketing-app), `MARKETING_APP_EVENTS_URL` is set, and the organization has a signing secret. Otherwise nothing is recorded and the change still succeeds.
- A sender polls the outbox every 5 seconds and POSTs each due event. Delivery is **at least once**; the same `event_id` and the same bytes are sent on every retry. Events are **not** ordered: deduplicate on `event_id`.

| Response               | What the sender does                                                       |
| ---------------------- | -------------------------------------------------------------------------- |
| `2xx`                  | Delivered; never sent again                                                |
| `400`                  | Rejected as malformed; not retried; logged as an error                     |
| `401`                  | Retried at most hourly; logged as an error (the secret differs on each side) |
| `404`, `5xx`, timeout, connection error | Retried after 10s, doubling per attempt, capped at 30 minutes |
| Not delivered in 72 hours | Expired; not retried; logged as an error                               |

Each attempt times out after 10 seconds.

## Signature

Every request carries

```
Content-Type: application/json
X-Signature-256: sha256=<hex HMAC-SHA256 of the exact request body>
```

keyed by the organization's signing secret. Verify it over the raw bytes before parsing. An organization with several channels uses the same secret on each of them in marketing-app.

## Which change produces which event

| Event                   | When                                                                  | Path                      |
| ----------------------- | --------------------------------------------------------------------- | ------------------------- |
| `order.paid`            | An agent order is recorded as paid in the cloud, paid there or applied from the box | `/backend/events`        |
| `order.cancelled`       | An agent order is voided                                               | `/backend/events`        |
| `reservation.confirmed` | Staff confirm a WhatsApp reservation before its `starts_at`             | `/backend/notifications` |
| `reservation.declined`  | Staff decline it before its `starts_at`, with an optional `reason`      | `/backend/notifications` |
| `reservation.cancelled` | The guest (through the agent) or staff cancel it before its `starts_at`; `cancelled_by` says which | `/backend/notifications` |
| `feedback.requested`    | The morning-after job opens a feedback request                         | `/backend/notifications` |

A request, a visit and a no-show produce no event.

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
| `MARKETING_APP_EVENTS_SECRET_<SLUG>` | Per organization | The organization's signing secret. `<SLUG>` is its slug upper-cased with `-` as `_`: `laguna-escondida` → `MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA` |
| `RESERVATION_VISITED_CRON`           | No       | When confirmed reservations are marked visited; default `*/15 * * * *` |
| `FEEDBACK_REQUEST_CRON`              | No       | When feedback requests are opened; default `5 * * * *` |

At startup, while the URL is set, a warning names each active organization without a secret; its agent orders and reservations record no events.

**Rollout.** Deploy with the URL unset: reservations work and nothing is sent. Once marketing-app serves both paths, set the URL and each organization's secret. **Rollback:** unset the URL; sending stops and the outbox keeps its rows.
