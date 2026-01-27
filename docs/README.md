# Laguna Escondida API Documentation

## Overview

This documentation covers the REST API for the Laguna Escondida backend system. The API provides endpoints for managing suppliers, products, purchase entries, and more.

## Base URL

```
https://your-domain.com/api
```

## Authentication

All endpoints (except `/api/auth/signin` and `/api/health`) require JWT authentication.

### Getting a Token

```bash
POST /api/auth/signin
Content-Type: application/json

{
  "username": "your_username",
  "password": "your_password"
}
```

### Using the Token

Include the JWT token in the `Authorization` header:

```
Authorization: Bearer <your_jwt_token>
```

## Common Response Codes

| Code | Description                             |
| ---- | --------------------------------------- |
| 200  | Success                                 |
| 201  | Created                                 |
| 204  | No Content (successful deletion)        |
| 400  | Bad Request - Invalid input             |
| 401  | Unauthorized - Missing or invalid token |
| 404  | Not Found                               |
| 409  | Conflict - Resource already exists      |
| 500  | Internal Server Error                   |

## Error Response Format

```json
{
  "error": "Description of the error"
}
```

## API Sections

- [Suppliers](api/suppliers.md) - Manage supplier/vendor information
- [Supplier Catalog](api/supplier-catalog.md) - Link products to suppliers with pricing
- [Purchase Entries](api/purchase-entries.md) - Record goods received from suppliers
- [Products](api/products.md) - Manage products and ingredients

## Data Types

| Type      | Description                   | Example                                  |
| --------- | ----------------------------- | ---------------------------------------- |
| UUID      | Universally unique identifier | `"550e8400-e29b-41d4-a716-446655440000"` |
| Decimal   | Numeric value as string       | `"10.50"`                                |
| Timestamp | ISO 8601 format               | `"2024-01-26T15:30:00Z"`                 |
