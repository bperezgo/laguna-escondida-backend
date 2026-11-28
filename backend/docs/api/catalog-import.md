# Catalog Import API

Build a restaurant's catalog (ingredients, sellables, plates with recipes and option slots, preparation areas and opening stock) from one document. You check the document first, then apply it, and the apply writes everything in one transaction or nothing.

**Cloud only.** Both routes exist only in the cloud. On the box (edge) they return `404`, like every other catalog write.

**Admin only.** Both routes need the `catalog:import` permission, which only the admin role holds. The dry run needs it too, because it reads the organization's catalog to report conflicts. The organization comes from the token, never from the document.

## Endpoints

| Method | Endpoint                       | Description                                         |
| ------ | ------------------------------ | --------------------------------------------------- |
| POST   | `/api/catalog/import/validate` | Check a document and return its problems and digest |
| POST   | `/api/catalog/import`          | Apply a validated document                          |

The two routes are separate on purpose. A forgotten flag can never turn a check into a write.

---

## The document

Entries point at each other by a local `ref`, never by product id. A ref is unique within the document. The order of sections and entries does not matter: a plate may name an ingredient defined further down.

An import **only creates products**. It never edits, merges or deletes one. To use a product that already exists (for example in a recipe), list it under `existing`. A new entry whose name or SKU matches an existing product is a conflict, not a silent reuse. So applying the same document twice fails with `name_conflict` on the second try and creates nothing.

Unknown fields are refused with `400`, for example `"product_id"` on a recipe line.

| Section         | Type   | Description                                                                |
| --------------- | ------ | -------------------------------------------------------------------------- |
| `defaults`      | object | Values for entries that leave them out                                     |
| `existing`      | array  | Existing products of the organization the document links to                |
| `ingredients`   | array  | New `INGREDIENT` (default) or `BOTH` products                              |
| `sellables`     | array  | New `SELLABLE` products. No recipe or slots                                |
| `plates`        | array  | New `COMPOSITE` products, each with an optional `recipe` and `slots`       |
| `opening_stock` | array  | Opening counts for products **this document creates**                      |

### `defaults`

| Field             | Type    | Description                                     |
| ----------------- | ------- | ----------------------------------------------- |
| `vat`             | number  | VAT percentage, e.g. `0`                        |
| `ico`             | number  | ICO percentage, e.g. `8`                        |
| `taxes_format`    | string  | Must be `percentage`                            |
| `unit_of_measure` | string  | `unit`, `kg`, `g`, `l` or `ml`                  |
| `prep_area`       | string  | Preparation area for sellable entries, e.g. `kitchen` |
| `prep_priority`   | integer | Preparation priority, `0` or more               |

The server never guesses. A field missing from both the entry and `defaults` is a `missing_field` problem.

### `existing[]`

| Field  | Type   | Required    | Description                                                     |
| ------ | ------ | ----------- | --------------------------------------------------------------- |
| `ref`  | string | Yes         | Local ref                                                       |
| `sku`  | string | One of them | Exact SKU                                                       |
| `name` | string | One of them | Name, compared trimmed and case-insensitively                   |

Only the caller's organization is searched. A product of another organization gives `existing_not_found`, exactly like a missing one.

### Product fields (`ingredients[]`, `sellables[]`, `plates[]`)

| Field                    | Type   | Required    | Description                                                                           |
| ------------------------ | ------ | ----------- | ------------------------------------------------------------------------------------- |
| `ref`                    | string | Yes         | Local ref, unique in the document                                                     |
| `name`                   | string | Yes         | 1-255 characters                                                                      |
| `category`               | string | Yes         | 1-100 characters                                                                      |
| `product_type`           | string | No          | Ingredients only: `INGREDIENT` (default) or `BOTH`                                    |
| `unit_of_measure`        | string | Defaultable | `unit`, `kg`, `g`, `l` or `ml`                                                        |
| `sku`                    | string | No          | Letters, digits and `-`. Generated when left out (see below)                          |
| `description`            | string | No          |                                                                                       |
| `total_price_with_taxes` | number | Sold types  | Required for sellables, plates and `BOTH`. Refused on an `INGREDIENT`                 |
| `vat`, `ico`             | number | Defaultable | Sold types only. They cannot both be `0`                                              |
| `taxes_format`           | string | Defaultable | Sold types only. `percentage`                                                         |
| `prep`                   | object | Defaultable | Sold types only: `{ "area": "kitchen", "priority": 0 }`. Refused on an `INGREDIENT`   |

