# Wompi Webhook

Wompi tells the cloud about every transaction on an organization's payment links. An approved transaction for an agent order's active link pays that order; see [Orders → Payment links](./orders.md#payment-links-agent-orders) for the link lifecycle.

- **Cloud only.** The edge does not serve this route (`404`), creates no links and receives no events.
- **Public.** No credential: the event's checksum, made with the organization's events secret, proves it is Wompi's.
- **One URL per organization.** Each organization's Wompi account is set, in Wompi's dashboard (*Desarrolladores → Eventos*), to its own URL, which names the organization.

## Endpoints

| Method | Endpoint                                  | Auth            | Description                  |
| ------ | ----------------------------------------- | --------------- | ---------------------------- |
| POST   | `/api/webhooks/wompi/:organization_id`    | event checksum  | Receive one Wompi event      |

## Receive an Event

`POST /api/webhooks/wompi/:organization_id`

### Path Parameters

| Param           | Type   | Description              |
| --------------- | ------ | ------------------------ |
| organization_id | string | The organization's UUID  |

### Request Body

Wompi's event, as Wompi sends it. The fields read:

| Field                                   | Type     | Description |
| --------------------------------------- | -------- | ----------- |
| `event`                                 | string   | Only `transaction.updated` is acted on; any other signed event is answered `200` and ignored |
| `data.transaction.id`                   | string   | Wompi's transaction id; each transaction is applied at most once |
| `data.transaction.status`               | string   | `APPROVED`, `PENDING`, `DECLINED`, `VOIDED` or `ERROR` |
| `data.transaction.amount_in_cents`      | integer  | Must equal the link's amount for an approved transaction to pay |
| `data.transaction.currency`             | string   | Must equal the link's currency |
| `data.transaction.payment_method_type`  | string   | Sets the bill's payment method: `CARD` → `credit_card`, `PSE` → `transfer_debit_bank`, any other → `transfer_debit_interbank` |
| `data.transaction.payment_link_id`      | string   | The Wompi link the transaction paid |
| `signature.properties`                  | string[] | The `data` fields the checksum covers, in order |
| `signature.checksum`                    | string   | Hex SHA-256 of those fields' values, then `timestamp`, then the events secret, concatenated |
| `timestamp`                             | integer  | UNIX seconds, part of the checksum |

```json
{
  "event": "transaction.updated",
  "data": {
    "transaction": {
      "id": "1234-1610641025-49201",
      "amount_in_cents": 4500000,
      "reference": "MZQ3X2DE2SMX",
      "currency": "COP",
      "payment_method_type": "CARD",
      "status": "APPROVED",
      "payment_link_id": "3Z0Cfi"
    }
  },
  "environment": "test",
  "signature": {
    "properties": ["transaction.id", "transaction.status", "transaction.amount_in_cents"],
    "checksum": "3476DDA50F64CD7CBD160689640506FEBEA93239BC524FC0469B2C68A3CC8BD0"
  },
  "timestamp": 1530291411,
  "sent_at": "2026-11-18T17:00:00.000Z"
}
```

The checksum above is illustrative; [the curl example](../examples/curl-examples.md#payment-links-and-the-wompi-webhook-cloud-only) computes a real one.

### What happens

| Event | Result |
| ----- | ------ |
| `APPROVED`, for the order's **active** link, the order unpaid and not voided, same amount and currency | The order is paid exactly as `pay-order` pays it: bill (no fiscal customer), e-invoice queued, lines released to the kitchen, `order.paid` recorded |
| `APPROVED`, anything else (replaced or expired link, paid or voided order, another amount, a link that is not ours) | Nothing is paid. The transaction is stored as **unmatched** and logged as an error: staff refund it in Wompi |
| `PENDING` | Stored. The order's expiry waits until the transaction resolves |
| `DECLINED`, `VOIDED`, `ERROR` | Stored; the order and its link are unchanged until the link expires |
| The same transaction again with a status already stored | Nothing |

Every transaction received is kept in `payment_link_transactions`, with the raw event.

### Responses

**200 OK** — applied, stored, or deliberately ignored

```json
{ "status": "ok" }
```

**401 Unauthorized** — the checksum does not match, the organization is unknown or has no payment links configured, or the body is not a Wompi event. Nothing changes; logged as a warning.

```json
{ "error": "invalid event" }
```

**500 Internal Server Error** — a verified event could not be applied (e.g. the database is down). Wompi retries anything but `200` up to three times: 30 minutes, 3 hours and 24 hours later.

```json
{ "error": "event not applied" }
```

## Configuration (cloud)

| Env var                         | Required          | Description |
| ------------------------------- | ----------------- | ----------- |
| `WOMPI_API_URL`                 | No                | `https://sandbox.wompi.co/v1` or `https://production.wompi.co/v1` (default) |
| `WOMPI_PRIVATE_KEY_<SLUG>`      | Per organization  | Creates and deactivates the organization's links. `<SLUG>` is the slug upper-cased with `-` as `_` |
| `WOMPI_EVENTS_SECRET_<SLUG>`    | Per organization  | Verifies the organization's events (*Secretos para integración técnica* in Wompi's dashboard) |
| `PAYMENT_LINK_TTL`              | No                | How long a link lives, default `30m`. Shortened only for tests |

An organization has payment links configured only with **both** its key and its secret; without them its agent orders are refused with `422 PAYMENT_LINKS_NOT_CONFIGURED`. At startup the cloud logs, for each active organization, whether it has them.
