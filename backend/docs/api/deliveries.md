# Deliveries API

Tracks a paid takeaway or delivery order from the kitchen to the customer: which orders are ready, which courier has each one on the road, and whether it was delivered or why it failed.

**Cloud only.** These routes are not served on the box (edge); every one returns `404` there. Till orders reach the cloud as `dine_in` and are not tracked.

## Delivery status

Only **paid, not voided** `takeaway` and `delivery` orders have a delivery status. Unpaid, voided and `dine_in` orders have none.

| Status             | Stored | Meaning                                                              |
| ------------------ | ------ | -------------------------------------------------------------------- |
| `preparing`        | no     | Nothing recorded yet, and a line is neither completed nor cancelled |
| `ready`            | no     | Nothing recorded yet, and every line is completed or cancelled       |
| `out_for_delivery` | yes    | On the road with a courier (a courier user or a third party)         |
| `delivered`        | yes    | Handed over (shown as "Recogido" for takeaway). Final                |
| `failed`           | yes    | The delivery failed, with a reason                                   |

`preparing` and `ready` are derived from the lines on every read. `ready_at` is stamped the first time the kitchen finishes the order; an undo of a line doesn't clear it.

### Transitions

| From                   | To                 | Order type | Endpoint   |
| ---------------------- | ------------------ | ---------- | ---------- |
| `preparing` or `ready` | `out_for_delivery` | delivery   | `dispatch` |
| `out_for_delivery`     | `delivered`        | delivery   | `deliver`  |
| `out_for_delivery`     | `failed`           | delivery   | `fail`     |
| `failed`               | `out_for_delivery` | delivery   | `dispatch` (re-dispatch) |
| `preparing` or `ready` | `delivered`        | takeaway   | `deliver`  |

Any other change answers `409 INVALID_DELIVERY_TRANSITION` with the order's `current_status` and records nothing. So does a change on an unpaid, voided or dine-in order, with `current_status: null`.

### Who may

| Who             | Permission          | May                                                                                     |
| --------------- | ------------------- | --------------------------------------------------------------------------------------- |
| admin, manager  | `deliveries:manage` | Every transition on any order of the organization                                        |
| courier         | `deliveries:carry`  | Claim a `ready` delivery order with no courier (dispatch to themselves); set **their own** order on the road `delivered` or `failed` |

Anything else a courier asks answers `403 DELIVERY_FORBIDDEN`: a `preparing` order, another courier's order, a re-dispatch, a takeaway, or naming another courier. Other roles get `403` from the routes. The transition is checked first, so a courier asking an impossible change of their own order gets `409`.

When two couriers claim the same order at once, one gets it and the other gets `409` with `current_status: "out_for_delivery"`.

## Permissions

| Permission          | Granted to     | Allows                                                       |
| ------------------- | -------------- | ------------------------------------------------------------ |
| `deliveries:read`   | admin, manager | The dashboard list and an order's history                    |
| `deliveries:manage` | admin, manager | Every transition; the list of couriers to assign             |
| `deliveries:carry`  | courier, admin | The courier's lists; claim, and deliver or fail their own orders |

## Endpoints

| Method | Endpoint                                 | Permission                  | Description                                 |
| ------ | ---------------------------------------- | --------------------------- | ------------------------------------------- |
| GET    | `/api/deliveries/orders`                 | `deliveries:read`           | The dashboard: orders by delivery status    |
| GET    | `/api/deliveries/orders/:id/history`     | `deliveries:read`           | An order's status changes, oldest first     |
| GET    | `/api/deliveries/couriers`               | `deliveries:manage`         | Active courier users, to assign             |
| GET    | `/api/deliveries/mine`                   | `deliveries:carry`          | What the courier can claim and what they carry |
| POST   | `/api/deliveries/orders/:id/dispatch`    | `deliveries:manage` or `:carry` | Set out for delivery, naming the courier |
| POST   | `/api/deliveries/orders/:id/deliver`     | `deliveries:manage` or `:carry` | Set delivered                            |
| POST   | `/api/deliveries/orders/:id/fail`        | `deliveries:manage` or `:carry` | Set failed, with a reason               |

