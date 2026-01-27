# Products API

Manage products, including sellable items and ingredients.

## Product Types

| Type         | Description                                                    | Has Selling Price |
| ------------ | -------------------------------------------------------------- | ----------------- |
| `SELLABLE`   | Sold directly to customers (e.g., beverages)                   | Yes               |
| `INGREDIENT` | Used in preparation, not sold directly (e.g., raw tomato)      | No                |
| `COMPOSITE`  | Made from other products, sold to customers (e.g., fish plate) | Yes               |
| `BOTH`       | Can be sold AND used as ingredient (e.g., tomato)              | Yes               |

## Units of Measure

| Code   | Name          |
| ------ | ------------- |
| `unit` | Units (count) |
| `kg`   | Kilograms     |
| `g`    | Grams         |
| `l`    | Liters        |
| `ml`   | Milliliters   |

## Endpoints

| Method | Endpoint                      | Description                  |
| ------ | ----------------------------- | ---------------------------- |
| POST   | `/api/products`               | Create a new product         |
| GET    | `/api/products`               | List all products            |
| GET    | `/api/products/:id`           | Get product by ID            |
| PUT    | `/api/products/:id`           | Update a product             |
| DELETE | `/api/products/:id`           | Delete a product             |
| GET    | `/api/products/:id/suppliers` | List suppliers for a product |

---

## Create Product

Creates a new product.

```
POST /api/products
```

### Request Body

| Field                  | Type   | Required    | Description                                   |
| ---------------------- | ------ | ----------- | --------------------------------------------- |
| name                   | string | Yes         | Product name (1-255 chars)                    |
| category               | string | Yes         | Category (1-100 chars)                        |
| product_type           | string | Yes         | One of: SELLABLE, INGREDIENT, COMPOSITE, BOTH |
| unit_of_measure        | string | Yes         | One of: unit, kg, g, l, ml                    |
| sku                    | string | Yes         | Stock keeping unit (1-255 chars)              |
| description            | string | No          | Product description                           |
| total_price_with_taxes | string | Conditional | Required for SELLABLE, COMPOSITE, BOTH        |
| vat                    | string | Conditional | VAT percentage, required if has price         |
| ico                    | string | Conditional | ICO percentage, required if has price         |
| taxes_format           | string | Conditional | Must be "percentage" if has price             |

### Example Request (Sellable Product)

```json
{
  "name": "Beer Corona",
  "category": "Beverages",
  "product_type": "SELLABLE",
  "unit_of_measure": "unit",
  "sku": "BEV-CORONA-001",
  "description": "Corona Extra 330ml",
  "total_price_with_taxes": "8000",
  "vat": "19",
  "ico": "8",
  "taxes_format": "percentage"
}
```

### Example Request (Ingredient)

```json
{
  "name": "Tomato",
  "category": "Vegetables",
  "product_type": "INGREDIENT",
  "unit_of_measure": "kg",
  "sku": "VEG-TOM-001",
  "description": "Fresh tomatoes for cooking"
}
```

Note: For INGREDIENT type, price fields are not required and will default to zero.

### Example Response (201 Created)

```json
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "name": "Beer Corona",
  "category": "Beverages",
  "product_type": "SELLABLE",
  "unit_of_measure": "unit",
  "version": 1,
  "unit_price": "6299.21",
  "vat": "0.19",
  "vat_amount": "1196.85",
  "ico": "0.08",
  "ico_amount": "503.94",
  "description": "Corona Extra 330ml",
  "sku": "BEV-CORONA-001",
  "total_price_with_taxes": "8000.00",
  "created_at": "2024-01-26T15:30:00Z",
  "updated_at": "2024-01-26T15:30:00Z"
}
```

---

## List Products

Returns all products.

```
GET /api/products
```

### Example Response (200 OK)

```json
{
  "products": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "Beer Corona",
      "category": "Beverages",
      "product_type": "SELLABLE",
      "unit_of_measure": "unit",
      "version": 1,
      "unit_price": "6299.21",
      "vat": "0.19",
      "vat_amount": "1196.85",
      "ico": "0.08",
      "ico_amount": "503.94",
      "description": "Corona Extra 330ml",
      "sku": "BEV-CORONA-001",
      "total_price_with_taxes": "8000.00",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    },
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "name": "Tomato",
      "category": "Vegetables",
      "product_type": "INGREDIENT",
      "unit_of_measure": "kg",
      "version": 1,
      "unit_price": "0",
      "vat": "0",
      "vat_amount": "0",
      "ico": "0",
      "ico_amount": "0",
      "description": "Fresh tomatoes for cooking",
      "sku": "VEG-TOM-001",
      "total_price_with_taxes": "0",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    }
  ],
  "total": 2
}
```

---

## Get Product by ID

Returns a single product.

```
GET /api/products/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Product ID  |

---

## Update Product

Updates an existing product.

```
PUT /api/products/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Product ID  |

### Request Body

Same as Create Product.

---

## Delete Product

Soft deletes a product.

```
DELETE /api/products/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Product ID  |

### Response (204 No Content)

No response body on success.

---

## List Suppliers for Product

Returns all suppliers that provide this product with their pricing.

```
GET /api/products/:id/suppliers
```

See [Supplier Catalog API](supplier-catalog.md#list-suppliers-for-product) for response format.