**Generated SKUs.** An entry without a SKU gets one from the `product-sku` rule: the prep area's prefix (`COC`, `BAR`, `PAR`, `SUS`, `POS`, or the area's first letters; `ING` for an ingredient without an area, `PRD` otherwise), then up to three name words, for example `COC-BAN-PAI`. A collision adds `-2`, `-3`, and so on. Given SKUs are reserved before any is generated, so a generated SKU never takes one written further down. Generated SKUs are listed in the summary.

### `plates[].recipe[]`

| Field        | Type   | Description                                       |
| ------------ | ------ | ------------------------------------------------- |
| `ingredient` | string | Ref of an `INGREDIENT` or `BOTH` product          |
| `quantity`   | number | Greater than 0, in the ingredient's unit          |

### `plates[].slots[]`

| Field                          | Type    | Description                                                           |
| ------------------------------ | ------- | --------------------------------------------------------------------- |
| `name`                         | string  | Slot name, unique in the plate                                        |
| `choices[].name`               | string  | Choice name, unique in the slot                                       |
| `choices[].default`            | boolean | Exactly one choice per slot is the default                            |
| `choices[].items[].ingredient` | string  | Ref of an `INGREDIENT` or `BOTH` product                              |
| `choices[].items[].quantity`   | integer | Whole number of g, ml or units, at least 1                            |

A choice with no items is allowed (the plate served without that slot).

### `opening_stock[]`

| Field     | Type    | Description                                                  |
| --------- | ------- | ------------------------------------------------------------ |
| `product` | string  | Ref of a product this document creates (not an `existing` one) |
| `counted` | integer | Whole number, `0` or more, in the product's unit             |

Each count is recorded like any cloud count: the stock row plus a `count` movement of `counted - on-hand`, where on-hand is 0 for a new product.

### Limits

| Limit                                         | Value  | Result                    |
| --------------------------------------------- | ------ | ------------------------- |
| Request body                                  | 2 MiB  | `413`                     |
| Products in total (new plus `existing`)       | 1,000  | `limit_exceeded`          |
| Recipe lines per plate                        | 50     | `limit_exceeded`          |
| Option slots per plate                        | 20     | `limit_exceeded`          |
| Choices per slot                              | 30     | `limit_exceeded`          |
| Items per choice                              | 50     | `limit_exceeded`          |
| Opening-stock entries                         | 1,000  | `limit_exceeded`          |

When a limit is exceeded, only the limit problems are reported and the rest of the document is not checked.

---

## Validate a document

Checks the whole document against the caller's organization and writes nothing. Every problem is reported, not only the first.

```
POST /api/catalog/import/validate
```

### Example Request

```json
{
  "defaults": {
    "vat": 0,
    "ico": 8,
    "taxes_format": "percentage",
    "unit_of_measure": "g",
    "prep_area": "kitchen",
    "prep_priority": 0
  },
  "existing": [
    { "ref": "limon", "name": "Limón" }
  ],
  "ingredients": [
    { "ref": "arroz-blanco", "name": "Arroz blanco", "category": "Insumos" },
    { "ref": "arroz-coco", "name": "Arroz de coco", "category": "Insumos" },
    { "ref": "tilapia-cruda", "name": "Tilapia entera", "category": "Insumos", "unit_of_measure": "unit" },
    { "ref": "patacon", "name": "Patacón", "category": "Acompañantes", "product_type": "BOTH",
      "unit_of_measure": "unit", "total_price_with_taxes": 4000 }
  ],
  "sellables": [
    { "ref": "coca-400", "name": "Coca-Cola 400ml", "category": "Bebidas", "unit_of_measure": "unit",
      "total_price_with_taxes": 5000, "prep": { "area": "bar" } }
  ],
  "plates": [
    {
      "ref": "tilapia", "name": "Tilapia frita", "category": "Platos", "unit_of_measure": "unit",
      "total_price_with_taxes": 32000, "sku": "COC-TIL-FRI",
      "recipe": [
        { "ingredient": "tilapia-cruda", "quantity": 1 },
        { "ingredient": "limon", "quantity": 0.5 }
      ],
      "slots": [
        {
          "name": "Arroz",
          "choices": [
            { "name": "Arroz blanco", "default": true, "items": [{ "ingredient": "arroz-blanco", "quantity": 200 }] },
            { "name": "Arroz de coco", "items": [{ "ingredient": "arroz-coco", "quantity": 200 }] },
            { "name": "Sin arroz" }
          ]
        }
      ]
    }
  ],
  "opening_stock": [
    { "product": "arroz-blanco", "counted": 25000 },
    { "product": "tilapia-cruda", "counted": 40 }
  ]
}
```

### Response (200 OK), valid document

