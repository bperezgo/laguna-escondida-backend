# Expenses API

Track non-product expenses such as indirect costs, investments, rent, and utility services.

## Expense Categories

Expense categories help organize expenses by type for accounting purposes.

### Default Categories

The system comes with these default categories:

| Code          | Name            | Description                                                  |
| ------------- | --------------- | ------------------------------------------------------------ |
| indirect_cost | Indirect Cost   | Indirect costs like cleaning supplies, paper, soap           |
| expense       | General Expense | General operational expenses                                 |
| investment    | Investment      | Capital investments like equipment, materials, improvements  |
| rent          | Rent            | Rental payments for buildings and spaces                     |
| service       | Service         | Utility services like electricity, water, internet           |

---

## Expense Category Endpoints

| Method | Endpoint                    | Description         |
| ------ | --------------------------- | ------------------- |
| POST   | `/api/expense-categories`   | Create category     |
| GET    | `/api/expense-categories`   | List all categories |
| GET    | `/api/expense-categories/:id` | Get category by ID  |
| PUT    | `/api/expense-categories/:id` | Update category     |

---

## Create Expense Category

Creates a new expense category.

```
POST /api/expense-categories
```

### Request Body

| Field       | Type   | Required | Description                      |
| ----------- | ------ | -------- | -------------------------------- |
| code        | string | Yes      | Unique code (1-50 chars)         |
| name        | string | Yes      | Display name (1-255 chars)       |
| description | string | No       | Category description (max 1000)  |

### Example Request

```json
{
  "code": "maintenance",
  "name": "Maintenance",
  "description": "Building and equipment maintenance costs"
}
```

### Example Response (201 Created)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "code": "maintenance",
  "name": "Maintenance",
  "description": "Building and equipment maintenance costs",
  "is_active": true,
  "created_at": "2024-01-26T15:30:00Z"
}
```

### Error Responses

**409 Conflict** - Category code already exists
```json
{
  "error": "Category code already exists"
}
```

---

## List Expense Categories

Returns all expense categories.

```
GET /api/expense-categories
```

### Example Response (200 OK)

```json
{
  "categories": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "code": "indirect_cost",
      "name": "Indirect Cost",
      "description": "Indirect costs like cleaning supplies, paper, soap for restrooms",
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": "660e8400-e29b-41d4-a716-446655440001",
      "code": "rent",
      "name": "Rent",
      "description": "Rental payments for buildings and spaces",
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "total": 2
}
```

---

## Get Expense Category by ID

Returns a single expense category.

```
GET /api/expense-categories/:id
```

### Path Parameters

| Parameter | Type | Description  |
| --------- | ---- | ------------ |
| id        | UUID | Category ID  |

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "code": "rent",
  "name": "Rent",
  "description": "Rental payments for buildings and spaces",
  "is_active": true,
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Error Responses

**404 Not Found**
```json
{
  "error": "Expense category not found"
}
```

---

## Update Expense Category

Updates an existing expense category.

```
PUT /api/expense-categories/:id
```

### Path Parameters

| Parameter | Type | Description  |
| --------- | ---- | ------------ |
| id        | UUID | Category ID  |

### Request Body

| Field       | Type    | Required | Description                      |
| ----------- | ------- | -------- | -------------------------------- |
| code        | string  | Yes      | Unique code (1-50 chars)         |
| name        | string  | Yes      | Display name (1-255 chars)       |
| description | string  | No       | Category description (max 1000)  |
| is_active   | boolean | Yes      | Whether category is active       |

### Example Request

```json
{
  "code": "rent",
  "name": "Building Rent",
  "description": "Monthly rental payments for buildings",
  "is_active": true
}
```

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "code": "rent",
  "name": "Building Rent",
  "description": "Monthly rental payments for buildings",
  "is_active": true,
  "created_at": "2024-01-01T00:00:00Z"
}
```

---

## Expense Endpoints

| Method | Endpoint                      | Description                      |
| ------ | ----------------------------- | -------------------------------- |
| POST   | `/api/expenses`               | Create a new expense             |
| GET    | `/api/expenses`               | List expenses (with filters)     |
| GET    | `/api/expenses/:id`           | Get expense by ID                |
| PUT    | `/api/expenses/:id`           | Update an expense                |
| DELETE | `/api/expenses/:id`           | Delete an expense                |
| POST   | `/api/expenses/:id/documents` | Upload supporting document       |