All routes need a JWT (`Authorization: Bearer <token>`) and act on the token's organization.

## The delivery order

Every list and every transition returns orders in this shape. It carries **no amounts**: couriers read it too, and only need to know it is paid.

```json
{
  "id": "019a0000-0000-7000-8000-0000000000d1",
  "temporal_identifier": "019a0000-0000-7000-8000-0000000000d2",
  "descriptor": "Pedido WhatsApp",
  "order_type": "delivery",
  "customer_name": "Ana",
  "customer_phone": "+573005551234",
  "delivery_address": "Calle 10 # 5-20, apto 301",
  "delivery_lat": "4.711000",
  "delivery_lng": "-74.072100",
  "delivery_map_url": "https://www.google.com/maps/search/?api=1&query=4.711,-74.0721",
  "delivery_status": "out_for_delivery",
  "status_since": "2026-11-20T12:10:00-05:00",
  "courier": { "user_id": "019a0000-0000-7000-8000-0000000000a1", "name": "Juan" },
  "failure_reason": "wrong_address",
  "paid": true,
  "paid_at": "2026-11-20T11:40:00-05:00",
  "ready_at": "2026-11-20T12:02:00-05:00",
  "created_at": "2026-11-20T11:35:00-05:00",
  "lines": [
    { "name": "Mojarra frita", "quantity": 2, "notes": "sin cebolla", "status": "completed" }
  ]
}
```

