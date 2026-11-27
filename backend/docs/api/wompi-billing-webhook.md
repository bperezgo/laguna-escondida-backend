# Wompi Billing Webhook

Wompi tells the cloud about every transaction on **the platform's own Wompi account**, the one restaurants pay their plan to. An approved transaction for a subscription charge makes that charge `approved` and gives the organization its paid period; see [Billing → Charges](./billing.md#charges).

This is not the [per-organization webhook](./wompi-webhook.md), which receives the events of each restaurant's own Wompi account for its payment links.

- **Cloud only.** The edge does not serve this route.
- **Public.** No credential: the event's checksum, made with the platform's events secret (`BILLING_WOMPI_EVENTS_SECRET`), proves it is Wompi's.
- **Exists only when charging is on.** While the cloud has no events secret the route answers `404`. See [Billing → Charging](./billing.md#charging).

## Setting the address in Wompi

In the platform's Wompi dashboard, under *Desarrolladores → Eventos* (the field is *URL de eventos*), set:

```
https://<cloud address>/api/webhooks/wompi-billing
```

Wompi keeps one address per environment: set the sandbox address in the sandbox account and the production address in the production account. The events secret (`test_events_…` or `prod_events_…`) and the integrity secret are on the same page, under *Secretos para integración técnica*; they go in `BILLING_WOMPI_EVENTS_SECRET` and `BILLING_WOMPI_INTEGRITY_SECRET`, and must be of the same environment as the billing keys.

## Endpoints

| Method | Endpoint                        | Auth           | Description             |
| ------ | ------------------------------- | -------------- | ----------------------- |
| POST   | `/api/webhooks/wompi-billing`   | event checksum | Receive one Wompi event |

## Receive an Event

`POST /api/webhooks/wompi-billing`

### Request Body

Wompi's event, as Wompi sends it. The fields read:

| Field                                 | Type     | Description |
| ------------------------------------- | -------- | ----------- |
| `event`                               | string   | Only `transaction.updated` changes anything |
| `data.transaction.id`                 | string   | Wompi's transaction id |
| `data.transaction.reference`          | string   | The charge's reference, `kb-<organization id>-<due date as yyyymmdd>-<attempt>` |
| `data.transaction.status`             | string   | `APPROVED`, `DECLINED`, `VOIDED` or `ERROR` (`PENDING` changes nothing) |
| `data.transaction.amount_in_cents`    | number   | Must equal the charge's amount × 100 for an approval to apply |
| `data.transaction.currency`           | string   | Must equal the charge's currency (`COP`) for an approval to apply |
| `signature.properties`                | string[] | The properties the checksum covers, read from `data` |
| `signature.checksum`                  | string   | SHA-256, hex, upper or lower case |
| `timestamp`                           | number   | Part of the checksum |

```json
{
  "event": "transaction.updated",
  "data": {
    "transaction": {
      "id": "1234-1610641025-49201",
      "amount_in_cents": 4000000,
      "reference": "kb-550e8400-e29b-41d4-a716-446655440030-20261215-1",
      "customer_email": "rosa@example.com",
      "currency": "COP",
      "payment_method_type": "CARD",
      "status": "APPROVED"
    }
  },
  "environment": "prod",
  "signature": {
    "properties": ["transaction.id", "transaction.status", "transaction.amount_in_cents"],
    "checksum": "<sha256 hex>"
  },
  "timestamp": 1530291411,
  "sent_at": "2026-12-15T14:02:45.000Z"
}
```

### The checksum

The SHA-256 of, concatenated with no separator: the value of each property named in `signature.properties`, in that order, then `timestamp`, then the platform's events secret. The properties are read from the event, never assumed.

### What an event does

| Event | Result |
| ----- | ------ |
| Wrong or missing checksum, a body that is not JSON, or over 1 MB | `401`, nothing changes |
| Signed, not `transaction.updated` (e.g. `nequi_token.updated`) | `200`, nothing changes |
| Signed, `reference` matches no charge | `200`, nothing changes |
| Signed, charge already settled | `200`, nothing changes: each transaction is applied at most once |
| Signed `APPROVED`, amount and currency equal the charge's | `200`; the charge is `approved`, the organization is paid, and its users and boxes are accepted within a minute |
| Signed `APPROVED`, amount or currency differ | `200`; the charge stays `pending`, the organization is unchanged, and an error is logged naming the organization and both amounts |
| Signed `DECLINED`, `VOIDED` or `ERROR` | `200`; the charge takes that status and the next retry follows its schedule |
| Signed, but the cloud could not store the result | `500`, so Wompi sends it again |

### Example Response (200 OK)

```json
{
  "status": "ok"
}
```

### Error Responses

**401 Unauthorized** — The event is not verifiably Wompi's

```json
{
  "error": "invalid event"
}
```

**404 Not Found** — The cloud has no events secret

**500 Internal Server Error** — A verified event could not be applied; Wompi retries after 30 minutes, 3 hours and 24 hours

```json
{
  "error": "event not applied"
}
```

## When no event arrives

Nothing depends on the event alone: a charge still `pending` 10 minutes after its creation is looked up at Wompi every 5 minutes and settled by the same rules.
