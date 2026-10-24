# cURL Examples

Quick copy-paste examples for testing the API.

## Setup

Set your base URL:

```bash
export BASE_URL="http://localhost:8080/api"
```

For admin-only endpoints (e.g. creating users), set the admin API key:

```bash
export ADMIN_API_KEY="your_admin_api_key"
```

---

## Authentication

### Sign In

```bash
curl -X POST "$BASE_URL/auth/signin" \
  -H "Content-Type: application/json" \
  -d '{
    "organization": "laguna-escondida",
    "username": "your_username",
    "password": "your_password"
  }'
```

`organization` is the organization's slug. It is required in the cloud; on an edge box it can be
left out, and when sent it must be the box's own. Tokens issued before organizations were added
are rejected: sign in again to get one that carries `org_id`.

Save the token from the response:

```bash
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Get Current User

```bash
curl -X GET "$BASE_URL/auth/me" \
  -H "Authorization: Bearer $TOKEN"
```

Only the organization (its branding, formatting and receipt settings):

```bash
curl -s "$BASE_URL/auth/me" \
  -H "Authorization: Bearer $TOKEN" | jq '.organization'
```

---

## Users

### Create User (admin only)

Uses the admin API key via the `X-API-Key` header, not a JWT. `name` is required, and in the
cloud so is `organization` (the slug), since the admin key belongs to no organization.

```bash
curl -X POST "$BASE_URL/users" \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "organization": "laguna-escondida",
    "username": "john_doe",
    "name": "John Doe",
    "password": "securepassword123",
    "role_ids": [2, 3]
  }'
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

### Export Expenses CSV (all)

```bash
curl -X GET "$BASE_URL/expenses/export" \
  -H "Authorization: Bearer $TOKEN" \
  -o gastos.csv
```

### Export Expenses CSV (filtered by date range)

```bash
curl -X GET "$BASE_URL/expenses/export?start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN" \
  -o gastos_2024.csv
```

### Export Expenses CSV (filtered by category)

```bash
curl -X GET "$BASE_URL/expenses/export?category_id=CATEGORY_ID&start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN" \
  -o gastos_categoria.csv
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

### Bulk Create Products (with supplier linking)

```bash
curl -X POST "$BASE_URL/products/bulk" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "supplier_id": "SUPPLIER_ID",
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
  }'
```

### List Products

```bash
curl -X GET "$BASE_URL/products" \
  -H "Authorization: Bearer $TOKEN"
```

### List Products filtered by type (for order creation)

```bash
curl -X GET "$BASE_URL/products?product_type=SELLABLE,BOTH" \
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

### Mark an Ingredient as a Side Dish (default 1, min 0, max 2)

```bash
curl -X PUT "$BASE_URL/products/COMPOSITE_PRODUCT_ID/ingredients/INGREDIENT_ID/side-dish" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_quantity": 1,
    "min_quantity": 0,
    "max_quantity": 2
  }'
```

### Clear an Ingredient's Side-Dish Flag

```bash
curl -X DELETE "$BASE_URL/products/COMPOSITE_PRODUCT_ID/ingredients/INGREDIENT_ID/side-dish" \
  -H "Authorization: Bearer $TOKEN"
```

### List a Composite's Side-Dish Options

```bash
curl -X GET "$BASE_URL/products/COMPOSITE_PRODUCT_ID/side-dishes" \
  -H "Authorization: Bearer $TOKEN"
```

### Create an Order with Side-Dish Selections (drop salad, 3 canasta)

```bash
curl -X POST "$BASE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "open_bill_id": "550e8400-e29b-41d4-a716-446655440000",
    "temporal_identifier": "660e8400-e29b-41d4-a716-446655440001",
    "descriptor": "Mesa 5",
    "products": [
      {
        "open_bill_product_id": "770e8400-e29b-41d4-a716-446655440002",
        "product_id": "COMPOSITE_PRODUCT_ID",
        "quantity": 1,
        "side_dishes": [
          { "ingredient_product_id": "SALAD_INGREDIENT_ID", "quantity": 0 },
          { "ingredient_product_id": "CANASTA_INGREDIENT_ID", "quantity": 3 }
        ]
      }
    ]
  }'
```

### Create an Order That Is Safe to Retry

Repeat this exact request with the same key: the second response is the same order, with `Idempotent-Replayed: true`.

```bash
IDEMPOTENCY_KEY=$(uuidgen)

curl -i -X POST "$BASE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "X-Idempotency-Key: $IDEMPOTENCY_KEY" \
  -d '{
    "open_bill_id": "550e8400-e29b-41d4-a716-446655440000",
    "temporal_identifier": "660e8400-e29b-41d4-a716-446655440001",
    "descriptor": "Mesa 5",
    "products": []
  }'
```

