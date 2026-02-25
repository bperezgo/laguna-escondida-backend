# Invoices API

Electronic invoices issued to customers (facturas electrónicas). Invoices are created from a bill (order) and submitted to the electronic invoicing provider. The provider returns a `CUFE` (Código Único de Factura Electrónica) and a `Tascode` that can be used to retrieve the PDF and XML documents.

## Endpoints

| Method | Endpoint               | Description                          |
| ------ | ---------------------- | ------------------------------------ |
| POST   | `/api/invoices`        | Create an electronic invoice         |
| GET    | `/api/invoices`        | List invoices (paginated, filtered)  |
| GET    | `/api/invoices/export` | Export invoices as CSV               |

---

## Create Electronic Invoice

`POST /api/invoices`

Submits an electronic invoice to the invoicing provider and stores the result. The products referenced in `items` must already exist in the system — their prices, VAT, and ICO values are pulled automatically.

**Required permission:** `invoices:create`

### Request Body

| Field          | Type     | Required | Description                                         |
| -------------- | -------- | -------- | --------------------------------------------------- |
| `payment_code` | string   | Yes      | Payment method (see enum values below)              |
| `customer`     | object   | No       | Customer information. Omit for anonymous sales      |
| `items`        | array    | Yes      | Products included in the invoice (min 1)            |

#### `payment_code` enum values

| Value                      | Description                    |
| -------------------------- | ------------------------------ |
| `credit_card`              | Credit card                    |
| `debit_card`               | Debit card                     |
| `cash`                     | Cash                           |
| `transfer_debit_bank`      | Debit bank transfer            |
| `transfer_credit_bank`     | Credit bank transfer           |
| `transfer_debit_interbank` | Interbank debit transfer       |

#### `customer` object

| Field           | Type   | Required | Description                          |
| --------------- | ------ | -------- | ------------------------------------ |
| `id`            | string | Yes      | Customer document number             |
| `document_type` | string | Yes      | `CC` (national ID) or `NIT`          |
| `name`          | string | Yes      | Customer full name                   |
| `email`         | string | Yes      | Customer email address               |

#### `items` array items

| Field       | Type    | Required | Description                                             |
| ----------- | ------- | -------- | ------------------------------------------------------- |
| `product_id`| UUID    | Yes      | Product UUID (must exist in the system)                 |
| `quantity`  | integer | Yes      | Number of units                                         |
| `allowance` | array   | No       | Optional discounts/surcharges applied to the line item  |

#### `allowance` array items

| Field         | Type   | Required | Description                              |
| ------------- | ------ | -------- | ---------------------------------------- |
| `charge`      | string | Yes      | `"true"` for surcharge, `"false"` for discount |
| `reasonCode`  | string | Yes      | Code identifying the reason              |
| `description` | string | Yes      | Human-readable reason                    |
| `baseAmount`  | string | Yes      | Amount the allowance is calculated on    |
| `amount`      | string | Yes      | Allowance amount                         |

### Example Request (anonymous sale, cash)

```json
{
  "payment_code": "cash",
  "items": [
    {
      "product_id": "550e8400-e29b-41d4-a716-446655440000",
      "quantity": 2
    },
    {
      "product_id": "661f9511-f3ac-52e5-b827-557766551111",
      "quantity": 1
    }
  ]
}
```

### Example Request (identified customer, card, with discount)

```json
{
  "payment_code": "credit_card",
  "customer": {
    "id": "1234567890",
    "document_type": "CC",
    "name": "Juan García",
    "email": "juan.garcia@email.com"
  },
  "items": [
    {
      "product_id": "550e8400-e29b-41d4-a716-446655440000",
      "quantity": 3,
      "allowance": [
        {
          "charge": "false",
          "reasonCode": "95",
          "description": "Descuento especial",
          "baseAmount": "45000.00",
          "amount": "4500.00"
        }
      ]
    }
  ]
}
```

### Example Response (201 Created)

```json
{
  "message": "Electronic invoice created successfully"
}
```

### Error Responses

**400 Bad Request** - Invalid request body
```json
{
  "error": "Invalid request body"
}
```

**404 Not Found** - One or more products not found
```json
{
  "error": "product not found"
}
```

**500 Internal Server Error**
```json
{
  "error": "Failed to create electronic invoice"
}
```

---

## List Invoices

`GET /api/invoices`

Returns a paginated list of invoices. Supports filtering by date range and customer identification number.

**Required permission:** `invoices:read`

### Query Parameters

| Parameter                | Type     | Required | Default | Description                                        |
| ------------------------ | -------- | -------- | ------- | -------------------------------------------------- |
| `page`                   | integer  | No       | `1`     | Page number (1-based)                              |
| `page_size`              | integer  | No       | `20`    | Results per page (max 100)                         |
| `created_at_start`       | ISO 8601 | No       | —       | Filter invoices created from this datetime         |
| `created_at_end`         | ISO 8601 | No       | —       | Filter invoices created up to this datetime        |
| `national_identification`| string   | No       | —       | Filter by customer document number                 |

### Example Request

```
GET /api/invoices?page=1&page_size=20&created_at_start=2024-01-01T00:00:00Z&created_at_end=2024-01-31T23:59:59Z
```

### Example Response (200 OK)

