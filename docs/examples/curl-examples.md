# cURL Examples

Quick copy-paste examples for testing the API.

## Setup

Set your base URL:

```bash
export BASE_URL="http://localhost:8080/api"
```

---

## Authentication

### Sign In

```bash
curl -X POST "$BASE_URL/auth/signin" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "your_username",
    "password": "your_password"
  }'
```

Save the token from the response:

```bash
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Get Current User

```bash
curl -X GET "$BASE_URL/auth/me" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Suppliers

### Create Supplier

```bash
curl -X POST "$BASE_URL/suppliers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Fresh Produce Co.",
    "contact_name": "Juan Garcia",
    "phone": "+57 300 123 4567",
    "email": "juan@freshproduce.com",
    "notes": "Delivers on Mondays and Thursdays"
  }'
```

### List Suppliers

```bash
curl -X GET "$BASE_URL/suppliers" \
  -H "Authorization: Bearer $TOKEN"
```

### Get Supplier

```bash
curl -X GET "$BASE_URL/suppliers/SUPPLIER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Update Supplier

```bash
curl -X PUT "$BASE_URL/suppliers/SUPPLIER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Fresh Produce Co. Updated",
    "contact_name": "Maria Garcia",
    "phone": "+57 300 987 6543",
    "email": "maria@freshproduce.com"
  }'
```

### Delete Supplier

```bash
curl -X DELETE "$BASE_URL/suppliers/SUPPLIER_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Supplier Catalog

### Add Product to Supplier

```bash
curl -X POST "$BASE_URL/suppliers/SUPPLIER_ID/products" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "PRODUCT_ID",
    "unit_cost": "2500.00",
    "supplier_sku": "FP-TOM-001"
  }'
```

### Update Product Pricing

```bash
curl -X PUT "$BASE_URL/suppliers/SUPPLIER_ID/products/PRODUCT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "unit_cost": "2800.00",
    "supplier_sku": "FP-TOM-002"
  }'
```

### Remove Product from Supplier

```bash
curl -X DELETE "$BASE_URL/suppliers/SUPPLIER_ID/products/PRODUCT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### List Products from Supplier

```bash
curl -X GET "$BASE_URL/suppliers/SUPPLIER_ID/products" \
  -H "Authorization: Bearer $TOKEN"
```

### List Suppliers for Product

```bash
curl -X GET "$BASE_URL/products/PRODUCT_ID/suppliers" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Purchase Entries

### Create Purchase Entry

```bash
curl -X POST "$BASE_URL/purchase-entries" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "supplier_id": "SUPPLIER_ID",
    "invoice_reference": "INV-2024-001",
    "notes": "Weekly produce delivery",
    "items": [
      {
        "product_id": "PRODUCT_ID_1",
        "quantity": "10.5",
        "unit_cost": "2500.00"
      },
      {
        "product_id": "PRODUCT_ID_2",
        "quantity": "20",
        "unit_cost": "1800.00"
      }
    ]
  }'
```

### List All Purchase Entries

```bash
curl -X GET "$BASE_URL/purchase-entries" \
  -H "Authorization: Bearer $TOKEN"
```

### Get Purchase Entry Details

```bash
curl -X GET "$BASE_URL/purchase-entries/ENTRY_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### List Purchase Entries by Supplier

```bash
curl -X GET "$BASE_URL/suppliers/SUPPLIER_ID/purchase-entries" \
  -H "Authorization: Bearer $TOKEN"
```

### Upload Single Document (PDF)

```bash
curl -X POST "$BASE_URL/purchase-entries/ENTRY_ID/documents?file_type=pdf" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/invoice.pdf"
```

### Upload Single Document (XML)

```bash
curl -X POST "$BASE_URL/purchase-entries/ENTRY_ID/documents?file_type=xml" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/invoice.xml"
```

### Upload ZIP File (Contains PDF + XML)

```bash
curl -X POST "$BASE_URL/purchase-entries/ENTRY_ID/documents" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/electronic_invoice.zip"
```

---

## Expenses

### Create Expense

```bash
curl -X POST "$BASE_URL/expenses" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "category_id": "CATEGORY_ID",
    "supplier_id": "SUPPLIER_ID",
    "amount": "500000.00",
    "description": "Monthly building rent - January 2024",
    "reference": "RENT-2024-01"
  }'
```

### List Expenses

```bash
curl -X GET "$BASE_URL/expenses" \
  -H "Authorization: Bearer $TOKEN"
```

### Upload Expense Document (PDF)

```bash
curl -X POST "$BASE_URL/expenses/EXPENSE_ID/documents?category_code=rent&file_type=pdf" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/receipt.pdf"
```

### Upload Expense Document (XML)

```bash
curl -X POST "$BASE_URL/expenses/EXPENSE_ID/documents?category_code=rent&file_type=xml" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/receipt.xml"
```

### Upload Expense ZIP File (Contains PDF + XML)

```bash
curl -X POST "$BASE_URL/expenses/EXPENSE_ID/documents?category_code=rent" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/electronic_invoice.zip"
```

---

## Products

### Create Sellable Product

```bash
curl -X POST "$BASE_URL/products" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
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
  }'
```

### Create Ingredient Product

```bash
curl -X POST "$BASE_URL/products" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Tomato",
    "category": "Vegetables",
    "product_type": "INGREDIENT",
    "unit_of_measure": "kg",
    "sku": "VEG-TOM-001",
    "description": "Fresh tomatoes for cooking"
  }'
```

### List Products

```bash
curl -X GET "$BASE_URL/products" \
  -H "Authorization: Bearer $TOKEN"
```

### Get Product

```bash
curl -X GET "$BASE_URL/products/PRODUCT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

### Delete Product

```bash
curl -X DELETE "$BASE_URL/products/PRODUCT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Product Ingredients (Composite Products)

### Create Composite Product

```bash
curl -X POST "$BASE_URL/products" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Fish Plate",
    "category": "Main Dishes",
    "product_type": "COMPOSITE",
    "unit_of_measure": "unit",
    "sku": "DISH-FISH-001",
    "description": "Grilled fish with rice and vegetables",
    "total_price_with_taxes": "35000",
    "vat": "19",
    "ico": "8",
    "taxes_format": "percentage"
  }'
```

### Add Ingredient to Composite Product

```bash
curl -X POST "$BASE_URL/products/COMPOSITE_PRODUCT_ID/ingredients" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "ingredient_product_id": "INGREDIENT_PRODUCT_ID",
    "quantity": "0.5"
  }'
```

### List Ingredients of Composite Product

```bash
curl -X GET "$BASE_URL/products/COMPOSITE_PRODUCT_ID/ingredients" \
  -H "Authorization: Bearer $TOKEN"
```

### Update Ingredient Quantity

```bash
curl -X PUT "$BASE_URL/products/COMPOSITE_PRODUCT_ID/ingredients/INGREDIENT_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "quantity": "0.75"
  }'
```

### Remove Ingredient from Composite Product

```bash
curl -X DELETE "$BASE_URL/products/COMPOSITE_PRODUCT_ID/ingredients/INGREDIENT_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Tips

### Pretty Print JSON

Add `| jq` to any command to format the JSON output:

```bash
curl -X GET "$BASE_URL/suppliers" \
  -H "Authorization: Bearer $TOKEN" | jq
```

### Get Only HTTP Status Code

```bash
curl -s -o /dev/null -w "%{http_code}" -X GET "$BASE_URL/suppliers" \
  -H "Authorization: Bearer $TOKEN"
```