---

## Create Expense

Records a new expense.

```
POST /api/expenses
```

### Request Body

| Field        | Type      | Required | Description                              |
| ------------ | --------- | -------- | ---------------------------------------- |
| category_id  | UUID      | Yes      | Expense category ID                      |
| supplier_id  | UUID      | No       | Supplier/vendor ID (optional)            |
| amount       | string    | Yes      | Expense amount (decimal, must be > 0)    |
| description  | string    | Yes      | Expense description (1-500 chars)        |
| expense_date | timestamp | No       | Date of expense (defaults to now)        |
| reference    | string    | No       | Invoice/receipt reference (max 255)      |
| notes        | string    | No       | Additional notes (max 1000)              |

### Example Request

```json
{
  "category_id": "550e8400-e29b-41d4-a716-446655440000",
  "supplier_id": "660e8400-e29b-41d4-a716-446655440001",
  "amount": "500000.00",
  "description": "Monthly building rent - January 2024",
  "expense_date": "2024-01-15T00:00:00Z",
  "reference": "RENT-2024-01",
  "notes": "Paid via bank transfer"
}
```

### Example Response (201 Created)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "category_id": "550e8400-e29b-41d4-a716-446655440000",
  "supplier_id": "660e8400-e29b-41d4-a716-446655440001",
  "amount": "500000.00",
  "description": "Monthly building rent - January 2024",
  "expense_date": "2024-01-15T00:00:00Z",
  "reference": "RENT-2024-01",
  "notes": "Paid via bank transfer",
  "created_at": "2024-01-26T15:30:00Z"
}
```

### Error Responses

**404 Not Found** - Category not found
```json
{
  "error": "Expense category not found"
}
```

**404 Not Found** - Supplier not found
```json
{
  "error": "Supplier not found"
}
```

**400 Bad Request** - Invalid amount
```json
{
  "error": "EXPENSE_INVALID_AMOUNT: amount must be a positive number"
}
```

---

## List Expenses

Returns all expenses with optional filtering.

```
GET /api/expenses
```

### Query Parameters

| Parameter    | Type      | Description                          |
| ------------ | --------- | ------------------------------------ |
| category_id  | UUID      | Filter by category                   |
| supplier_id  | UUID      | Filter by supplier                   |
| start_date   | timestamp | Filter expenses after this date      |
| end_date     | timestamp | Filter expenses before this date     |

### Example Request

```
GET /api/expenses?category_id=550e8400-e29b-41d4-a716-446655440000&start_date=2024-01-01T00:00:00Z
```

### Example Response (200 OK)

```json
{
  "expenses": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "category_id": "550e8400-e29b-41d4-a716-446655440000",
      "category_code": "rent",
      "category_name": "Rent",
      "supplier_id": "660e8400-e29b-41d4-a716-446655440001",
      "supplier_name": "Building Owner LLC",
      "amount": "500000.00",
      "description": "Monthly building rent - January 2024",
      "expense_date": "2024-01-15T00:00:00Z",
      "reference": "RENT-2024-01",
      "notes": "Paid via bank transfer",
      "created_at": "2024-01-26T15:30:00Z"
    },
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "category_id": "990e8400-e29b-41d4-a716-446655440004",
      "category_code": "service",
      "category_name": "Service",
      "supplier_id": null,
      "supplier_name": null,
      "amount": "150000.00",
      "description": "Electricity bill - January 2024",
      "expense_date": "2024-01-20T00:00:00Z",
      "reference": "ELEC-2024-01",
      "notes": null,
      "created_at": "2024-01-26T15:35:00Z"
    }
  ],
  "total": 2
}
```

---

## Get Expense by ID

Returns a single expense with category and supplier details.

```
GET /api/expenses/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Expense ID  |