```json
{
  "invoices": [
    {
      "id": "7a1b2c3d-4e5f-6789-abcd-ef0123456789",
      "total_amount": "47500.00",
      "discount_amount": "0.00",
      "vat": "7600.00",
      "ico": "0.00",
      "tip": "0.00",
      "document_url": "https://provider.com/docs/abc123",
      "cufe": "abc123def456abc123def456abc123def456abc123def456abc123def456abc1",
      "tascode": "TAS-2024-00042",
      "customer_id": "1234567890",
      "pdf_download_url": "https://storage.example.com/org/sales_invoices/7a1b2c3d-4e5f-6789-abcd-ef0123456789.pdf?X-Signature=...",
      "xml_download_url": "https://storage.example.com/org/sales_invoices/7a1b2c3d-4e5f-6789-abcd-ef0123456789.xml?X-Signature=...",
      "created_at": "2024-01-15T10:30:00Z"
    }
  ],
  "total_count": 42,
  "page": 1,
  "page_size": 20,
  "total_pages": 3
}
```

#### Response fields

| Field            | Type             | Description                                                          |
| ---------------- | ---------------- | -------------------------------------------------------------------- |
| `invoices`       | array            | List of invoice items for the current page                           |
| `total_count`    | integer          | Total number of invoices matching the filter                         |
| `page`           | integer          | Current page number                                                  |
| `page_size`      | integer          | Number of results per page                                           |
| `total_pages`    | integer          | Total number of pages                                                |

#### Invoice item fields

| Field              | Type    | Description                                                              |
| ------------------ | ------- | ------------------------------------------------------------------------ |
| `id`               | UUID    | Invoice UUID                                                             |
| `total_amount`     | decimal | Total invoice amount (before discounts, including taxes)                 |
| `discount_amount`  | decimal | Total discount applied                                                   |
| `vat`              | decimal | VAT tax amount                                                           |
| `ico`              | decimal | ICO tax amount (impuesto al consumo)                                     |
| `tip`              | decimal | Tip amount                                                               |
| `document_url`     | string  | URL to the invoice document from the provider (may be `null`)            |
| `cufe`             | string  | Código Único de Factura Electrónica — unique invoice code from DIAN      |
| `tascode`          | string  | Internal reference code from the invoicing provider                      |
| `customer_id`      | string  | Customer document number (`null` for anonymous sales)                    |
| `pdf_download_url` | string  | Presigned URL to download the PDF (1-hour expiry, `null` if not stored)  |
| `xml_download_url` | string  | Presigned URL to download the XML (1-hour expiry, `null` if not stored)  |
| `created_at`       | datetime| Invoice creation timestamp                                               |

### Error Responses

**500 Internal Server Error**
```json
{
  "error": "Failed to list invoices"
}
```

---

## Export Invoices CSV

`GET /api/invoices/export`

Downloads all invoices matching the filters as a CSV file. Intended for accountants. The response is a file attachment — no pagination, returns all matching records.

**Required permission:** `invoices:export`

### Query Parameters

| Parameter                | Type     | Required | Description                                        |
| ------------------------ | -------- | -------- | -------------------------------------------------- |
| `created_at_start`       | ISO 8601 | No       | Filter invoices created from this datetime         |
| `created_at_end`         | ISO 8601 | No       | Filter invoices created up to this datetime        |
| `national_identification`| string   | No       | Filter by customer document number                 |

### Example Request

```
GET /api/invoices/export?created_at_start=2024-01-01T00:00:00Z&created_at_end=2024-01-31T23:59:59Z
```

### CSV Columns

| Column              | Description                                                    |
| ------------------- | -------------------------------------------------------------- |
| `Fecha de Creacion` | Invoice creation datetime (`YYYY-MM-DD HH:MM:SS`)              |
| `CUFE`              | Código Único de Factura Electrónica                            |
| `Tascode`           | Provider reference code                                        |
| `Total`             | Total invoice amount                                           |
| `Descuento`         | Discount amount                                                |
| `VAT`               | VAT tax amount                                                 |
| `ICO`               | ICO tax amount (impuesto al consumo)                           |
| `Propina`           | Tip amount                                                     |
| `URL Documento`     | URL to the invoice document from the provider (empty if none)  |
| `URL PDF`           | Presigned download URL for PDF (empty if none)                 |
| `URL XML`           | Presigned download URL for XML (empty if none)                 |

### Example Response (200 OK)

Returns a `text/csv` file with header:
```
Content-Disposition: attachment; filename=facturas_2024-01-26.csv
```

```csv
Fecha de Creacion,CUFE,Tascode,Total,Descuento,VAT,ICO,Propina,URL Documento,URL PDF,URL XML
2024-01-15 10:30:00,abc123def456...,TAS-2024-00042,47500.00,0.00,7600.00,0.00,0.00,https://provider.com/docs/abc123,https://storage.example.com/...pdf,https://storage.example.com/...xml
2024-01-16 14:22:00,xyz789ghi012...,TAS-2024-00043,15000.00,1500.00,2160.00,0.00,0.00,https://provider.com/docs/xyz789,,
```

> **Note:** `URL PDF` and `URL XML` are presigned URLs with a 1-hour expiry. They will be empty for invoices whose documents have not yet been stored.

### Error Responses

**500 Internal Server Error**
```json
{
  "error": "Failed to export invoices"
}
```