### Complete a Kitchen Line After the Order Was Paid

Paying closes the order, but its kitchen lines keep cooking. The kitchen still completes them:

```bash
# 1. Pay the order (manager/admin)
curl -X POST "$BASE_URL/orders/pay-order" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "550e8400-e29b-41d4-a716-446655440000",
    "payment_type": "cash"
  }'

# 2. Complete the line afterwards (cooker/manager/admin) -> 200
curl -X PATCH "$BASE_URL/orders/550e8400-e29b-41d4-a716-446655440000/products/770e8400-e29b-41d4-a716-446655440002/complete" \
  -H "Authorization: Bearer $TOKEN"

# Cancelling a line of a paid order is refused -> 409
curl -X PATCH "$BASE_URL/orders/550e8400-e29b-41d4-a716-446655440000/products/770e8400-e29b-41d4-a716-446655440002/cancel" \
  -H "Authorization: Bearer $TOKEN"
```

### Create a Delivery Order (held until paid)

Requires at least one active delivery zone containing the pin (cloud only, see Delivery Zones below). The phone is normalized to `+573005551234`; an outside pin returns `422 OUTSIDE_DELIVERY_ZONE`. Sent to the box instead of the cloud, it returns `422 DELIVERY_CLOUD_ONLY`:

```bash
export EDGE_URL="http://localhost:8082/api"
curl -X POST "$EDGE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"open_bill_id":"019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d55","temporal_identifier":"019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d56","order_type":"delivery","customer_phone":"300 555 1234","delivery_address":"Cra 7 # 12-34","delivery_lat":"4.598100","delivery_lng":"-74.076000","products":[]}'
# 422 {"error":"Deliveries are taken in the cloud","code":"DELIVERY_CLOUD_ONLY"}
```

Against the cloud:

```bash
curl -X POST "$BASE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
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
        "product_id": "PRODUCT_ID",
        "quantity": 1
      }
    ]
  }'
```

### Create a Takeaway Order

```bash
curl -X POST "$BASE_URL/orders" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "open_bill_id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d60",
    "temporal_identifier": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d61",
    "order_type": "takeaway",
    "customer_phone": "+34 612 345 678",
    "products": [
      { "open_bill_product_id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d62", "product_id": "PRODUCT_ID", "quantity": 2 }
    ]
  }'
```

### List Delivery Orders (unpaid, and paid today)

```bash
# Unpaid, still active
curl -X GET "$BASE_URL/orders?order_type=delivery" \
  -H "Authorization: Bearer $TOKEN"

# Paid (or voided) today; paid ones carry paid_at
curl -X GET "$BASE_URL/orders/closed?order_type=delivery" \
  -H "Authorization: Bearer $TOKEN"

# One customer's active orders, any phone format (URL-encode the +)
curl -X GET "$BASE_URL/orders?customer_phone=%2B573005551234" \
  -H "Authorization: Bearer $TOKEN"
```

### Release a Held Order to the Kitchen by Paying It

```bash
# Before payment the kitchen cannot start it -> 409 ORDER_AWAITING_PAYMENT
curl -X PATCH "$BASE_URL/orders/019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d50/products/019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d52/in-progress" \
  -H "Authorization: Bearer $TOKEN"

# Paying puts the lines on the kitchen board, waiting time from now
curl -X POST "$BASE_URL/orders/pay-order" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "019a1f3c-5b2e-7c41-9a8d-3e6f1b2c4d50",
    "payment_type": "cash"
  }'
```

---

## Delivery Zones (cloud only)

### Create a Zone (admin/manager)

Positions are `[longitude, latitude]`.

```bash
curl -X POST "$BASE_URL/delivery-zones" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Centro",
    "boundary": {
      "type": "Polygon",
      "coordinates": [[[-74.10, 4.58], [-74.06, 4.58], [-74.06, 4.62], [-74.10, 4.62], [-74.10, 4.58]]]
    }
  }'
```

### Check the Restaurant's Own Pin After Uploading

```bash
curl -X GET "$BASE_URL/delivery-zones/check?lat=4.5981&lng=-74.076" \
  -H "Authorization: Bearer $TOKEN"
```

### List Zones

```bash
curl -X GET "$BASE_URL/delivery-zones" \
  -H "Authorization: Bearer $TOKEN"
```

### Deactivate a Zone (stops new deliveries there)

```bash
curl -X PUT "$BASE_URL/delivery-zones/ZONE_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "active": false }'
```

### Delete a Zone

```bash
curl -X DELETE "$BASE_URL/delivery-zones/ZONE_ID" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Platform Organizations (cloud only, admin API key)

See `docs/api/organizations.md`. New organizations can't be used until tenancy steps 2–4 land.

### Create an Organization (with e-invoicing prefixes)

```bash
curl -X POST "$BASE_URL/platform/organizations" \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "El Muelle S.A.S",
    "slug": "el-muelle",
    "display_name": "El Muelle",
    "primary_color": "#0E7C86",
    "einvoicing": { "invoice_prefix": "MUE", "support_document_prefix": "DSM" }
  }'
