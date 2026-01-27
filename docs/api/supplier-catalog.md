# Supplier Catalog API

Manage which products each supplier provides and at what price.

## Endpoints

| Method | Endpoint                                  | Description                     |
| ------ | ----------------------------------------- | ------------------------------- |
| POST   | `/api/suppliers/:id/products`             | Add product to supplier catalog |
| PUT    | `/api/suppliers/:id/products/:product_id` | Update product pricing          |
| DELETE | `/api/suppliers/:id/products/:product_id` | Remove product from catalog     |
| GET    | `/api/suppliers/:id/products`             | List products from a supplier   |
| GET    | `/api/products/:id/suppliers`             | List suppliers for a product    |

---

## Add Product to Supplier

Adds a product to a supplier's catalog with pricing information.

```
POST /api/suppliers/:id/products
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Supplier ID |

### Request Body

| Field        | Type   | Required | Description                                     |
| ------------ | ------ | -------- | ----------------------------------------------- |
| product_id   | UUID   | Yes      | Product ID to add                               |
| unit_cost    | string | Yes      | Cost per unit (decimal)                         |
| supplier_sku | string | No       | Supplier's SKU for this product (max 255 chars) |

### Example Request

```json
{
  "product_id": "660e8400-e29b-41d4-a716-446655440001",
  "unit_cost": "2500.00",
  "supplier_sku": "FP-TOM-001"
}
```

### Example Response (201 Created)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
  "product_id": "660e8400-e29b-41d4-a716-446655440001",
  "unit_cost": "2500.00",
  "supplier_sku": "FP-TOM-001",
  "created_at": "2024-01-26T15:30:00Z",
  "updated_at": "2024-01-26T15:30:00Z"
}
```

### Error Responses

**404 Not Found** - Supplier or product not found

```json
{
  "error": "Supplier not found"
}
```

**409 Conflict** - Product already in supplier's catalog

```json
{
  "error": "Product already exists in supplier catalog"
}
```

---

## Update Product Pricing

Updates the unit cost for a product in a supplier's catalog.

```
PUT /api/suppliers/:id/products/:product_id
```

### Path Parameters

| Parameter  | Type | Description |
| ---------- | ---- | ----------- |
| id         | UUID | Supplier ID |
| product_id | UUID | Product ID  |

### Request Body

| Field        | Type   | Required | Description                     |
| ------------ | ------ | -------- | ------------------------------- |
| unit_cost    | string | Yes      | New cost per unit (decimal)     |
| supplier_sku | string | No       | Supplier's SKU for this product |

### Example Request

```json
{
  "unit_cost": "2800.00",
  "supplier_sku": "FP-TOM-002"
}
```

### Example Response (200 OK)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
  "product_id": "660e8400-e29b-41d4-a716-446655440001",
  "unit_cost": "2800.00",
  "supplier_sku": "FP-TOM-002",
  "created_at": "2024-01-26T15:30:00Z",
  "updated_at": "2024-01-26T16:00:00Z"
}
```

---

## Remove Product from Supplier

Removes a product from a supplier's catalog.

```
DELETE /api/suppliers/:id/products/:product_id
```

### Path Parameters

| Parameter  | Type | Description |
| ---------- | ---- | ----------- |
| id         | UUID | Supplier ID |
| product_id | UUID | Product ID  |

### Response (204 No Content)

No response body on success.

---

## List Products from Supplier

Returns all products that a supplier provides, with pricing.

```
GET /api/suppliers/:id/products
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Supplier ID |

### Example Response (200 OK)

```json
{
  "items": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
      "product_id": "660e8400-e29b-41d4-a716-446655440001",
      "product_name": "Tomato",
      "unit_cost": "2500.00",
      "supplier_sku": "FP-TOM-001",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    },
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
      "product_id": "990e8400-e29b-41d4-a716-446655440004",
      "product_name": "Potato",
      "unit_cost": "1800.00",
      "supplier_sku": "FP-POT-001",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    }
  ],
  "total": 2
}
```

---

## List Suppliers for Product

Returns all suppliers that provide a specific product, with their pricing.

```
GET /api/products/:id/suppliers
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Product ID  |

### Example Response (200 OK)

```json
{
  "items": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
      "supplier_name": "Fresh Produce Co.",
      "product_id": "660e8400-e29b-41d4-a716-446655440001",
      "unit_cost": "2500.00",
      "supplier_sku": "FP-TOM-001",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    },
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440005",
      "supplier_id": "bb0e8400-e29b-41d4-a716-446655440006",
      "supplier_name": "Local Farm",
      "product_id": "660e8400-e29b-41d4-a716-446655440001",
      "unit_cost": "2200.00",
      "supplier_sku": "LF-TOM-100",
      "created_at": "2024-01-26T14:00:00Z",
      "updated_at": "2024-01-26T14:00:00Z"
    }
  ],
  "total": 2
}
```

This is useful for comparing prices from different suppliers for the same product.
