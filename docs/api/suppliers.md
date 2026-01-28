# Suppliers API

Manage supplier/vendor information.

## Endpoints

| Method | Endpoint             | Description           |
| ------ | -------------------- | --------------------- |
| POST   | `/api/suppliers`     | Create a new supplier |
| GET    | `/api/suppliers`     | List all suppliers    |
| GET    | `/api/suppliers/:id` | Get supplier by ID    |
| PUT    | `/api/suppliers/:id` | Update a supplier     |
| DELETE | `/api/suppliers/:id` | Delete a supplier     |

---

## Create Supplier

Creates a new supplier.

```
POST /api/suppliers
```

### Request Body

| Field                 | Type   | Required | Description                                           |
| --------------------- | ------ | -------- | ----------------------------------------------------- |
| name                  | string | Yes      | Supplier name (1-255 chars)                           |
| identification_type   | string | No       | Identification type (e.g., NIT, CC, CE) (max 50 chars)|
| identification_number | string | No       | Identification number (max 50 chars)                  |
| contact_name          | string | No       | Contact person name (max 255 chars)                   |
| phone                 | string | No       | Phone number (max 50 chars)                           |
| email                 | string | No       | Email address (valid email format)                    |
| notes                 | string | No       | Additional notes (max 1000 chars)                     |

### Example Request

```json
{
  "name": "Fresh Produce Co.",
  "identification_type": "NIT",
  "identification_number": "900123456-1",
  "contact_name": "Juan Garcia",
  "phone": "+57 300 123 4567",
  "email": "juan@freshproduce.com",
  "notes": "Delivers on Mondays and Thursdays"
}
```

### Example Response (201 Created)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Fresh Produce Co.",
  "identification_type": "NIT",
  "identification_number": "900123456-1",
  "contact_name": "Juan Garcia",
  "phone": "+57 300 123 4567",
  "email": "juan@freshproduce.com",
  "notes": "Delivers on Mondays and Thursdays",
  "created_at": "2024-01-26T15:30:00Z",
  "updated_at": "2024-01-26T15:30:00Z"
}
```

---

## List Suppliers

Returns all suppliers.

```
GET /api/suppliers
```

### Example Response (200 OK)

```json
{
  "suppliers": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Fresh Produce Co.",
      "identification_type": "NIT",
      "identification_number": "900123456-1",
      "contact_name": "Juan Garcia",
      "phone": "+57 300 123 4567",
      "email": "juan@freshproduce.com",
      "notes": "Delivers on Mondays and Thursdays",
      "created_at": "2024-01-26T15:30:00Z",
      "updated_at": "2024-01-26T15:30:00Z"
    }
  ],
  "total": 1
}
```

---

## Get Supplier by ID

Returns a single supplier by ID.

```
GET /api/suppliers/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Supplier ID |

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Fresh Produce Co.",
  "identification_type": "NIT",
  "identification_number": "900123456-1",
  "contact_name": "Juan Garcia",
  "phone": "+57 300 123 4567",
  "email": "juan@freshproduce.com",
  "notes": "Delivers on Mondays and Thursdays",
  "created_at": "2024-01-26T15:30:00Z",
  "updated_at": "2024-01-26T15:30:00Z"
}
```

### Error Response (404 Not Found)

```json
{
  "error": "Supplier not found"
}
```

---

## Update Supplier

Updates an existing supplier.

```
PUT /api/suppliers/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Supplier ID |

### Request Body

| Field                 | Type   | Required | Description                                           |
| --------------------- | ------ | -------- | ----------------------------------------------------- |
| name                  | string | Yes      | Supplier name (1-255 chars)                           |
| identification_type   | string | No       | Identification type (e.g., NIT, CC, CE) (max 50 chars)|
| identification_number | string | No       | Identification number (max 50 chars)                  |
| contact_name          | string | No       | Contact person name (max 255 chars)                   |
| phone                 | string | No       | Phone number (max 50 chars)                           |
| email                 | string | No       | Email address (valid email format)                    |
| notes                 | string | No       | Additional notes (max 1000 chars)                     |

### Example Request

```json
{
  "name": "Fresh Produce Co. Updated",
  "identification_type": "NIT",
  "identification_number": "900123456-1",
  "contact_name": "Maria Garcia",
  "phone": "+57 300 987 6543",
  "email": "maria@freshproduce.com",
  "notes": "Now delivers on Tuesdays and Fridays"
}
```

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Fresh Produce Co. Updated",
  "identification_type": "NIT",
  "identification_number": "900123456-1",
  "contact_name": "Maria Garcia",
  "phone": "+57 300 987 6543",
  "email": "maria@freshproduce.com",
  "notes": "Now delivers on Tuesdays and Fridays",
  "created_at": "2024-01-26T15:30:00Z",
  "updated_at": "2024-01-26T16:00:00Z"
}
```

---

## Delete Supplier

Soft deletes a supplier.

```
DELETE /api/suppliers/:id
```

### Path Parameters

| Parameter | Type | Description |
| --------- | ---- | ----------- |
| id        | UUID | Supplier ID |

### Response (204 No Content)

No response body on success.

### Error Response (404 Not Found)

```json
{
  "error": "Supplier not found"
}
```
