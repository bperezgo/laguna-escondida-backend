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

| Code | Description                              |
| ---- | ---------------------------------------- |
| 200  | Success                                  |
| 201  | Created                                  |
| 204  | No Content (successful deletion)         |
| 400  | Bad Request - Invalid input              |
| 401  | Unauthorized - Missing or invalid token  |
| 403  | Forbidden - Insufficient permissions     |
| 404  | Not Found                                |
| 409  | Conflict - Resource already exists       |
| 500  | Internal Server Error                    |

## Error Response Format

```json
{
  "error": "Description of the error"
}
```

## API Sections

- [Authentication](api/auth.md) - User authentication, permissions, and current user info
- [Users](api/users.md) - Create users (admin only)
- [Suppliers](api/suppliers.md) - Manage supplier/vendor information
- [Supplier Catalog](api/supplier-catalog.md) - Link products to suppliers with pricing
- [Purchase Entries](api/purchase-entries.md) - Record goods received from suppliers
- [Products](api/products.md) - Manage products and ingredients
- [Expenses](api/expenses.md) - Track non-product expenses (rent, services, investments, etc.)

## Playbooks & Engineering Docs

Deeper engineering context lives in [`playbooks/`](playbooks/). For the offline-first
sync engine (cloud ↔ edge replication), start here to reload context:

- [Sync Acceptance Spec](playbooks/SYNC_ACCEPTANCE_SPEC.md) — **read first.** What the sync
  engine must guarantee and why, as a catalog of invariants traceable to the tests and the
  manual playbook. This is the document to read after time away to remember what needs testing.
- [Local Sync Testing](playbooks/SYNC_LOCAL_TESTING.md) — manual two-node (cloud + edge) rig
  and the step-by-step checklist to verify sync by hand.
- [Architecture](playbooks/ARCHITECTURE.md) — system architecture overview.

## Data Types

| Type      | Description                   | Example                                  |
| --------- | ----------------------------- | ---------------------------------------- |
| UUID      | Universally unique identifier | `"550e8400-e29b-41d4-a716-446655440000"` |
| Decimal   | Numeric value as string       | `"10.50"`                                |
| Timestamp | ISO 8601 format               | `"2024-01-26T15:30:00Z"`                 |