```

### List Organizations

```bash
curl -X GET "$BASE_URL/platform/organizations" \
  -H "X-API-Key: $ADMIN_API_KEY"
```

### Get an Organization

```bash
curl -X GET "$BASE_URL/platform/organizations/ORG_ID" \
  -H "X-API-Key: $ADMIN_API_KEY"
```

### Update Settings (only the fields sent change; "" clears an optional field)

```bash
curl -X PATCH "$BASE_URL/platform/organizations/ORG_ID" \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "ticket_footer": "¡Gracias por su visita!", "logo_url": "" }'
```

### Suspend an Organization (there is no delete)

```bash
curl -X PATCH "$BASE_URL/platform/organizations/ORG_ID" \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "status": "suspended" }'
```

### Replace the E-Invoicing Prefixes (only for a new DIAN resolution)

```bash
curl -X PUT "$BASE_URL/platform/organizations/ORG_ID/einvoicing" \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "invoice_prefix": "MUE", "support_document_prefix": "DSM" }'
```

---

## Financial Summary

### Get Financial Summary for a Date Range

```bash
curl -X GET "$BASE_URL/financial/summary?start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN"
```

### Get Financial Summary for Current Month

```bash
curl -X GET "$BASE_URL/financial/summary?start_date=2024-06-01T00:00:00Z&end_date=2024-06-30T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN"
```

---

## Invoices

### Create Electronic Invoice (anonymous, cash)

```bash
curl -X POST "$BASE_URL/invoices" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payment_code": "cash",
    "items": [
      { "product_id": "PRODUCT_ID_1", "quantity": 2 },
      { "product_id": "PRODUCT_ID_2", "quantity": 1 }
    ]
  }'
```

### Create Electronic Invoice (identified customer, credit card)

```bash
curl -X POST "$BASE_URL/invoices" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payment_code": "credit_card",
    "customer": {
      "id": "1234567890",
      "document_type": "CC",
      "name": "Juan García",
      "email": "juan.garcia@email.com"
    },
    "items": [
      { "product_id": "PRODUCT_ID_1", "quantity": 1 }
    ]
  }'
```

### List Invoices (paginated)

```bash
curl -X GET "$BASE_URL/invoices?page=1&page_size=20" \
  -H "Authorization: Bearer $TOKEN"
```

### List Invoices by Date Range

```bash
curl -X GET "$BASE_URL/invoices?created_at_start=2024-01-01T00:00:00Z&created_at_end=2024-01-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN"
```

### List Invoices by Customer Identification

```bash
curl -X GET "$BASE_URL/invoices?national_identification=1234567890" \
  -H "Authorization: Bearer $TOKEN"
```

### Export Invoices CSV (all)

```bash
curl -X GET "$BASE_URL/invoices/export" \
  -H "Authorization: Bearer $TOKEN" \
  -o facturas.csv
```

### Export Invoices CSV (filtered by date range)

```bash
curl -X GET "$BASE_URL/invoices/export?created_at_start=2024-01-01T00:00:00Z&created_at_end=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN" \
  -o facturas_2024.csv
```

---

## Purchase Entries with Date Filtering

### List Purchase Entries by Date Range

```bash
curl -X GET "$BASE_URL/purchase-entries?start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN"
```

### List Purchase Entries by Supplier and Date Range

```bash
curl -X GET "$BASE_URL/purchase-entries?supplier_id=SUPPLIER_ID&start_date=2024-01-01T00:00:00Z&end_date=2024-06-30T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN"
```

### Export Purchase Entries CSV (all)

```bash
curl -X GET "$BASE_URL/purchase-entries/export" \
  -H "Authorization: Bearer $TOKEN" \
  -o entradas_compra.csv
```

### Export Purchase Entries CSV (filtered by date range)

```bash
curl -X GET "$BASE_URL/purchase-entries/export?start_date=2024-01-01T00:00:00Z&end_date=2024-12-31T23:59:59Z" \
  -H "Authorization: Bearer $TOKEN" \
  -o entradas_compra_2024.csv
```

### Export Purchase Entries CSV (filtered by supplier)

```bash
curl -X GET "$BASE_URL/purchase-entries/export?supplier_id=SUPPLIER_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -o entradas_proveedor.csv
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

---

## Device (Ticket Printing)

> Edge binary only. See `docs/api/device.md`.

### Print a ticket for an open bill

```bash
curl -X POST "$BASE_URL/device/print" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "open_bill_id": "550e8400-e29b-41d4-a716-446655440099",
    "copies": 1
  }'
```

