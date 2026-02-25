# Purchase Entries API

Record goods received from suppliers. Also known as "Ingress" or inventory intake.

## Endpoints

| Method | Endpoint                                 | Description                       |
| ------ | ---------------------------------------- | --------------------------------- |
| POST   | `/api/purchase-entries`                  | Create a new purchase entry       |
| GET    | `/api/purchase-entries`                  | List all purchase entries         |
| GET    | `/api/purchase-entries/export`           | Export purchase entries as CSV    |
| GET    | `/api/purchase-entries/:id`              | Get purchase entry by ID          |
| GET    | `/api/suppliers/:id/purchase-entries`    | List purchase entries by supplier |
| POST   | `/api/purchase-entries/:id/documents`    | Upload supporting document        |

---

## Create Purchase Entry

Records a new receipt of goods from a supplier. Automatically updates the supplier catalog with the latest unit cost for each product.

```
POST /api/purchase-entries
```

### Request Body

| Field             | Type      | Required | Description                                |
| ----------------- | --------- | -------- | ------------------------------------------ |
| supplier_id       | UUID      | Yes      | Supplier ID                                |
| invoice_reference | string    | No       | Supplier invoice number (max 255 chars)    |
| entry_date        | timestamp | No       | Date goods were received (defaults to now) |
| notes             | string    | No       | Additional notes (max 1000 chars)          |
| items             | array     | Yes      | List of products received (min 1 item)     |

### Item Object

| Field      | Type   | Required | Description                              |
| ---------- | ------ | -------- | ---------------------------------------- |
| product_id | UUID   | Yes      | Product ID                               |
| quantity   | string | Yes      | Quantity received (decimal, must be > 0) |
| unit_cost  | string | Yes      | Cost per unit (decimal, must be >= 0)    |

### Example Request

```json
{
  "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
  "invoice_reference": "INV-2024-001",
  "entry_date": "2024-01-26T10:00:00Z",
  "notes": "Weekly produce delivery",
  "items": [
    {
      "product_id": "660e8400-e29b-41d4-a716-446655440001",
      "quantity": "10.5",
      "unit_cost": "2500.00"
    },
    {
      "product_id": "770e8400-e29b-41d4-a716-446655440002",
      "quantity": "20",
      "unit_cost": "1800.00"
    }
  ]
}
```

### Example Response (201 Created)

```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
  "total_amount": "62250.00",
  "invoice_reference": "INV-2024-001",
  "entry_date": "2024-01-26T10:00:00Z",
  "notes": "Weekly produce delivery",
  "items": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440004",
      "purchase_entry_id": "880e8400-e29b-41d4-a716-446655440003",
      "product_id": "660e8400-e29b-41d4-a716-446655440001",
      "quantity": "10.5",
      "unit_cost": "2500.00",
      "total_cost": "26250.00"
    },
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440005",
      "purchase_entry_id": "880e8400-e29b-41d4-a716-446655440003",
      "product_id": "770e8400-e29b-41d4-a716-446655440002",
      "quantity": "20",
      "unit_cost": "1800.00",
      "total_cost": "36000.00"
    }
  ],
  "created_at": "2024-01-26T15:30:00Z"
}
```

### Calculation Notes

- `total_cost` per item = `quantity` × `unit_cost`
- `total_amount` = sum of all item `total_cost` values

### Side Effects

When creating a purchase entry:

1. If the product is not in the supplier's catalog, it's automatically added
2. If the product exists in the catalog, the `unit_cost` is updated to the new value

---

## List Purchase Entries

Returns all purchase entries, ordered by entry date (newest first).

```
GET /api/purchase-entries
```

### Example Response (200 OK)

```json
{
  "entries": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
      "supplier_name": "Fresh Produce Co.",
      "total_amount": "62250.00",
      "invoice_reference": "INV-2024-001",
      "entry_date": "2024-01-26T10:00:00Z",
      "notes": "Weekly produce delivery",
      "created_at": "2024-01-26T15:30:00Z"
    }
  ],
  "total": 1
}
```

Note: The list response does not include items. Use the detail endpoint to get items.

---

## Get Purchase Entry by ID

Returns a single purchase entry with all its items.

```
GET /api/purchase-entries/:id
```

### Path Parameters

| Parameter | Type | Description       |
| --------- | ---- | ----------------- |
| id        | UUID | Purchase entry ID |

### Example Response (200 OK)

```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
  "supplier_name": "Fresh Produce Co.",
  "total_amount": "62250.00",
  "invoice_reference": "INV-2024-001",
  "entry_date": "2024-01-26T10:00:00Z",
  "notes": "Weekly produce delivery",
  "pdf_storage_path": null,
  "xml_storage_path": null,
  "items": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440004",
      "purchase_entry_id": "880e8400-e29b-41d4-a716-446655440003",
      "product_id": "660e8400-e29b-41d4-a716-446655440001",
      "product_name": "Tomato",
      "quantity": "10.5",
      "unit_cost": "2500.00",
      "total_cost": "26250.00"
    },
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440005",
      "purchase_entry_id": "880e8400-e29b-41d4-a716-446655440003",
      "product_id": "770e8400-e29b-41d4-a716-446655440002",
      "product_name": "Potato",
      "quantity": "20",
      "unit_cost": "1800.00",
      "total_cost": "36000.00"
    }
  ],
  "created_at": "2024-01-26T15:30:00Z"
}
```

---

## List Purchase Entries by Supplier

Returns all purchase entries for a specific supplier.

