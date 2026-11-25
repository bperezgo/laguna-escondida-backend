# Payment Settings API

How the calling organization's agent orders (WhatsApp takeaway and delivery) are paid: confirmed by staff after a transfer (`manual`), or by Wompi (`wompi`). See [Orders → Payment strategies](./orders.md#payment-strategies-agent-orders).

**Cloud only.** These routes exist only in the cloud binary; on the edge binary they return `404`.

**Tenant-scoped.** Every request acts on the organization of the caller's token. There is no organization id in the path.

## Endpoints

| Method | Endpoint                          | Description                  | Permission            |
| ------ | --------------------------------- | ---------------------------- | --------------------- |
| GET    | `/api/organization/payment-settings` | Read the payment settings | `integrations:read`   |
| PUT    | `/api/organization/payment-settings` | Replace the payment settings | `integrations:manage` |

Both permissions are admin-only today.

## Model

| Field                   | Type    | Description |
| ----------------------- | ------- | ----------- |
| `strategy`              | string  | `manual` or `wompi`. An organization that never saved any is `wompi` |
| `manual_instructions`   | string  | What the customer is told to do to pay by transfer (key, account holder). At most 1000 characters |
| `manual_window_minutes` | integer | How long an unpaid manual order waits before it is cancelled. 10 to 1440, default 60 |

The strategy applies to orders created after it is saved: an order keeps the one it was created under. Choosing `wompi` keeps the stored instructions and window.

## Get Payment Settings

```
GET /api/organization/payment-settings
```

### Example Response (200 OK)

```json
{
  "strategy": "manual",
  "manual_instructions": "Llave Bre-B 3001234567, a nombre de Laguna Escondida SAS",
  "manual_window_minutes": 60
}
```

An organization that never saved any:

```json
{ "strategy": "wompi", "manual_instructions": "", "manual_window_minutes": 60 }
```

## Put Payment Settings

Replaces the three settings.

```
PUT /api/organization/payment-settings
```

### Request Body

| Field                   | Type    | Required | Description |
| ----------------------- | ------- | -------- | ----------- |
| `strategy`              | string  | Yes      | `manual` or `wompi` |
| `manual_instructions`   | string  | Yes for `manual` | Trimmed; at most 1000 characters |
| `manual_window_minutes` | integer | No       | 10 to 1440. Omitted means 60 |

### Example Request

```json
{
  "strategy": "manual",
  "manual_instructions": "Llave Bre-B 3001234567, a nombre de Laguna Escondida SAS",
  "manual_window_minutes": 60
}
```

### Example Response (200 OK)

The saved settings, as in Get.

### Error Responses

Nothing is stored when the request is refused.

**400 Bad Request** - A value is invalid; `field` names it

```json
{
  "error": "invalid manual_instructions: is required for the manual strategy",
  "field": "manual_instructions"
}
```

| `field`                 | Refused when |
| ----------------------- | ------------ |
| `strategy`              | not `manual` or `wompi` |
| `manual_instructions`   | empty after trimming under `manual`, or over 1000 characters |
| `manual_window_minutes` | not a whole number from 10 to 1440 |

**403 Forbidden** - The caller lacks `integrations:read` / `integrations:manage` (a manager, for example).

**401 Unauthorized** - Missing or invalid token.
