---
name: invoice-to-products
description: Extract products from a supplier invoice PDF and generate JSON for POST /api/products/bulk
user_invocable: true
---

# Invoice to Products Skill

Extract product line items from a Colombian supplier invoice PDF and generate the JSON body for `POST /api/products/bulk`.

## Input

The user provides:
1. A **PDF file path** of the supplier invoice
2. A **supplier ID** (UUID)
3. Optionally, a **default category** (defaults to `"SNACKS"`)

## Steps

### 1. Read the PDF

Use the Read tool to read the PDF file. The invoice follows the Colombian FACTURA ELECTRONICA DE VENTA format.

### 2. Extract product line items

From the invoice table, extract each row with these columns:

| Invoice Column | Maps To         |
|---------------|-----------------|
| DESCRIPCION   | `name`          |
| COD BARRAS    | `sku`           |
| IVA (%)       | `vat`           |
| VALOR UNITARIO | Used to compute `total_price_with_taxes` |
| CODIGO        | `supplier_sku`  |

### 3. Calculate total_price_with_taxes

For each product:
```
total_price_with_taxes = VALOR UNITARIO * (1 + IVA/100)
```

Example: VALOR UNITARIO = 5462, IVA = 19% -> total_price_with_taxes = 5462 * 1.19 = 6499.78

### 4. Check for duplicate SKUs

Use the Postgres MCP server to check for existing products:

```sql
SELECT id, name, sku FROM products WHERE sku IN ('sku1', 'sku2', ...) AND deleted_at IS NULL;
```

Report which products already exist in the database.

### 5. Apply default values

For each extracted product, apply these defaults:
- `category`: User-provided or `"SNACKS"`
- `product_type`: `"SELLABLE"`
- `unit_of_measure`: `"unit"`
- `ico`: `"0"`
- `taxes_format`: `"percentage"`

### 6. Generate JSON

Build the request body for `POST /api/products/bulk`:

```json
{
  "supplier_id": "<provided-supplier-uuid>",
  "items": [
    {
      "name": "PRODUCT NAME FROM INVOICE",
      "category": "SNACKS",
      "product_type": "SELLABLE",
      "unit_of_measure": "unit",
      "vat": "19",
      "ico": "0",
      "taxes_format": "percentage",
      "sku": "7702914551403",
      "total_price_with_taxes": "6499.78",
      "supplier_sku": "59107"
    }
  ]
}
```

### 7. Present for review

Show the user:
1. Number of products extracted
2. Any duplicate SKUs found in the database
3. The complete JSON body
4. Ask for confirmation before making the API call

## Notes

- The invoice may have products with different VAT rates (0%, 5%, 19%). Extract the correct rate per product.
- Product names should be kept as-is from the invoice (uppercase).
- If a barcode (COD BARRAS) is missing for a product, flag it for the user to provide manually.
- The ICO tax is always "0" for supplier invoice products unless the user specifies otherwise.
