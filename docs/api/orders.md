# Orders (Open Bills)

Manages open orders (open bills) — active tabs that can contain products before being paid and converted to a bill.

## Endpoints

| Method | Endpoint                                                     | Description                    |
| ------ | ------------------------------------------------------------ | ------------------------------ |
| POST   | `/api/orders`                                                | Create a new order             |
| GET    | `/api/orders`                                                | List all active orders (filterable) |
| GET    | `/api/orders/closed`                                         | List orders closed today (filterable) |
| GET    | `/api/orders/closed/:id`                                     | Get a closed order with products |
| GET    | `/api/orders/:id`                                            | Get order with products        |
| PUT    | `/api/orders/:id`                                            | Update an order                |
| DELETE | `/api/orders/:id`                                            | Delete (soft) an order         |
| POST   | `/api/orders/pay-order`                                      | Pay and close an order         |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/complete`    | Mark product as completed      |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/in-progress` | Mark product as in-progress    |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/uncomplete`  | Revert a product to created    |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/cancel`      | Cancel a product in the order  |

> **Note:** Paying an order (`POST /api/orders/pay-order`) requires the `orders:pay` permission, which is granted only to **manager** and **admin** roles. Servers can create, update, and cancel product lines, but cannot pay/close a bill.
>
> **Note:** Marking a product **completed** or **in-progress** requires the `orders:complete-product` permission (the kitchen preparation flow), granted to **cooker**, **manager**, and **admin** roles. **Cancelling** a product requires `orders:update` (server, manager, admin) — cookers cannot cancel product lines.

### The agent role