```json
{
  "valid": true,
  "applied": false,
  "problems": [],
  "summary": {
    "products": 6,
    "ingredients": 4,
    "sellables": 1,
    "plates": 1,
    "existing": 1,
    "recipe_lines": 2,
    "option_slots": 1,
    "option_choices": 3,
    "preparation_areas": 3,
    "opening_stock": 2,
    "generated_skus": [
      { "ref": "arroz-blanco", "sku": "ING-ARR-BLA" },
      { "ref": "arroz-coco", "sku": "ING-ARR-COC" },
      { "ref": "tilapia-cruda", "sku": "ING-TIL-ENT" },
      { "ref": "patacon", "sku": "COC-PAT" },
      { "ref": "coca-400", "sku": "BAR-COC-COL-400" }
    ]
  },
  "digest": "ed8cd33c687104c654c4af77f1e27de97380de0ea2d1d6552f2304b866c46983"
}
```

`summary.products` counts only the products the document creates. `preparation_areas` counts the preparation responsibilities it creates (one per sold entry).

### Response (200 OK), document with problems

A document with problems still answers `200`, with `valid: false`. `path` is a JSON pointer to the offending entry, line, slot, choice or item.

```json
{
  "valid": false,
  "applied": false,
  "problems": [
    {
      "path": "/sellables/0",
      "code": "name_conflict",
      "message": "a product named \"Coca-Cola 400ml\" already exists; rename this entry or list the product under existing"
    },
    {
      "path": "/plates/0/recipe/0",
      "code": "unknown_ref",
      "message": "no entry or existing item defines the ref \"yuca\""
    },
    {
      "path": "/plates/0/slots/0/choices/0/items/0",
      "code": "invalid_quantity",
      "message": "a choice quantity must be a whole number from 1 to 1000000; express it in g, ml or units"
    },
    {
      "path": "/plates/0/slots/0",
      "code": "slot_default_count",
      "message": "the slot must have exactly one default choice, it has 0"
    }
  ],
  "summary": {
    "products": 3,
    "ingredients": 1,
    "sellables": 1,
    "plates": 1,
    "existing": 0,
    "recipe_lines": 1,
    "option_slots": 1,
    "option_choices": 2,
    "preparation_areas": 2,
    "opening_stock": 0,
    "generated_skus": [
      { "ref": "arroz", "sku": "ING-ARR" },
      { "ref": "coca", "sku": "COC-COC-COL-400" },
      { "ref": "p", "sku": "COC-MOJ-FRI" }
    ]
  },
  "digest": "eaa4e8795f16f035128533094a5699090aa7a5a8335342c1b138810d20eb735e"
}
```

### The digest

`digest` is the SHA-256 of the document with `defaults` folded into each entry and the keys in a fixed order. Key order and whitespace in the request do not change it, and any changed value does. Apply requires it as `expected_digest`. That ties the operator's approval of the summary to the exact document applied. It guards against drift between turns of an AI client. It does not authenticate anything.

---

## Apply a document

Validates the document again inside one transaction, against the organization as it is now. Only if there is no problem and the digest matches does it write, in this order: products (ingredients, then sellables, then plates), recipe lines, option slots, preparation responsibilities, opening-stock counts. Any problem or failed write leaves nothing behind.

```
POST /api/catalog/import
```

### Request Body

The same document, plus:

| Field             | Type   | Required | Description                                     |
| ----------------- | ------ | -------- | ----------------------------------------------- |
| `expected_digest` | string | Yes      | The `digest` the validation returned            |

```json
{
  "expected_digest": "ed8cd33c687104c654c4af77f1e27de97380de0ea2d1d6552f2304b866c46983",
  "defaults": { "vat": 0, "ico": 8, "taxes_format": "percentage", "unit_of_measure": "g", "prep_area": "kitchen", "prep_priority": 0 },
  "existing": [{ "ref": "limon", "name": "Limón" }],
  "ingredients": ["…the same entries as validated…"]
}
```

### Response (201 Created)

The validation result with `applied: true` and `products`, the id and SKU of every created product keyed by ref. `existing` refs are not listed.