### Example Response (200 OK)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "category_id": "550e8400-e29b-41d4-a716-446655440000",
  "category_code": "rent",
  "category_name": "Rent",
  "supplier_id": "660e8400-e29b-41d4-a716-446655440001",
  "supplier_name": "Building Owner LLC",
  "amount": "500000.00",
  "description": "Monthly building rent - January 2024",
  "expense_date": "2024-01-15T00:00:00Z",
  "reference": "RENT-2024-01",
  "notes": "Paid via bank transfer",
  "pdf_storage_path": null,
  "xml_storage_path": null,
  "created_at": "2024-01-26T15:30:00Z"
}
```

### Error Responses

**404 Not Found**
```json
{
  "error": "Expense not found"
}
```

---

## Update Expense

Updates an existing expense.

```
PUT /api/expenses/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Expense ID  |

### Request Body

| Field        | Type      | Required | Description                              |
| ------------ | --------- | -------- | ---------------------------------------- |
| category_id  | UUID      | Yes      | Expense category ID                      |
| supplier_id  | UUID      | No       | Supplier/vendor ID (optional)            |
| amount       | string    | Yes      | Expense amount (decimal, must be > 0)    |
| description  | string    | Yes      | Expense description (1-500 chars)        |
| expense_date | timestamp | No       | Date of expense                          |
| reference    | string    | No       | Invoice/receipt reference (max 255)      |
| notes        | string    | No       | Additional notes (max 1000)              |

### Example Request

```json
{
  "category_id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": "550000.00",
  "description": "Monthly building rent - January 2024 (adjusted)",
  "expense_date": "2024-01-15T00:00:00Z",
  "reference": "RENT-2024-01-ADJ"
}
```

### Example Response (200 OK)

```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "category_id": "550e8400-e29b-41d4-a716-446655440000",
  "amount": "550000.00",
  "description": "Monthly building rent - January 2024 (adjusted)",
  "expense_date": "2024-01-15T00:00:00Z",
  "reference": "RENT-2024-01-ADJ",
  "created_at": "2024-01-26T15:30:00Z"
}
```

---

## Delete Expense

Deletes an expense record.

```
DELETE /api/expenses/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Expense ID  |

### Response (204 No Content)

No response body on success.

### Error Responses

**404 Not Found**
```json
{
  "error": "Expense not found"
}
```

---

## Upload Expense Document

Uploads supporting documents (PDF, XML, or ZIP containing both) for an expense. This is useful for storing electronic invoices and receipts.

```
POST /api/expenses/:id/documents
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Expense ID  |

### Query Parameters

| Parameter     | Type   | Required    | Description                                              |
| ------------- | ------ | ----------- | -------------------------------------------------------- |
| category_code | string | Yes         | Category code for storage path                           |
| file_type     | string | Conditional | Required for single PDF/XML uploads. Not needed for ZIP  |

### Request Body

Multipart form data with:

| Field | Type | Required | Description                                       |
| ----- | ---- | -------- | ------------------------------------------------- |
| file  | file | Yes      | PDF, XML, or ZIP file containing both PDF and XML |

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
POST /api/expenses/770e8400-e29b-41d4-a716-446655440002/documents?category_code=rent&file_type=pdf
Content-Type: multipart/form-data

file: <binary PDF data>
```

### Example Response (Single File - 200 OK)

```json
{
  "pdf_storage_path": "org123/expenses/rent_770e8400-e29b-41d4-a716-446655440002.pdf"
}
```

### Example Request (ZIP File)

```
POST /api/expenses/770e8400-e29b-41d4-a716-446655440002/documents?category_code=rent
Content-Type: multipart/form-data

file: <binary ZIP data containing invoice.pdf and invoice.xml>
```

### Example Response (ZIP File - 200 OK)

```json
{
  "pdf_storage_path": "org123/expenses/rent_770e8400-e29b-41d4-a716-446655440002.pdf",
  "xml_storage_path": "org123/expenses/rent_770e8400-e29b-41d4-a716-446655440002.xml"
}
```

### Storage Path Format

Documents are stored at:
```
{organization_id}/expenses/{category_code}_{expense_id}.{extension}
```

### Error Responses

**400 Bad Request** - Missing file
```json
{
  "error": "File is required"
}
```

**400 Bad Request** - Missing category code
```json
{
  "error": "Category code is required"
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

**404 Not Found** - Expense not found
```json
{
  "error": "Expense not found"
}
```
