# Products API

Manage products, including sellable items and ingredients.

**Cloud only: catalog writes.** The box (edge) serves every read here (`GET /api/products`, `/api/products/categories`, `/api/products/:id`, `/api/products/:id/ingredients`, `/api/products/:id/option-slots`, `/api/product-responsibilities/:id`) from what it last pulled from the cloud. These routes exist only in the cloud; on the box they return `404`, because the next pull would overwrite the edit:

- `POST /api/products`, `POST /api/products/bulk`, `PUT /api/products/:id`, `DELETE /api/products/:id`
- `POST /api/product-responsibilities`, `PUT /api/product-responsibilities/:id`, `DELETE /api/product-responsibilities/:id`
- `POST /api/products/:id/ingredients`, `PUT /api/products/:id/ingredients/:ingredient_id`, `DELETE /api/products/:id/ingredients/:ingredient_id`
- `PUT /api/products/:id/option-slots`
- `GET /api/products/:id/suppliers` (the box keeps no suppliers)

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

| Method | Endpoint                                      | Description                             |
| ------ | --------------------------------------------- | --------------------------------------- |
| POST   | `/api/products`                               | Create a new product                    |
| POST   | `/api/products/bulk`                          | Bulk create products                    |
| GET    | `/api/products`                               | List all products                       |
| GET    | `/api/products/categories`                    | List all product categories             |
| GET    | `/api/products/:id`                           | Get product by ID                       |
| PUT    | `/api/products/:id`                           | Update a product                        |
| DELETE | `/api/products/:id`                           | Delete a product                        |
| GET    | `/api/products/:id/suppliers`                 | List suppliers for a product            |
| POST   | `/api/products/:id/ingredients`               | Add ingredient to composite product     |
| GET    | `/api/products/:id/ingredients`               | List ingredients of a composite product |
| PUT    | `/api/products/:id/ingredients/:ingredientId` | Update ingredient quantity              |
| DELETE | `/api/products/:id/ingredients/:ingredientId` | Remove ingredient from product          |
| GET    | `/api/products/:id/option-slots`                       | List a plate's option slots          |
| PUT    | `/api/products/:id/option-slots`                       | Replace a plate's option slots (cloud only) |

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

## Bulk Create Products

Creates multiple products in a single request. Optionally links all created products to a supplier.

```
POST /api/products/bulk
```

### Request Body

| Field       | Type   | Required | Description                                                  |
| ----------- | ------ | -------- | ------------------------------------------------------------ |
| supplier_id | string | No       | UUID of the supplier to link products to                     |
| items       | array  | Yes      | Array of product items (min 1)                               |

**Each item in `items`:**

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
| supplier_sku           | string | No          | Supplier's own SKU for this product           |

### Example Request

```json
{
  "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
  "items": [
    {
      "name": "MAIZITOS LIMON 215G",
      "category": "SNACKS",
      "product_type": "SELLABLE",
      "unit_of_measure": "unit",
      "vat": "19",
      "ico": "0",
      "taxes_format": "percentage",
      "sku": "7702914551403",
      "total_price_with_taxes": "6499.78",
      "supplier_sku": "59107"
    },
    {
      "name": "MAIZITOS NATURAL 215G",
      "category": "SNACKS",
      "product_type": "SELLABLE",
      "unit_of_measure": "unit",
      "vat": "19",
      "ico": "0",
      "taxes_format": "percentage",
      "sku": "7702914550505",
      "total_price_with_taxes": "6499.78",
      "supplier_sku": "59109"
    }
  ]
}
```

### Example Response (201 Created)

```json
{
  "created": [
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "name": "MAIZITOS LIMON 215G",
      "category": "SNACKS",
      "product_type": "SELLABLE",
      "unit_of_measure": "unit",
      "version": 1,
      "unit_price": "5462.00",
      "vat": "0.19",
      "vat_amount": "1037.78",
      "ico": "0",
      "ico_amount": "0",
      "sku": "7702914551403",
      "total_price_with_taxes": "6499.78",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    }
  ],
  "errors": [
    {
      "index": 1,
      "sku": "7702914550505",
      "name": "MAIZITOS NATURAL 215G",
      "message": "product with this SKU already exists"
    }
  ]
}
```

### Error Responses

**404 Not Found** - Supplier not found