```json
{
  "valid": true,
  "applied": true,
  "problems": [],
  "summary": {
    "products": 6,
    "ingredients": 4,
    "sellables": 1,
    "plates": 1,
    "existing": 1,
    "recipe_lines": 2,
    "option_slots": 1,
    "option_choices": 3,
    "preparation_areas": 3,
    "opening_stock": 2,
    "generated_skus": [
      { "ref": "arroz-blanco", "sku": "ING-ARR-BLA" },
      { "ref": "arroz-coco", "sku": "ING-ARR-COC" },
      { "ref": "tilapia-cruda", "sku": "ING-TIL-ENT" },
      { "ref": "patacon", "sku": "COC-PAT" },
      { "ref": "coca-400", "sku": "BAR-COC-COL-400" }
    ]
  },
  "digest": "ed8cd33c687104c654c4af77f1e27de97380de0ea2d1d6552f2304b866c46983",
  "products": {
    "arroz-blanco": { "id": "019a7c3e-1f2a-7b41-9c11-5a3e2d4b6c01", "sku": "ING-ARR-BLA" },
    "arroz-coco": { "id": "019a7c3e-1f2a-7b41-9c11-5a3e2d4b6c02", "sku": "ING-ARR-COC" },
    "tilapia-cruda": { "id": "019a7c3e-1f2a-7b41-9c11-5a3e2d4b6c03", "sku": "ING-TIL-ENT" },
    "patacon": { "id": "019a7c3e-1f2a-7b41-9c11-5a3e2d4b6c04", "sku": "COC-PAT" },
    "coca-400": { "id": "019a7c3e-1f2a-7b41-9c11-5a3e2d4b6c05", "sku": "BAR-COC-COL-400" },
    "tilapia": { "id": "019a7c3e-1f2a-7b41-9c11-5a3e2d4b6c06", "sku": "COC-TIL-FRI" }
  }
}
```

### Response (409 Conflict), digest mismatch

The document differs from the one validated, or `expected_digest` is missing. Nothing is written. Validate again and show the operator the new summary.

```json
{
  "valid": false,
  "applied": false,
  "problems": [
    {
      "path": "/expected_digest",
      "code": "digest_mismatch",
      "message": "the document differs from the one validated; validate it again and confirm the new summary"
    }
  ],
  "summary": { "products": 0, "ingredients": 0, "sellables": 0, "plates": 0, "existing": 0, "recipe_lines": 0, "option_slots": 0, "option_choices": 0, "preparation_areas": 0, "opening_stock": 0, "generated_skus": [] },
  "digest": "3b1f0c9d8e7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c"
}
```

### Response (422 Unprocessable Entity), problems found

The digest matched, but validating inside the transaction found problems, for example because a product with the same name was created since the dry run. The body has the same shape as a validation with problems. Nothing is written.

---

## Problem codes

| Code                 | Meaning                                                                                   |
| -------------------- | ----------------------------------------------------------------------------------------- |
| `duplicate_ref`      | The ref is already used by another entry (reported on the later one)                      |
| `unknown_ref`        | A recipe line, choice item or opening count names a ref nothing defines                   |
| `existing_not_found` | No product of this organization has that SKU or name                                      |
| `existing_ambiguous` | Several products of this organization have that name. Name it by SKU                      |
| `name_conflict`      | A new entry's name (trimmed, case-insensitive) matches an existing product or another entry |
| `sku_conflict`       | A given SKU (case-insensitive) matches an existing product or another entry              |
| `missing_field`      | A required value is missing from the entry and from `defaults`. The message names the field |
| `invalid_field`      | A value breaks a product rule (bad SKU, price 0, VAT and ICO both 0, a price on an `INGREDIENT`, …) |
| `invalid_unit`       | `unit_of_measure` is not `unit`, `kg`, `g`, `l` or `ml`                                    |
| `invalid_link_type`  | A recipe line or choice item points at a `SELLABLE` or `COMPOSITE` product                 |
| `invalid_quantity`   | A recipe quantity is not above 0, a choice quantity is not a whole number from 1, or a count is negative or fractional |
| `duplicate_item`     | A recipe, choice or `opening_stock` lists the same ref twice                               |
| `invalid_slot`       | A slot or choice has no name, a duplicate name, or the slot has no choices                 |
| `slot_default_count` | A slot does not have exactly one default choice                                            |
| `stock_on_existing`  | `opening_stock` names an `existing` ref. Count existing products from the stock screen     |
| `limit_exceeded`     | The document is larger than a limit above                                                  |
| `digest_mismatch`    | Apply only: `expected_digest` does not match the document                                  |

---

## Errors

| Status | When                                                                                   |
| ------ | -------------------------------------------------------------------------------------- |
| 400    | The body is not JSON, has an unknown field or a wrong type, or has data after the document |
| 401    | No valid token                                                                         |
| 403    | The caller lacks `catalog:import` (every role but admin)                               |
| 404    | Called on a box (edge)                                                                 |
| 409    | Apply: `digest_mismatch`                                                               |
| 413    | The body is larger than 2 MiB                                                          |
| 422    | Apply: the document has problems                                                       |
| 500    | Unexpected failure. On apply, nothing was written                                      |

```json
{ "error": "Invalid catalog import document: json: unknown field \"product_id\"" }
```
