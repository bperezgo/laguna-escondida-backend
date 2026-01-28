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
      "name": "waitress",
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

Returns the currently authenticated user's information including roles and permissions. Use this endpoint to validate the user's session and get fresh permissions data.

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
      "name": "waitress",
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
    "products:read",
    "products:create",
    "expenses:read",
    "expenses:create"
  ]
}
```

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

**500 Internal Server Error** - Failed to retrieve user

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

---

## Role-Permission Mapping

| Role       | Description                                        |
| ---------- | -------------------------------------------------- |
| admin      | Full access to all resources and operations        |
| manager    | Access to most operations except user management   |
| waitress   | Order and product operations, real-time updates    |
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
