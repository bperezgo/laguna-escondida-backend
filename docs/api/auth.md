# Authentication

Handles user authentication and current user information.

## Endpoints

| Method | Endpoint          | Description                        |
| ------ | ----------------- | ---------------------------------- |
| POST   | `/api/auth/signin` | Authenticate user and get token   |
| GET    | `/api/auth/me`     | Get current authenticated user    |

---

## Sign In

`POST /api/auth/signin`

Authenticates a user and returns a JWT token with user information and permissions.

### Request Body

| Field    | Type   | Required | Description    |
| -------- | ------ | -------- | -------------- |
| username | string | Yes      | User's username |
| password | string | Yes      | User's password |

### Example Request

```json
{
  "username": "john_doe",
  "password": "securepassword123"
}
```

### Example Response (200 OK)

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "john_doe",
  "roles": [
    {
      "id": 1,
      "name": "server",
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": 3,
      "name": "manager",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "permissions": [
    "orders:read",
    "orders:create",
    "orders:update",
    "orders:delete",
    "orders:pay",
    "products:read",
    "products:create",
    "expenses:read",
    "expenses:create"
  ]
}
```

### Error Responses

**401 Unauthorized** - Invalid credentials

```json
{
  "error": "Invalid username or password"
}
```

---

## Get Current User

`GET /api/auth/me`

Returns the currently authenticated user's information including roles and permissions, and the organization the user works in. Use this endpoint to validate the user's session and get fresh permissions data, and to brand and format the app for the organization.

- `organization` is always present. Its optional fields (`legal_name` … `secondary_color`) are `null` when the organization has not set them; fall back to the current defaults.
- `organization` holds settings only: no status, e-invoicing prefixes or timestamps. They are maintained through the [Platform Organizations API](./organizations.md).
- Until the token carries the organization (tenancy step 3), it is the deployment's configured organization. On the edge, the optional fields stay `null` until the sync carries them.

### Headers

| Header        | Value               | Required |
| ------------- | ------------------- | -------- |
| Authorization | Bearer `<jwt_token>` | Yes      |

### Example Response (200 OK)

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "john_doe",
  "name": "John Doe",
  "roles": [
    {
      "id": 1,
      "name": "server",
      "created_at": "2024-01-01T00:00:00Z"
    },
    {
      "id": 3,
      "name": "manager",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "permissions": [
    "orders:read",
    "orders:create",
    "orders:update",
    "orders:delete",
    "orders:pay",
    "products:read",
    "products:create",
    "expenses:read",
    "expenses:create"
  ],
  "organization": {
    "id": "bb0cad3e-be13-4bad-9a37-bba9525bfece",
    "slug": "laguna-escondida",
    "name": "Laguna Escondida, Restaurante y Pesca S.A.S",
    "timezone": "America/Bogota",
    "country": "CO",
    "locale": "es-CO",
    "currency": "COP",
    "legal_name": "Laguna Escondida, Restaurante y Pesca S.A.S",
    "nit": "901234567-8",
    "address": null,
    "phone": null,
    "ticket_footer": "¡Gracias por su visita!",
    "ticket_legal_notice": null,
    "display_name": "Laguna Escondida",
    "logo_url": "https://cdn.lagunaescondida.co/logo.png",
    "primary_color": "#14B8A6",
    "secondary_color": "#C9A227"
  }
}
```

### Organization Fields

| Field                 | Type           | Description                                             |
| --------------------- | -------------- | ------------------------------------------------------- |
| `id`                  | string (UUID)  | Organization id                                         |
| `slug`                | string         | Unique, URL-safe identifier                             |
| `name`                | string         | Legal or commercial name                                |
| `timezone`            | string         | IANA time zone for the business day, e.g. `America/Bogota` |
| `country`             | string         | ISO 3166-1 alpha-2, e.g. `CO`                           |
| `locale`              | string         | Number and date formatting, e.g. `es-CO`                |
| `currency`            | string         | ISO 4217, e.g. `COP`                                    |
| `legal_name`          | string \| null | Business name for receipts                              |
| `nit`                 | string \| null | Tax id for receipts                                     |
| `address`             | string \| null | Address for receipts                                    |
| `phone`               | string \| null | Phone for receipts                                      |
| `ticket_footer`       | string \| null | Receipt footer text                                     |
| `ticket_legal_notice` | string \| null | Receipt legal notice                                    |
| `display_name`        | string \| null | Name to show in the app; fall back to `name`            |
| `logo_url`            | string \| null | `https` logo URL                                        |
| `primary_color`       | string \| null | `#RRGGBB`                                               |
| `secondary_color`     | string \| null | `#RRGGBB`                                               |

### Error Responses

**401 Unauthorized** - Missing or invalid token

```json
{
  "error": "Authorization header is required"
}
```

```json
{
  "error": "Invalid or expired token"
}
```

**500 Internal Server Error** - Failed to retrieve the user or their organization

```json
{
  "error": "Failed to get user information"
}
```

---

## JWT Token Structure

The JWT token contains the following claims:

| Claim       | Type     | Description                              |
| ----------- | -------- | ---------------------------------------- |
| user_id     | string   | User's unique identifier (UUID)          |
| username    | string   | User's username                          |
| role_ids    | []int    | Array of role IDs assigned to the user   |
| permissions | []string | Array of permission strings              |
| exp         | number   | Token expiration timestamp (Unix epoch)  |
| iat         | number   | Token issued at timestamp (Unix epoch)   |

### Token Expiration

Tokens expire after **24 hours**. After expiration, the user must sign in again to obtain a new token.

---

## Permissions

Permissions follow the `resource:action` format. Available permissions include:

### Orders
- `orders:read` - View orders
- `orders:create` - Create new orders
- `orders:update` - Update existing orders
- `orders:delete` - Delete orders
- `orders:pay` - Pay and close an order (manager/admin only)

### Products
- `products:read` - View products
- `products:create` - Create new products
- `products:update` - Update existing products
- `products:delete` - Delete products

### Expenses
- `expenses:read` - View expenses
- `expenses:create` - Create new expenses
- `expenses:update` - Update existing expenses
- `expenses:delete` - Delete expenses
- `expenses:upload` - Upload expense documents

### Suppliers
- `suppliers:read` - View suppliers
- `suppliers:create` - Create new suppliers
- `suppliers:update` - Update existing suppliers
- `suppliers:delete` - Delete suppliers

### Stock
- `stock:read` - View stock levels
- `stock:create` - Create stock entries
- `stock:update` - Update stock levels
- `stock:delete` - Delete stock entries

### Invoices
- `invoices:read` - View invoices
- `invoices:create` - Create new invoices
- `invoices:export` - Export invoices

### Purchase Entries
- `purchase-entries:read` - View purchase entries
- `purchase-entries:create` - Create new purchase entries
- `purchase-entries:upload` - Upload purchase entry documents

### Edge appliance
- `edge:update` - See and trigger the box's staged appliance update (admin and manager, edge mode)

---

## Role-Permission Mapping

| Role       | Description                                        |
| ---------- | -------------------------------------------------- |
| admin      | Full access to all resources and operations        |
| manager    | Access to most operations except user management   |
| server     | Order and product operations, real-time updates    |
| cooker     | Read-only order/product access, command updates    |
| accountant | Financial operations (expenses, invoices, entries) |

---

## Frontend Integration

### Decoding JWT Token

```typescript
import jwtDecode from 'jwt-decode';

interface JWTPayload {
  user_id: string;
  username: string;
  role_ids: number[];
  permissions: string[];
  exp: number;
}

const token = localStorage.getItem('token');
const decoded = jwtDecode<JWTPayload>(token);
const permissions = decoded.permissions;
```

### Checking Permissions

```typescript
function hasPermission(permission: string): boolean {
  const token = localStorage.getItem('token');
  if (!token) return false;
  
  const decoded = jwtDecode<JWTPayload>(token);
  return decoded.permissions.includes(permission);
}

// Usage
if (hasPermission('expenses:create')) {
  // Show create expense button
}
```

### Permission Gate Component

```tsx
function PermissionGate({ 
  permission, 
  children, 
  fallback = null 
}: { 
  permission: string; 
  children: React.ReactNode; 
  fallback?: React.ReactNode;
}) {
  if (hasPermission(permission)) {
    return <>{children}</>;
  }
  return <>{fallback}</>;
}

// Usage
<PermissionGate permission="expenses:create">
  <CreateExpenseButton />
</PermissionGate>
```