### Print two copies

```bash
curl -X POST "$BASE_URL/device/print" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "open_bill_id": "550e8400-e29b-41d4-a716-446655440099", "copies": 2 }'
```

---

## Telemetry Ingest

> Cloud binary only, and only when `TELEMETRY_FORWARD_URL` points at the
> cloud Alloy sidecar. See `docs/api/telemetry.md`. The real client is the edge Alloy
> sidecar — these examples are for smoke-testing the route.

Set the node key the edge authenticates with:

```bash
export NODE_SYNC_KEY="your_node_sync_key"
```

### Relay an OTLP logs export

The body is raw OTLP protobuf, so send it from a file rather than inline. `export.pb` is a
serialized `ExportLogsServiceRequest` (e.g. captured from Alloy, or produced by
`telemetrygen`).

```bash
curl -X POST "$BASE_URL/telemetry/v1/logs" \
  -H "X-Node-Key: $NODE_SYNC_KEY" \
  -H "Content-Type: application/x-protobuf" \
  --data-binary @export.pb
```

### Relay a gzipped traces export (what Alloy sends)

```bash
gzip -c export.pb | curl -X POST "$BASE_URL/telemetry/v1/traces" \
  -H "X-Node-Key: $NODE_SYNC_KEY" \
  -H "Content-Type: application/x-protobuf" \
  -H "Content-Encoding: gzip" \
  --data-binary @-
```

### Relay a metrics export

```bash
curl -X POST "$BASE_URL/telemetry/v1/metrics" \
  -H "X-Node-Key: $NODE_SYNC_KEY" \
  -H "Content-Type: application/x-protobuf" \
  --data-binary @export.pb
```

### Check auth is rejecting a bad node key (expects 401)

```bash
export BAD_NODE_KEY="not-the-real-key"

curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/telemetry/v1/logs" \
  -H "X-Node-Key: $BAD_NODE_KEY" \
  -H "Content-Type: application/x-protobuf" \
  --data-binary @export.pb
```

---

## Stock

On-hand is **cloud-owned**. Reads answer on both nodes; every write answers only on the
cloud and returns `404` on an edge node. Point `$BASE_URL` at the cloud for this section.
See `docs/api/stock.md`.

### List stock

```bash
curl -s "$BASE_URL/stock" \
  -H "Authorization: Bearer $TOKEN"
```

### Check how current the cloud's view of the restaurant is

Read this **before** offering a batch count: sales the restaurant has not replicated yet are
missing from the cloud's number and would be subtracted twice by a count taken now.

```bash
curl -s "$BASE_URL/stock/sync-freshness" \
  -H "Authorization: Bearer $TOKEN"
```

```json
{ "last_movement_applied_at": "2026-09-22T18:08:00.048818Z", "staleness_seconds": 154 }
```

Nulls mean *unknown* (the restaurant has never pushed a movement), not "nothing outstanding".

### Create a stock row

```bash
curl -X POST "$BASE_URL/stock" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e",
    "amount": 100
  }'
```

### Adjust on-hand by a signed change

```bash
curl -X PUT "$BASE_URL/stock/0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e/add-or-decrease" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "change": -15 }'
```

### Record a batch count

The counted amount is converted to a change against the cloud's current on-hand — a count of
`20` against `30` records `-10`, so a sale replicated afterwards still takes effect on top.

```bash
curl -X POST "$BASE_URL/stock/bulk" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "items": [
      { "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e", "amount": 20 },
      { "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6f", "amount": 8 }
    ]
  }'
```

### Delete a stock row

```bash
curl -X DELETE "$BASE_URL/stock/0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e" \
  -H "Authorization: Bearer $TOKEN"
```

### Confirm the writes are absent on an edge node (expects 404)

```bash
export EDGE_URL="http://localhost:8082/api"

curl -s -o /dev/null -w "%{http_code}\n" -X POST "$EDGE_URL/stock" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "product_id": "0192f3a1-4c5e-7b2a-9d8e-1f2a3b4c5d6e", "amount": 1 }'
```

### Write opening balances (cutover only, admin key)

Makes every product's amount equal the sum of its movements. Safe to re-run — the second
call reports `"written_balances": 0`. See `docs/playbooks/STOCK_CUTOVER_RUNBOOK.md`.

```bash
curl -X POST "$BASE_URL/stock/opening-balances" \
  -H "X-API-Key: $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{ "organization": "laguna-escondida" }'
```

---

## Edge Update (edge node, admin)

### Get update state

```bash
export EDGE_URL="http://localhost:8082/api"

curl "$EDGE_URL/edge/update" \
  -H "Authorization: Bearer $TOKEN"
```

### Request the staged update

```bash
curl -X POST "$EDGE_URL/edge/update" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{ "version": "v1.0.1" }'
```