```
GET /api/suppliers/:id/purchase-entries
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Supplier ID |

### Example Response (200 OK)

```json
{
  "entries": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
      "supplier_name": "Fresh Produce Co.",
      "total_amount": "62250.00",
      "invoice_reference": "INV-2024-001",
      "entry_date": "2024-01-26T10:00:00Z",
      "notes": "Weekly produce delivery",
      "created_at": "2024-01-26T15:30:00Z"
    },
    {
      "id": "bb0e8400-e29b-41d4-a716-446655440006",
      "supplier_id": "550e8400-e29b-41d4-a716-446655440000",
      "supplier_name": "Fresh Produce Co.",
      "total_amount": "45000.00",
      "invoice_reference": "INV-2024-002",
      "entry_date": "2024-01-19T10:00:00Z",
      "notes": "Previous week delivery",
      "created_at": "2024-01-19T15:30:00Z"
    }
  ],
  "total": 2
}
```

### Error Response (404 Not Found)

```json
{
  "error": "Supplier not found"
}
```

---

## Upload Purchase Entry Document

Uploads supporting documents (PDF, XML, or ZIP containing both) for a purchase entry. This is useful for storing supplier electronic invoices and receipts for accountability.

```
POST /api/purchase-entries/:id/documents
```

### Path Parameters

| Parameter | Type | Description       |
| --------- | ---- | ----------------- |
| id        | UUID | Purchase entry ID |

### Query Parameters

| Parameter | Type   | Required | Description                                              |
| --------- | ------ | -------- | -------------------------------------------------------- |
| file_type | string | Conditional | Required for single PDF/XML uploads. Not needed for ZIP |

### Request Body

Multipart form data with:

| Field | Type | Required | Description                                          |
| ----- | ---- | -------- | ---------------------------------------------------- |
| file  | file | Yes      | PDF, XML, or ZIP file containing both PDF and XML    |

### ZIP File Support

When uploading a ZIP file, the system will:

1. Automatically detect the ZIP format (no `file_type` query parameter needed)
2. Extract the contents and validate exactly one PDF and one XML file exist
3. Upload both files to storage
4. Update the database with both storage paths

**ZIP Requirements:**
- Must contain exactly 1 PDF file
- Must contain exactly 1 XML file
- Files can be at any level within the ZIP (nested folders are supported)

### Example Request (Single File)

```
POST /api/purchase-entries/880e8400-e29b-41d4-a716-446655440003/documents?file_type=pdf
Content-Type: multipart/form-data

file: <binary PDF data>
```

### Example Response (Single File - 200 OK)

```json
{
  "pdf_storage_path": "org123/purchase-entries/880e8400-e29b-41d4-a716-446655440003.pdf"
}
```

### Example Request (ZIP File)

```
POST /api/purchase-entries/880e8400-e29b-41d4-a716-446655440003/documents
Content-Type: multipart/form-data

file: <binary ZIP data containing invoice.pdf and invoice.xml>
```

### Example Response (ZIP File - 200 OK)

```json
{
  "pdf_storage_path": "org123/purchase-entries/880e8400-e29b-41d4-a716-446655440003.pdf",
  "xml_storage_path": "org123/purchase-entries/880e8400-e29b-41d4-a716-446655440003.xml"
}
```

### Storage Path Format

Documents are stored at:

```
{organization_id}/purchase-entries/{purchase_entry_id}.{extension}
```

### Error Responses

**400 Bad Request** - Missing file

```json
{
  "error": "File is required"
}
```

**400 Bad Request** - Invalid file type (for single file uploads)

```json
{
  "error": "File type must be 'pdf' or 'xml' for single file uploads"
}
```

**400 Bad Request** - ZIP missing PDF

```json
{
  "error": "ZIP file must contain exactly one PDF file"
}
```

**400 Bad Request** - ZIP missing XML

```json
{
  "error": "ZIP file must contain exactly one XML file"
}
```

**400 Bad Request** - ZIP contains multiple PDFs

```json
{
  "error": "ZIP file contains multiple PDF files, expected exactly one"
}
```

**400 Bad Request** - ZIP contains multiple XMLs

```json
{
  "error": "ZIP file contains multiple XML files, expected exactly one"
}
```

**400 Bad Request** - Invalid/corrupt ZIP

```json
{
  "error": "invalid or corrupt ZIP file"
}
```

**404 Not Found** - Purchase entry not found

```json
{
  "error": "Purchase entry not found"
}
```

---

## Export Purchase Entries CSV

`GET /api/purchase-entries/export`

Downloads all purchase entries as a CSV file. Supports date and supplier filters. Intended for accountants.

**Required permission:** `purchase-entries:export`

### Query Parameters

| Parameter     | Type     | Required | Description                              |
| ------------- | -------- | -------- | ---------------------------------------- |
| `start_date`  | ISO 8601 | No       | Filter entries from this date            |
| `end_date`    | ISO 8601 | No       | Filter entries up to this date           |
| `supplier_id` | UUID     | No       | Filter by supplier                       |

### CSV Columns

| Column                  | Description                            |
| ----------------------- | -------------------------------------- |
| `Fecha`                 | Entry date (`YYYY-MM-DD`)                      |
| `ID`                    | Purchase entry UUID                            |
| `Proveedor`             | Supplier name                                  |
| `Referencia de Factura` | Invoice reference (empty if none)              |
| `Monto Total`           | Total amount                                   |
| `Notas`                 | Notes (empty if none)                          |
| `URL PDF`               | Presigned download URL for PDF (empty if none) |
| `URL XML`               | Presigned download URL for XML (empty if none) |

### Example Response (200 OK)

Returns a `text/csv` file with `Content-Disposition: attachment; filename=entradas_compra_2024-01-26.csv`.

### Error Responses

**500 Internal Server Error**
```json
{
  "error": "Failed to export purchase entries"
}
```