```json
{
  "error": "Supplier not found"
}
```

**400 Bad Request** - Empty items or validation error

```json
{
  "error": "failed to create product: items cannot be empty"
}
```

---

## List Products

Returns all products, optionally filtered by product type.

```
GET /api/products
```

### Query Parameters

| Parameter      | Type   | Required | Description                                                                                                                         |
| -------------- | ------ | -------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `product_type` | string | No       | Comma-separated list of product types to include. Valid values: `SELLABLE`, `INGREDIENT`, `COMPOSITE`, `BOTH`. Unknown values match no products. When omitted, all products are returned. |

**Example:** `GET /api/products?product_type=SELLABLE,BOTH` — returns only products usable in order creation (sellable directly or both sellable and ingredient).

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

## List Categories

Returns all unique product categories.

```
GET /api/products/categories
```

### Example Response (200 OK)

```json
["Beverages", "Desserts", "Main Courses", "Sides", "Vegetables"]
```

### Error Responses

**500 Internal Server Error** - Failed to retrieve categories

```json
{
  "error": "Failed to list categories"
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

---

# Product Ingredients API

Manage ingredients for COMPOSITE products. When a COMPOSITE product is sold, stock is automatically decreased for its ingredients rather than the composite itself.

---

## Add Ingredient to Product

Adds an ingredient to a composite product.

```
POST /api/products/:id/ingredients
```

### Path Parameters

| Parameter | Type | Description                     |
| --------- | ---- | ------------------------------- |
| id        | UUID | Composite product ID            |

### Request Body

| Field               | Type   | Required | Description                                      |
| ------------------- | ------ | -------- | ------------------------------------------------ |
| ingredient_product_id | string | Yes      | UUID of the ingredient product                   |
| quantity            | string | Yes      | Quantity of ingredient needed per composite unit |

### Example Request

```json
{
  "ingredient_product_id": "770e8400-e29b-41d4-a716-446655440002",
  "quantity": "2.5"
}
```

### Example Response (201 Created)

```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "composite_product_id": "660e8400-e29b-41d4-a716-446655440001",
  "ingredient_product_id": "770e8400-e29b-41d4-a716-446655440002",
  "default_quantity": "2.5",
  "created_at": "2024-01-26T16:00:00Z",
  "updated_at": "2024-01-26T16:00:00Z"
}
```

> **Note:** The recipe amount is `default_quantity` (renamed from `quantity`). Every recipe row is always consumed; what the customer can choose lives in the plate's [option slots](#plate-option-slots).

### Error Responses

**400 Bad Request** - Product is not a composite product

```json
{
  "error": "Product is not a composite product"
}
```

**400 Bad Request** - A product cannot be an ingredient of itself

```json
{
  "error": "A product cannot be an ingredient of itself"
}
```

**404 Not Found** - Product not found

```json
{
  "error": "Product not found"
}
```

**409 Conflict** - Ingredient already exists

```json
{
  "error": "Ingredient already exists for this product"
}
```

---

## List Ingredients

Returns all ingredients for a composite product with full product details.

```
GET /api/products/:id/ingredients
```

### Path Parameters

| Parameter | Type | Description          |
| --------- | ---- | -------------------- |
| id        | UUID | Composite product ID |

### Example Response (200 OK)

```json
{
  "ingredients": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "composite_product_id": "660e8400-e29b-41d4-a716-446655440001",
      "ingredient_product_id": "770e8400-e29b-41d4-a716-446655440002",
      "default_quantity": "2.5",
      "created_at": "2024-01-26T16:00:00Z",
      "updated_at": "2024-01-26T16:00:00Z",
      "ingredient_product": {
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
    }
  ]
}
```

---

## Update Ingredient Quantity

Updates the quantity of an ingredient in a composite product.

```
PUT /api/products/:id/ingredients/:ingredientId
```

### Path Parameters

| Parameter    | Type | Description          |
| ------------ | ---- | -------------------- |
| id           | UUID | Composite product ID |
| ingredientId | UUID | Ingredient ID        |

### Request Body

| Field    | Type   | Required | Description      |
| -------- | ------ | -------- | ---------------- |
| quantity | string | Yes      | New quantity     |

### Example Request

```json
{
  "quantity": "3.0"
}
```

### Example Response (200 OK)

```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "composite_product_id": "660e8400-e29b-41d4-a716-446655440001",
  "ingredient_product_id": "770e8400-e29b-41d4-a716-446655440002",
  "default_quantity": "3.0",
  "created_at": "2024-01-26T16:00:00Z",
  "updated_at": "2024-01-26T16:30:00Z"
}
```

### Error Responses

**404 Not Found** - Ingredient not found

```json
{
  "error": "Ingredient not found"
}
```

---

## Remove Ingredient

Removes an ingredient from a composite product.

```
DELETE /api/products/:id/ingredients/:ingredientId
```

### Path Parameters

| Parameter    | Type | Description          |
| ------------ | ---- | -------------------- |
| id           | UUID | Composite product ID |
| ingredientId | UUID | Ingredient ID        |

### Response (204 No Content)

No response body on success.

### Error Responses

**404 Not Found** - Ingredient not found

```json
{
  "error": "Ingredient not found"
}
```

---

## Stock Behavior with Composite Products

When a COMPOSITE product is ordered:

1. The stock of the **composite product itself is NOT decreased**
2. Instead, the stock of each **ingredient** is decreased
3. The quantity decreased is `ingredient.default_quantity * order_quantity` for each recipe row, plus the bundles of the line's chosen options (see [Plate Option Slots](#plate-option-slots))

### Example

If a "Fish Plate" (COMPOSITE) has ingredients:
- Rice: 0.5 kg per plate
- Fish: 0.3 kg per plate
- Lemon: 1 unit per plate

When 3 Fish Plates are ordered:
- Rice stock decreases by: 0.5 * 3 = 1.5 kg
- Fish stock decreases by: 0.3 * 3 = 0.9 kg
- Lemon stock decreases by: 1 * 3 = 3 units

This happens automatically via the event-driven stock management system.

---

# Plate Option Slots

A composite plate can have **option slots** (they replaced per-ingredient side dishes): ordered, named groups of **choices**, each slot with exactly one default. Every order line for the plate ends with exactly one choice per slot (the one sent, or the default), so "white **or** coconut rice", "fries **instead of** the two canastas" or "a canasta **in place of** the salad" can be configured and nothing else can be ordered.

A choice is a **bundle**: ingredient products with whole quantities (e.g. "Canastas" = 2 canasta de patacón). An empty bundle means the plate is served without that slot (e.g. "Sin ensalada"); it exists only where configured. Bundle ingredients reference products directly and need not be recipe rows; a composite bundle item (e.g. "Canasta con hogao") is consumed through its own recipe. Choices never change the price; an upcharge stays a separate product.

**Recipe rule:** the plate's recipe holds only the fixed parts (the fish), which are always consumed. Anything the customer can choose lives only in the slots' choices, whose bundles are consumed on top. An ingredient that is both a recipe row and in a chosen bundle consumes the bundle's quantity instead of the recipe's, but a recipe row is still consumed when another choice is picked, so keep choosable parts out of the recipe. See the [Orders API](./orders.md#plate-choices) for how a line picks choices.

## Endpoints

| Method | Endpoint                         | Description                                 |
| ------ | -------------------------------- | ------------------------------------------- |
| GET    | `/api/products/:id/option-slots` | List a plate's option slots (cloud and box) |
| PUT    | `/api/products/:id/option-slots` | Replace a plate's option slots (cloud only) |

---

## List Option Slots

Returns the plate's live slots in order, each with its choices in order, the default marked, and each choice's bundle with ingredient names. A plate without slots (or any non-composite product) returns an empty list. On the box it returns the configuration last pulled from the cloud.

Requires `products:read`.

```
GET /api/products/:id/option-slots
```

### Path Parameters

| Parameter | Type | Description          |
| --------- | ---- | -------------------- |
| id        | UUID | Composite product ID |

### Example Response (200 OK)

```json
{
  "slots": [
    {
      "id": "019a5b10-0000-7000-8000-000000000a01",
      "composite_product_id": "019a5b00-0000-7000-8000-0000000000f1",
      "name": "Arroz",
      "position": 0,
      "choices": [
        {
          "id": "019a5b10-0000-7000-8000-000000000b01",
          "name": "Arroz blanco",
          "position": 0,
          "is_default": true,
          "items": [
            {
              "id": "019a5b10-0000-7000-8000-000000000c01",
              "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a1",
              "ingredient_name": "Porción arroz blanco",
              "quantity": 1,
              "created_at": "2026-11-09T15:00:00Z",
              "updated_at": "2026-11-09T15:00:00Z"
            }
          ],
          "created_at": "2026-11-09T15:00:00Z",
          "updated_at": "2026-11-09T15:00:00Z"
        },
        {
          "id": "019a5b10-0000-7000-8000-000000000b02",
          "name": "Arroz de coco",
          "position": 1,
          "is_default": false,
          "items": [
            {
              "id": "019a5b10-0000-7000-8000-000000000c02",
              "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a2",
              "ingredient_name": "Porción arroz de coco",
              "quantity": 1,
              "created_at": "2026-11-09T15:00:00Z",
              "updated_at": "2026-11-09T15:00:00Z"
            }
          ],
          "created_at": "2026-11-09T15:00:00Z",
          "updated_at": "2026-11-09T15:00:00Z"
        }
      ],
      "created_at": "2026-11-09T15:00:00Z",
      "updated_at": "2026-11-09T15:00:00Z"
    }
  ]
}
```

### Error Responses

**404 Not Found** — the product does not exist

```json
{
  "error": "Product not found"
}
```

---

## Replace Option Slots

Makes the request the plate's **whole** slot list, in one transaction: list order is display order. A slot or choice sent with the `id` of an existing slot or choice **of this plate** keeps that id, so open orders and cached ids stay valid; one sent without an id (or with an unknown one) gets a new id. A slot or choice left out is removed and can no longer be ordered; lines already taken with it keep showing it. An empty `slots` list removes every slot. A rejected request leaves the previous configuration unchanged. The change reaches the box on its next pull.

Cloud only; requires `products:update`. On the box this route returns `404`.

```
PUT /api/products/:id/option-slots
```

### Path Parameters

| Parameter | Type | Description          |
| --------- | ---- | -------------------- |
| id        | UUID | Composite product ID |

### Request Body

| Field                                       | Type    | Required | Description |
| ------------------------------------------- | ------- | -------- | ----------- |
| slots                                       | array   | Yes      | The plate's full ordered slot list (`[]` removes every slot) |
| slots[].id                                  | UUID    | No       | An existing slot of this plate, to keep its id |
| slots[].name                                | string  | Yes      | Not empty; unique within the plate (case-insensitive) |
| slots[].choices                             | array   | Yes      | At least one; exactly one with `is_default: true` |
| slots[].choices[].id                        | UUID    | No       | An existing choice of this plate, to keep its id |
| slots[].choices[].name                      | string  | Yes      | Not empty; unique within the slot (case-insensitive) |
| slots[].choices[].is_default                | boolean | No       | `true` on exactly one choice per slot |
| slots[].choices[].items                     | array   | No       | The bundle; empty or omitted means "without" |
| slots[].choices[].items[].ingredient_product_id | UUID | Yes   | A product of the same organization; at most once per bundle; not the plate itself |
| slots[].choices[].items[].quantity          | int     | Yes      | Whole units, at least 1 |

### Example Request

```json
{
  "slots": [
    {
      "name": "Arroz",
      "choices": [
        { "name": "Arroz blanco", "is_default": true, "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a1", "quantity": 1 }] },
        { "name": "Arroz de coco", "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a2", "quantity": 1 }] }
      ]
    },
    {
      "name": "Acompañante",
      "choices": [
        { "name": "Canastas", "is_default": true, "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a3", "quantity": 2 }] },
        { "name": "Papas", "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a4", "quantity": 1 }] },
        { "name": "Patacón", "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a5", "quantity": 1 }] }
      ]
    },
    {
      "name": "Ensalada",
      "choices": [
        { "name": "Ensalada", "is_default": true, "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a6", "quantity": 1 }] },
        { "name": "Canasta extra", "items": [{ "ingredient_product_id": "019a5b00-0000-7000-8000-0000000000a3", "quantity": 1 }] }
      ]
    }
  ]
}
```

### Example Response (200 OK)

The saved list, in the shape of [List Option Slots](#list-option-slots).

### Error Responses

**400 Bad Request** — the document breaks a rule; the message names the slot or choice

```json
{
  "error": "invalid plate option slots: slot \"Arroz\" must have exactly one default choice, got 2"
}
```

**400 Bad Request** — the product is not composite, or a bundle names the plate itself

```json
{
  "error": "product is not a composite product"
}
```

**404 Not Found** — the plate does not exist

```json
{
  "error": "Product not found"
}
```

**404 Not Found** — a bundle ingredient does not exist in this organization (another organization's product looks the same)

```json
{
  "error": "a bundle ingredient does not exist"
}
```

---

# Product Preparation Responsibilities API

Manage which preparation area (kitchen, bar, grill, etc.) is responsible for preparing each product. Each product can have one preparation responsibility with a priority level.

## Endpoints

| Method | Endpoint                            | Description                         |
| ------ | ----------------------------------- | ----------------------------------- |
| POST   | `/api/product-responsibilities`     | Assign preparation area to product  |
| GET    | `/api/product-responsibilities/:id` | Get preparation responsibility by ID |
| PUT    | `/api/product-responsibilities/:id` | Update preparation area or priority |
| DELETE | `/api/product-responsibilities/:id` | Remove preparation responsibility   |

---

## Create Product Responsibility

Assigns a preparation area and priority to a product.

```
POST /api/product-responsibilities
```

### Request Body

| Field        | Type   | Required | Description                        |
| ------------ | ------ | -------- | ---------------------------------- |
| product_name | string | Yes      | Exact product name (1-255 chars)   |
| area         | string | Yes      | Preparation area (1-255 chars)     |
| priority     | int    | Yes      | Priority level (0 = highest, etc.) |

### Example Request

```json
{
  "product_name": "Fish Ceviche",
  "area": "kitchen",
  "priority": 1
}
```

### Example Response (201 Created)

```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "product_id": "660e8400-e29b-41d4-a716-446655440001",
  "area": "kitchen",
  "priority": 1,
  "created_at": "2024-01-26T17:00:00Z",
  "updated_at": "2024-01-26T17:00:00Z"
}
```

### Error Responses

**404 Not Found** - Product not found

```json
{
  "error": "Product not found"
}
```

**400 Bad Request** - Invalid request body

```json
{
  "error": "Invalid request body"
}
```

---

## Get Product Responsibility by ID

Returns a single product preparation responsibility.

```
GET /api/product-responsibilities/:id
```

### Path Parameters

| Parameter | Type | Description               |
| --------- | ---- | ------------------------- |
| id        | UUID | Product responsibility ID |

### Example Response (200 OK)

```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "product_id": "660e8400-e29b-41d4-a716-446655440001",
  "area": "kitchen",
  "priority": 1,
  "created_at": "2024-01-26T17:00:00Z",
  "updated_at": "2024-01-26T17:00:00Z"
}
```

### Error Responses

**404 Not Found** - Responsibility not found

```json
{
  "error": "Product responsibility not found"
}
```

---

## Update Product Responsibility

Updates the preparation area or priority for a product.

```
PUT /api/product-responsibilities/:id
```

### Path Parameters

| Parameter | Type | Description               |
| --------- | ---- | ------------------------- |
| id        | UUID | Product responsibility ID |

### Request Body

| Field    | Type   | Required | Description                        |
| -------- | ------ | -------- | ---------------------------------- |
| area     | string | Yes      | Preparation area (1-255 chars)     |
| priority | int    | Yes      | Priority level (0 = highest, etc.) |

### Example Request

```json
{
  "area": "bar",
  "priority": 2
}
```

### Example Response (200 OK)

```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "product_id": "660e8400-e29b-41d4-a716-446655440001",
  "area": "bar",
  "priority": 2,
  "created_at": "2024-01-26T17:00:00Z",
  "updated_at": "2024-01-26T17:15:00Z"
}
```

### Error Responses

**404 Not Found** - Responsibility not found

```json
{
  "error": "Product responsibility not found"
}
```

**400 Bad Request** - Invalid request body

```json
{
  "error": "Invalid request body"
}
```

---

## Delete Product Responsibility

Removes a product's preparation responsibility assignment.

```
DELETE /api/product-responsibilities/:id
```

### Path Parameters

| Parameter | Type | Description               |
| --------- | ---- | ------------------------- |
| id        | UUID | Product responsibility ID |

### Response (204 No Content)

No response body on success.

### Error Responses

**404 Not Found** - Responsibility not found

```json
{
  "error": "Product responsibility not found"
}
```
