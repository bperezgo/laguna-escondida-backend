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

| Method | Endpoint                                      | Description                             |
| ------ | --------------------------------------------- | --------------------------------------- |
| POST   | `/api/products`                               | Create a new product                    |
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
  "quantity": "2.5",
  "created_at": "2024-01-26T16:00:00Z",
  "updated_at": "2024-01-26T16:00:00Z"
}
```

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
      "quantity": "2.5",
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
  "quantity": "3.0",
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
3. The quantity decreased is: `ingredient.quantity * order_quantity`

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