A user whose **only** role is `agent` (the WhatsApp agent's service user) is further limited by the service:

- `POST /api/orders` accepts only `takeaway` and `delivery`. A `dine_in` order (or one without `order_type`) is refused with `403 AGENT_ORDER_TYPE_NOT_ALLOWED`.
- `PUT /api/orders/:id`, `DELETE /api/orders/:id` and `PATCH …/cancel` work only on takeaway and delivery orders **it created**. Any other order is refused with `403 ORDER_NOT_OWNED` and left untouched.

```json
{
  "error": "The agent can only change takeaway and delivery orders it created",
  "code": "ORDER_NOT_OWNED"
}
```

A user with a staff role besides `agent` is not limited.

## Order types: dine-in, takeaway and delivery

Every order has an `order_type`:

| Type       | Meaning                          | Required on create                                          |
| ---------- | -------------------------------- | ----------------------------------------------------------- |
| `dine_in`  | Eaten at the restaurant (default) | Nothing new                                                |
| `takeaway` | Picked up by the customer         | `customer_phone`                                           |
| `delivery` | Taken to the customer             | `customer_phone`, `delivery_address`, `delivery_lat`, `delivery_lng` inside an active [delivery zone](./delivery-zones.md) |

- **Set on create only.** `PUT /api/orders/:id` cannot change the type, contact, address or location; a customer who moved cancels and re-orders.
- **Phones are stored in E.164** (`+573005551234`). Spaces, dashes, dots and parentheses are ignored. A number starting with `+` is read as international; anything else as a number of the configured home region (`PHONE_HOME_REGION`, default Colombia), which also accepts that region's numbers written with the country code but no `+` (`573005551234`). Invalid numbers are rejected with `400 INVALID_PHONE`.
- **The delivery location is the pin, not the address.** `delivery_lat`/`delivery_lng` are decimal degrees (strings, up to 6 decimals). The address is free text for the rider (building, apartment, door) and is never geocoded. The order records the oldest active zone containing the pin; if none contains it, or no zone is active, the order is refused with `422`.
- `delivery_lat`/`delivery_lng` sent with a `dine_in` or `takeaway` order are ignored.
- The contact is **not** a fiscal customer: no bill owner is created. Choosing who the invoice is for still happens at `pay-order`.
- Orders replicated from the restaurant (edge) are always `dine_in`.

### Held until paid

The lines of an **unpaid** `takeaway` or `delivery` order are **held out of the kitchen**:

- They are not on any kitchen board (SSE feed and "Comandas Listas"), and no live SSE event is sent when such an order is created, edited or deleted.
- `in-progress` and `complete` on them return `409 ORDER_AWAITING_PAYMENT`, whoever calls.
- The order stays editable (`PUT`), its lines cancellable, and it can be deleted (stock returned), like any open order. Stock is reserved at creation, as for any order.

**Paying releases them.** After `POST /api/orders/pay-order`, the lines appear live on their kitchen boards (an `open_bill_product.created` SSE event per line), and from then on follow the [paid-order lifecycle](#kitchen-lines-after-payment). The kitchen's waiting time (`created_at` on the board) is the **payment instant**, both in the live event and after a reload. `dine_in` is unchanged.

## Kitchen lines after payment

Paying an order (`POST /api/orders/pay-order`) **closes the order but not its kitchen lines**:

- The order leaves the active list (`GET /api/orders`), cannot be edited, re-paid or deleted, and appears in the closed orders of its day. `GET /api/orders/:id` returns `404` for it.
- Its unfinished lines **stay on the kitchen board** (SSE feed and "Comandas Listas") until they are completed or cancelled, exactly like an open order's lines.
- On a paid order's lines, **complete**, **in-progress** and **uncomplete** work as usual (`200`). **Cancel** is refused with `409 Conflict`: the money is already taken, so a refund is a staff decision outside the system.
- A **deleted (voided)** order's lines leave the board, and every line endpoint on it returns `404 Order not found`.
- Orders closed before this behavior shipped are treated as voided: their lines do not reappear on the board.

### Line timing

Each line records `started_at` (first set in-progress) and `completed_at` (completed). Completing a line that was never in progress records only `completed_at`. Uncompleting clears `completed_at` (and `started_at` too when the line was in progress). Later edits to the order never move them. The kitchen feed's `completed_at` is the real completion instant. Lines completed before this change fall back to their last update time.

---

## Create Order

`POST /api/orders`

Creates a new open order. Products are optional — an empty order can be created.

The `temporal_identifier` must be unique among **active** orders. An order is active while it is not completed, cancelled, or deleted. Creating an order whose `temporal_identifier` matches an existing active order is rejected with `409 Conflict`. Once an order is completed or cancelled, its `temporal_identifier` may be reused.

### Retrying safely: `X-Idempotency-Key`

Send an optional `X-Idempotency-Key` header (1–255 characters, a fresh UUID per order works well) to make the request safe to retry after a timeout or a dropped connection.

- The **first** request with a key creates the order as usual.
- A **retry** with the same key, same user and same body creates nothing. It returns `201 Created` with the order the first request created, plus the response header `Idempotent-Replayed: true`. The body is the original response, even if the order has changed since.
- If the first request **failed** (any 4xx/5xx), the key is freed, and a retry is processed as a new request.
- Without the header, nothing changes: each request creates an order.

| Status | `code`                            | When                                                                   |
| ------ | --------------------------------- | ---------------------------------------------------------------------- |
| 400    | `INVALID_IDEMPOTENCY_KEY`         | The key is longer than 255 characters                                  |
| 409    | `IDEMPOTENCY_REQUEST_IN_PROGRESS` | A request with this key is still being processed; retry in a few seconds |
| 422    | `IDEMPOTENCY_KEY_REUSED`          | The key was already used by another user or with a different body      |

A request that dies mid-flight holds its key for 2 minutes, after which a retry takes it over. Keys are kept per node (cloud or edge box) and are not synced.

### Request Body

| Field               | Type     | Required | Description                                   |
| ------------------- | -------- | -------- | --------------------------------------------- |
| open_bill_id        | string   | Yes      | UUID for the new order                         |
| temporal_identifier | string   | Yes      | UUID used as a temporary human-readable label; unique among active orders |
| descriptor          | string   | No       | Optional description / table name              |
| products            | array    | No       | List of products to add                        |
| products[].open_bill_product_id | string | Yes | UUID for each line item              |
| products[].product_id           | string | Yes | Product UUID                         |
| products[].quantity             | int    | Yes | Quantity (min 1)                     |
| products[].notes                | string | No  | Optional notes for the item          |
| products[].side_dishes          | array  | No  | Side-dish selections for a composite product (see below) |
| products[].side_dishes[].ingredient_product_id | string | Yes | UUID of the side-dish ingredient |
| products[].side_dishes[].quantity              | int    | Yes | Chosen quantity, within the option's `[min, max]` (0 removes it) |
| order_type          | string | No       | `dine_in` (default), `takeaway` or `delivery` |
| customer_name       | string | No       | Customer name |
| customer_phone      | string | Conditional | Required for `takeaway` and `delivery`; normalized to E.164 |
| delivery_address    | string | Conditional | Required for `delivery`; reference for the rider |
| delivery_lat        | string | Conditional | Required for `delivery`; latitude in [-90, 90] |
| delivery_lng        | string | Conditional | Required for `delivery`; longitude in [-180, 180] |

> **Side dishes:** For a composite product whose ingredients are configured as side dishes (see [Products API → Side Dishes](./products.md#side-dishes)), a line may carry `side_dishes` to override how much of each is served. Omitted side dishes resolve to their configured default, so a line with no `side_dishes` behaves exactly as before. A selection referencing a non-side-dish ingredient, or a quantity outside `[min, max]`, rejects the whole order. Side-dish changes never affect the plate price; stock consumption follows the selected quantities.

### Example Request

```json
{
  "open_bill_id": "550e8400-e29b-41d4-a716-446655440000",
  "temporal_identifier": "660e8400-e29b-41d4-a716-446655440001",
  "descriptor": "Mesa 5",
  "products": [
    {
      "open_bill_product_id": "770e8400-e29b-41d4-a716-446655440002",
      "product_id": "880e8400-e29b-41d4-a716-446655440003",
      "quantity": 2,
      "notes": "Sin hielo",
      "side_dishes": [
        { "ingredient_product_id": "aa0e8400-e29b-41d4-a716-446655440010", "quantity": 0 },
        { "ingredient_product_id": "bb0e8400-e29b-41d4-a716-446655440011", "quantity": 3 }
      ]
    }
  ]
}
```

### Example Response (201 Created)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "temporal_identifier": "660e8400-e29b-41d4-a716-446655440001",
  "total_amount": "25000",
  "status": "created",
  "descriptor": "Mesa 5",
  "created_at": "2025-06-15T10:30:00Z",
  "updated_at": "2025-06-15T10:30:00Z",
  "order_type": "dine_in"
}
```

### Example Request (Delivery)

```json
{
  "open_bill_id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d50",
  "temporal_identifier": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d51",
  "descriptor": "Domicilio Ana",
  "order_type": "delivery",
  "customer_name": "Ana Gómez",
  "customer_phone": "300 555 1234",
  "delivery_address": "Cra 7 # 12-34, Edificio Colón, apto 501",
  "delivery_lat": "4.598100",
  "delivery_lng": "-74.076000",
  "products": [
    {
      "open_bill_product_id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d52",
      "product_id": "880e8400-e29b-41d4-a716-446655440003",
      "quantity": 1
    }
  ]
}
```

### Example Response (201 Created, Delivery)

```json
{
  "id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d50",
  "temporal_identifier": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d51",
  "total_amount": "32000",
  "status": "created",
  "descriptor": "Domicilio Ana",
  "created_at": "2026-10-15T12:00:00Z",
  "updated_at": "2026-10-15T12:00:00Z",
  "order_type": "delivery",
  "customer_name": "Ana Gómez",
  "customer_phone": "+573005551234",
  "delivery_address": "Cra 7 # 12-34, Edificio Colón, apto 501",
  "delivery_lat": "4.5981",
  "delivery_lng": "-74.076",
  "delivery_zone_id": "019a0f00-1111-7000-8000-00000000c3e7",
  "delivery_map_url": "https://www.google.com/maps/search/?api=1&query=4.5981,-74.076"
}
```

### Error Responses

**400 Bad Request** — invalid `order_type`, a missing contact/location field, an invalid phone or location, or an invalid side-dish selection. The `code` names the problem.

```json
{
  "error": "delivery_address is required for this order type",
  "code": "ORDER_MISSING_FIELD"
}
```

| `code`                            | When                                                          |
| --------------------------------- | ------------------------------------------------------------- |
| `ORDER_INVALID_TYPE`              | `order_type` is not `dine_in`, `takeaway` or `delivery`        |
| `ORDER_MISSING_FIELD`             | `customer_phone` (takeaway/delivery), `delivery_address` or `delivery_lat`/`delivery_lng` (delivery) missing |
| `INVALID_PHONE`                   | `customer_phone` is not a valid number for its country         |
| `ORDER_INVALID_DELIVERY_LOCATION` | `delivery_lat`/`delivery_lng` not numeric or out of range      |

**422 Unprocessable Entity** — the delivery pin is outside every active delivery zone (or no zone is active)

```json
{
  "error": "The delivery location is outside every delivery zone",
  "code": "OUTSIDE_DELIVERY_ZONE"
}
```

**404 Not Found** — one or more products do not exist

```json
{
  "error": "One or more products not found"
}
```

**409 Conflict** — an active order already uses this `temporal_identifier`

```json
{
  "error": "An active order with this temporal identifier already exists"
}
```

**500 Internal Server Error** — the order could not be created

```json
{
  "error": "Failed to create order"
}
```

---

## Update Order

`PUT /api/orders/:id`

Updates an existing order's products, description, and/or temporal identifier. Sending an empty `products` array clears all products. Fields `temporal_identifier` and `descriptor` are optional — only sent fields are updated. The order's `order_type`, contact, address, location and zone never change on update.

### Path Parameters

| Param | Type   | Description       |
| ----- | ------ | ----------------- |
| id    | string | Order UUID        |

### Request Body

| Field               | Type   | Required | Description                                     |
| ------------------- | ------ | -------- | ----------------------------------------------- |
| temporal_identifier | string | No       | New temporal identifier UUID                     |
| descriptor          | string | No       | New description / table name                     |
| products            | array  | No       | Full list of products (replaces existing)        |
| products[].open_bill_product_id | string | Yes | UUID for each line item                |
| products[].product_id           | string | Yes | Product UUID                           |
| products[].quantity             | int    | Yes | Quantity (min 1)                       |
| products[].notes                | string | No  | Optional notes for the item            |
| products[].side_dishes          | array  | No  | Side-dish selections (same shape as Create; editing them adjusts stock and re-notifies the kitchen) |

### Example Request

```json
{
  "temporal_identifier": "660e8400-e29b-41d4-a716-446655440099",
  "descriptor": "Mesa 12",
  "products": [
    {
      "open_bill_product_id": "770e8400-e29b-41d4-a716-446655440002",
      "product_id": "880e8400-e29b-41d4-a716-446655440003",
      "quantity": 3,
      "notes": "Con limón",
      "side_dishes": [
        { "ingredient_product_id": "bb0e8400-e29b-41d4-a716-446655440011", "quantity": 1 }
      ]
    }
  ]
}
```

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "temporal_identifier": "660e8400-e29b-41d4-a716-446655440099",
  "total_amount": "37500",
  "status": "created",
  "descriptor": "Mesa 12",
  "created_at": "2025-06-15T10:30:00Z",
  "updated_at": "2025-06-15T10:35:00Z"
}
```

### Error Responses

**404 Not Found** - Order not found

```json
{
  "error": "Order not found"
}
```

**404 Not Found** - Product not found

```json
{
  "error": "One or more products not found"
}
```

---

## List All Active Orders

`GET /api/orders`

Returns all open orders that have not been deleted. Without query parameters it returns every active order, as before.

### Query Parameters

| Param          | Type   | Required | Description |
| -------------- | ------ | -------- | ----------- |
| order_type     | string | No       | `dine_in`, `takeaway` or `delivery` |
| customer_phone | string | No       | Any format; normalized like on create, so `300-555-1234` matches `+573005551234` |

When both are given an order must match both. An invalid value returns `400` (`ORDER_INVALID_TYPE` / `INVALID_PHONE`). Each item carries the order-type fields (`order_type`, and for takeaway/delivery the contact, location, `delivery_zone_id` and `delivery_map_url`).

`GET /api/orders?order_type=delivery` is what a delivery screen polls for unpaid agent orders; pair it with `GET /api/orders/closed?order_type=delivery` for the ones paid and being prepared.

### Example Response (200 OK)

```json
{
  "open_bills": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "temporal_identifier": "660e8400-e29b-41d4-a716-446655440001",
      "total_amount": "25000",
      "status": "created",
      "created_by": {
        "id": "user-uuid",
        "user_name": "admin",
        "name": "Admin User"
      },
      "descriptor": "Mesa 5",
      "created_at": "2025-06-15T10:30:00Z",
      "updated_at": "2025-06-15T10:30:00Z"
    }
  ],
  "total": 1
}
```

---

## List Orders Closed Today

`GET /api/orders/closed`

Returns the orders created during the current business day (America/Bogota) that are closed, paid or voided, most recently closed first. Accepts the same `order_type` and `customer_phone` filters as the active list.

Each item has `paid_at` when it was paid; a voided order has none.

### Example Response (200 OK)

```json
{
  "open_bills": [
    {
      "id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d50",
      "temporal_identifier": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d51",
      "total_amount": "32000",
      "status": "created",
      "created_by": { "id": "user-uuid", "user_name": "agente", "name": "WhatsApp Agent" },
      "descriptor": "Domicilio Ana",
      "created_at": "2026-10-15T12:00:00Z",
      "updated_at": "2026-10-15T12:20:00Z",
      "paid_at": "2026-10-15T12:20:00Z",
      "order_type": "delivery",
      "customer_name": "Ana Gómez",
      "customer_phone": "+573005551234",
      "delivery_address": "Cra 7 # 12-34, Edificio Colón, apto 501",
      "delivery_lat": "4.5981",
      "delivery_lng": "-74.076",
      "delivery_zone_id": "019a0f00-1111-7000-8000-00000000c3e7",
      "delivery_map_url": "https://www.google.com/maps/search/?api=1&query=4.5981,-74.076"
    }
  ],
  "total": 1
}
```

`GET /api/orders/closed/:id` returns one closed order with its products (same shape as [Get Order With Products](#get-order-with-products), plus `paid_at`).

---

## Get Order With Products

`GET /api/orders/:id`

Returns a specific order with its product details.

### Path Parameters

| Param | Type   | Description |
| ----- | ------ | ----------- |
| id    | string | Order UUID  |

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "temporal_identifier": "660e8400-e29b-41d4-a716-446655440001",
  "total_amount": "25000",
  "status": "created",
  "created_by": {
    "id": "user-uuid",
    "user_name": "admin",
    "name": "Admin User"
  },
  "descriptor": "Mesa 5",
  "products": [
    {
      "open_bill_product_id": "770e8400-e29b-41d4-a716-446655440002",
      "product": {
        "id": "880e8400-e29b-41d4-a716-446655440003",
        "name": "Limonada",
        "price": "12500"
      },
      "quantity": 2,
      "notes": "Sin hielo",
      "status": "completed",
      "area": "bar",
      "priority": 1,
      "started_at": "2025-06-15T10:32:00Z",
      "completed_at": "2025-06-15T10:41:00Z"
    }
  ],
  "created_at": "2025-06-15T10:30:00Z",
  "updated_at": "2025-06-15T10:30:00Z"
}
```

---

## Delete Order

`DELETE /api/orders/:id`

Voids an open (unpaid) order: it leaves the active list and its stock is returned.

### Path Parameters

| Param | Type   | Description |
| ----- | ------ | ----------- |
| id    | string | Order UUID  |

### Example Response (200 OK)

```json
{
  "message": "Order deleted successfully"
}
```

### Error Responses

**404 Not Found** — the id is not an open order: it does not exist, is already voided, or is paid. Nothing changes: no stock is returned and nothing is replicated.

```json
{
  "error": "Order not found"
}
```

**403 Forbidden** — the caller is the agent and the order is not one of its takeaway or delivery orders (`ORDER_NOT_OWNED`, see [The agent role](#the-agent-role)).

---

## Complete Product

`PATCH /api/orders/:id/products/:open_bill_product_id/complete`

Marks a product in the order as completed and records its `completed_at`. Works on open and paid orders (see [Kitchen lines after payment](#kitchen-lines-after-payment)).

**Required permission:** `orders:complete-product` (cooker, manager, admin)

### Path Parameters

| Param                | Type   | Description              |
| -------------------- | ------ | ------------------------ |
| id                   | string | Order UUID               |
| open_bill_product_id | string | Open bill product UUID   |

### Example Response (200 OK)

```json
{
  "message": "Product completed successfully"
}
```

### Error Responses

**404 Not Found** — the order does not exist or was deleted (voided)

```json
{
  "error": "Order not found"
}
```

**409 Conflict** — the product is already completed

```json
{
  "error": "Product is already completed"
}
```

**409 Conflict** — the order is an unpaid takeaway or delivery, held until payment (also returned by `in-progress`)

```json
{
  "error": "The order must be paid before the kitchen can prepare it",
  "code": "ORDER_AWAITING_PAYMENT"
}
```

---

## Set Product In Progress

`PATCH /api/orders/:id/products/:open_bill_product_id/in-progress`

Marks a product as in-progress and records its `started_at` (only the first time). Works on open and paid orders. On an unpaid takeaway or delivery order it returns `409 ORDER_AWAITING_PAYMENT` (see [Held until paid](#held-until-paid)).

**Required permission:** `orders:complete-product` (cooker, manager, admin)

### Path Parameters

| Param                | Type   | Description              |
| -------------------- | ------ | ------------------------ |
| id                   | string | Order UUID               |
| open_bill_product_id | string | Open bill product UUID   |

### Example Response (200 OK)

```json
{
  "message": "Product set to in progress successfully"
}
```

---

## Uncomplete Product

`PATCH /api/orders/:id/products/:open_bill_product_id/uncomplete`

Reverts a completed or in-progress product to `created` (undo a strike-through, or un-pin a line). Clears `completed_at`; also clears `started_at` when the line was in progress. Works on open and paid orders.

**Required permission:** `orders:complete-product` (cooker, manager, admin)

### Path Parameters

| Param                | Type   | Description              |
| -------------------- | ------ | ------------------------ |
| id                   | string | Order UUID               |
| open_bill_product_id | string | Open bill product UUID   |

### Example Response (200 OK)

```json
{
  "message": "Product uncompleted successfully"
}
```

### Error Responses

**409 Conflict** — the product is neither completed nor in progress

```json
{
  "error": "Product is not completed"
}
```

---

## Cancel Product

`PATCH /api/orders/:id/products/:open_bill_product_id/cancel`

Cancels a product in the order. Not allowed once the order is paid.

**Required permission:** `orders:update` (server, manager, admin)

### Path Parameters

| Param                | Type   | Description              |
| -------------------- | ------ | ------------------------ |
| id                   | string | Order UUID               |
| open_bill_product_id | string | Open bill product UUID   |

### Example Response (200 OK)

```json
{
  "message": "Product cancelled successfully"
}
```

### Error Responses

**404 Not Found** — the order does not exist or was deleted (voided)

```json
{
  "error": "Order not found"
}
```

**409 Conflict** — the order is already paid

```json
{
  "error": "A line of a paid order cannot be cancelled"
}
```

**409 Conflict** — the product is completed or already cancelled

```json
{
  "error": "Product cannot be cancelled from current status"
}
```