| Field              | Notes                                                                                   |
| ------------------ | --------------------------------------------------------------------------------------- |
| `delivery_status`  | See [Delivery status](#delivery-status)                                                 |
| `status_since`     | When it entered its current status: the change's instant, `ready_at` for `ready`, `paid_at` for `preparing` |
| `courier`          | Absent until the order first goes out. `user_id` is absent for a third party            |
| `failure_reason`   | The last failure's reason; kept after a re-dispatch. Absent if it never failed          |
| `delivery_*`       | Delivery orders only                                                                    |
| `ready_at`         | Absent until the kitchen first finished the order                                       |
| `lines`            | Cancelled lines are left out                                                            |

## GET /api/deliveries/orders

The dashboard list: every tracked order that is not `delivered`, whatever its day, plus those `delivered` during the current business day (America/Bogota). Ordered by `paid_at`, oldest first.

| Query        | Values                                                          |
| ------------ | --------------------------------------------------------------- |
| `status`     | Optional. `preparing`, `ready`, `out_for_delivery`, `delivered`, `failed` |
| `order_type` | Optional. `takeaway` or `delivery`                              |

**200 OK**

```json
{ "orders": [ { "id": "019a0000-0000-7000-8000-0000000000d1", "delivery_status": "ready", "...": "..." } ] }
```

**400 Bad Request** — an unknown filter value

```json
{ "error": "invalid status: must be preparing, ready, out_for_delivery, delivered or failed", "code": "VALIDATION_ERROR", "field": "status" }
```

**403 Forbidden** — a courier, or any role without `deliveries:read`.

## GET /api/deliveries/orders/:id/history

The order's recorded changes, oldest first, with when it was paid and became ready. Entries are never changed or deleted. The `note` is for staff only.

**200 OK**

```json
{
  "order_id": "019a0000-0000-7000-8000-0000000000d1",
  "paid_at": "2026-11-20T11:40:00-05:00",
  "ready_at": "2026-11-20T12:02:00-05:00",
  "entries": [
    {
      "status": "out_for_delivery",
      "occurred_at": "2026-11-20T12:10:00-05:00",
      "actor": { "id": "019a0000-0000-7000-8000-0000000000a9", "name": "Gerente" },
      "courier": { "user_id": "019a0000-0000-7000-8000-0000000000a1", "name": "Juan" }
    },
    {
      "status": "failed",
      "occurred_at": "2026-11-20T12:40:00-05:00",
      "actor": { "id": "019a0000-0000-7000-8000-0000000000a1", "name": "Juan" },
      "courier": { "user_id": "019a0000-0000-7000-8000-0000000000a1", "name": "Juan" },
      "reason": "customer_unreachable",
      "note": "portería no deja subir, no contesta"
    }
  ]
}
```

**404 Not Found** — no tracked order with that id in the organization: `{"error": "Order not found", "code": "ORDER_NOT_FOUND"}`.

## GET /api/deliveries/couriers

Active users of the organization holding the courier role, by name.

**200 OK**

```json
{ "couriers": [ { "id": "019a0000-0000-7000-8000-0000000000a1", "name": "Juan" } ] }
```

## GET /api/deliveries/mine

For the courier page. `claimable`: the organization's `ready` delivery orders with no courier, oldest ready first. `on_the_road`: the caller's own `out_for_delivery` orders, in the order they went out, oldest first. Never another courier's orders.

**200 OK**

```json
{
  "claimable": [ { "id": "019a0000-0000-7000-8000-0000000000d3", "delivery_status": "ready", "...": "..." } ],
  "on_the_road": [ { "id": "019a0000-0000-7000-8000-0000000000d1", "delivery_status": "out_for_delivery", "...": "..." } ]
}
```

## POST /api/deliveries/orders/:id/dispatch

Sets the order `out_for_delivery`. Name exactly one courier:

| Field             | Type   | Description                                                       |
| ----------------- | ------ | ----------------------------------------------------------------- |
| `courier_user_id` | string | An active courier user of the organization                        |
| `courier_name`    | string | A third party, 1–80 characters (trimmed)                          |

A courier may send no body (or `{}`): it means themselves. Naming anyone else is `403`.

```json
{ "courier_user_id": "019a0000-0000-7000-8000-0000000000a1" }
```

```json
{ "courier_name": "Mensajeros Ya" }
```

**200 OK** — the updated [delivery order](#the-delivery-order).

**400 Bad Request** — both, neither, a name over 80 characters, or a user who is not an active courier:

```json
{ "error": "invalid courier_user_id: must be an active courier of the organization", "code": "VALIDATION_ERROR", "field": "courier_user_id" }
```

**409 Conflict** — not a valid transition, or another courier claimed it first:

```json
{ "error": "the order is out_for_delivery and cannot dispatch", "code": "INVALID_DELIVERY_TRANSITION", "current_status": "out_for_delivery" }
```

**403 Forbidden** — `{"error": "You may not make that delivery change", "code": "DELIVERY_FORBIDDEN"}`.

## POST /api/deliveries/orders/:id/deliver

Sets a delivery order on the road, or a `preparing`/`ready` takeaway, `delivered`. No body.

**200 OK** — the updated order. **409** as above (for example a `ready` delivery order: it must go out first). **403** for a courier on anyone else's order, or on a takeaway.

## POST /api/deliveries/orders/:id/fail

Sets a delivery order on the road `failed`.

| Field    | Type   | Required | Description                                                         |
| -------- | ------ | -------- | ------------------------------------------------------------------- |
| `reason` | string | Yes      | `customer_unreachable`, `wrong_address`, `refused` or `other`       |
| `note`   | string | No       | Up to 500 characters. Staff only: never sent to the customer        |

```json
{ "reason": "customer_unreachable", "note": "portería no deja subir, no contesta" }
```

**200 OK** — the updated order. **400** for a missing or unknown `reason` or a note over 500 characters (`field` names which). **409** / **403** as above.

## Customer notifications

For an order taken by the WhatsApp agent, marketing-app is told on `/backend/notifications` (see [marketing-events.md](marketing-events.md)): `order.ready` when a takeaway becomes ready, `order.out_for_delivery` on every dispatch, `order.delivered` and `order.delivery_failed` (with the reason, never the note). Each is recorded in the same transaction as the change. Orders without a channel (taken at the till) notify nothing.
