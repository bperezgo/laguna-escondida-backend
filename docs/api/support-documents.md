# Support Documents (Documentos Soporte)

Support documents are Colombian tax documents for purchases from informal suppliers. They use the same electronic invoicing system but with a different prefix and require a mandatory provider (supplier) instead of an optional customer.

## Endpoints

| Method | Endpoint                                              | Description                          |
| ------ | ----------------------------------------------------- | ------------------------------------ |
| POST   | `/api/support-documents`                              | Create a support document            |
| GET    | `/api/support-documents`                              | List support documents               |
| GET    | `/api/support-documents/export`                       | Export support documents to CSV      |
| POST   | `/api/support-documents/update-missing-document-urls` | Update missing document URLs (admin) |

---

## Create Support Document

`POST /api/support-documents`

Creates a new support document and submits it to the electronic invoicing system.

### Request Body

| Field                  | Type   | Required | Description                            |
| ---------------------- | ------ | -------- | -------------------------------------- |
| payment_code           | string | Yes      | Payment method code (see values below) |
| provider               | object | Yes      | Provider (supplier) information        |
| provider.id            | string | Yes      | Provider document number (NIT or CC)   |
| provider.document_type | string | Yes      | Document type: `"CC"` or `"NIT"`       |
| provider.name          | string | Yes      | Provider name                          |
| provider.email         | string | Yes      | Provider email                         |
| items                  | array  | Yes      | List of items                          |
| items[].product_id     | string | Yes      | Product UUID                           |
| items[].quantity       | int    | Yes      | Quantity                               |

**Payment Code Values:**

- `credit_card`
- `debit_card`
- `cash`
- `transfer_debit_bank`
- `transfer_credit_bank`
- `transfer_debit_interbank`

### Example Request

```json
{
  "payment_code": "cash",
  "provider": {
    "id": "900123456",
    "document_type": "NIT",
    "name": "Proveedor Ejemplo S.A.S",
    "email": "proveedor@example.com"
  },
  "items": [
    {
      "product_id": "550e8400-e29b-41d4-a716-446655440000",
      "quantity": 5
    }
  ]
}
```

### Example Response (201 Created)

```json
{
  "message": "Support document created successfully"
}
```

### Error Responses

**400 Bad Request** - Missing provider or invalid body

```json
{
  "error": "Provider document number and name are required"
}
```

**500 Internal Server Error**

```json
{
  "error": "Failed to create support document"
}
```

---

## List Support Documents

`GET /api/support-documents`

Returns a paginated list of support documents.

### Query Parameters

| Parameter                | Type   | Required | Description                              |
| ------------------------ | ------ | -------- | ---------------------------------------- |
| page                     | int    | No       | Page number (default: 1)                 |
| page_size                | int    | No       | Items per page (default: 20, max: 100)   |
| created_at_start         | string | No       | Start date (YYYY-MM-DD or RFC3339)       |
| created_at_end           | string | No       | End date (YYYY-MM-DD or RFC3339)         |
| provider_document_number | string | No       | Filter by provider document number (NIT) |

### Example Response (200 OK)

```json
{
  "support_documents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "total_amount": "50000.0000",
      "discount_amount": "0.0000",
      "vat": "9500.0000",
      "ico": "0.0000",
      "tip": "0.0000",
      "cufe": "abc123...",
      "tascode": "TAS-12345",
      "provider_document_number": "900123456",
      "provider_name": "Proveedor Ejemplo S.A.S",
      "pdf_download_url": "https://...",
      "xml_download_url": "https://...",
      "created_at": "2026-04-16T10:30:00Z"
    }
  ],
  "total_count": 1,
  "page": 1,
  "page_size": 20,
  "total_pages": 1
}
```

---

## Export Support Documents CSV

`GET /api/support-documents/export`

Exports support documents as a CSV file.

### Query Parameters

| Parameter                | Type   | Required | Description                              |
| ------------------------ | ------ | -------- | ---------------------------------------- |
| created_at_start         | string | No       | Start date (YYYY-MM-DD or RFC3339)       |
| created_at_end           | string | No       | End date (YYYY-MM-DD or RFC3339)         |
| provider_document_number | string | No       | Filter by provider document number (NIT) |

### Response

Returns a CSV file with headers:
`Fecha de Creacion, CUFE, Tascode, Proveedor NIT, Proveedor Nombre, Total, Descuento, VAT, ICO, Propina, URL Documento, URL PDF, URL XML`

---

## Update Missing Document URLs (Admin)

`POST /api/support-documents/update-missing-document-urls`

**Authentication:** Admin API Key (header: `X-Admin-API-Key`)

Fetches PDF/XML documents from the electronic invoicing system for support documents that are pending.

### Example Response (200 OK)

```json
{
  "updated_count": 3,
  "failed_bills": [
    {
      "bill_id": "550e8400-e29b-41d4-a716-446655440001",
      "error": "PDF URL is empty"
    }
  ]
}
```
