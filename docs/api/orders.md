# Orders (Open Bills)

Manages open orders (open bills) — active tabs that can contain products before being paid and converted to a bill.

## Endpoints

| Method | Endpoint                                                     | Description                    |
| ------ | ------------------------------------------------------------ | ------------------------------ |
| POST   | `/api/orders`                                                | Create a new order             |
| GET    | `/api/orders`                                                | List all active orders         |
| GET    | `/api/orders/:id`                                            | Get order with products        |
| PUT    | `/api/orders/:id`                                            | Update an order                |
| DELETE | `/api/orders/:id`                                            | Delete (soft) an order         |
| POST   | `/api/orders/pay-order`                                      | Pay and close an order         |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/complete`    | Mark product as completed      |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/in-progress` | Mark product as in-progress    |
| PATCH  | `/api/orders/:id/products/:open_bill_product_id/cancel`      | Cancel a product in the order  |

> **Note:** Paying an order (`POST /api/orders/pay-order`) requires the `orders:pay` permission, which is granted only to **manager** and **admin** roles. Servers can create, update, and cancel product lines, but cannot pay/close a bill.

---

## Create Order

`POST /api/orders`

Creates a new open order. Products are optional — an empty order can be created.

### Request Body

| Field               | Type     | Required | Description                                   |
| ------------------- | -------- | -------- | --------------------------------------------- |
| open_bill_id        | string   | Yes      | UUID for the new order                         |
| temporal_identifier | string   | Yes      | UUID used as a temporary human-readable label  |
| descriptor          | string   | No       | Optional description / table name              |
| products            | array    | No       | List of products to add                        |
| products[].open_bill_product_id | string | Yes | UUID for each line item              |
| products[].product_id           | string | Yes | Product UUID                         |
| products[].quantity             | int    | Yes | Quantity (min 1)                     |
| products[].notes                | string | No  | Optional notes for the item          |

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
      "notes": "Sin hielo"
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
  "updated_at": "2025-06-15T10:30:00Z"
}
```

---

## Update Order

`PUT /api/orders/:id`

Updates an existing order's products, description, and/or temporal identifier. Sending an empty `products` array clears all products. Fields `temporal_identifier` and `descriptor` are optional — only sent fields are updated.

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
      "notes": "Con limón"
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

Returns all open orders that have not been deleted.

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
      "status": "created",
      "area": "bar",
      "priority": 1
    }
  ],
  "created_at": "2025-06-15T10:30:00Z",
  "updated_at": "2025-06-15T10:30:00Z"
}
```

---

## Delete Order

`DELETE /api/orders/:id`

Soft-deletes an order.

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

---

## Complete Product

`PATCH /api/orders/:id/products/:open_bill_product_id/complete`

Marks a product in the order as completed.

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

---

## Set Product In Progress

`PATCH /api/orders/:id/products/:open_bill_product_id/in-progress`

Marks a product as in-progress.

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

## Cancel Product

`PATCH /api/orders/:id/products/:open_bill_product_id/cancel`

Cancels a product in the order.

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
