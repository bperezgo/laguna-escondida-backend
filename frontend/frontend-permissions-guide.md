# Frontend Permissions Integration Guide

This document provides all the information needed to implement permission-based access control in the Next.js frontend.

## Overview

The backend implements Permission-Based Access Control (PBAC) where:
- Users have **roles** (waitress, admin, manager, cooker, accountant)
- Roles are mapped to **permissions** (e.g., `orders:read`, `expenses:create`)
- Each API endpoint requires a specific permission
- Permissions are included in the JWT token and sign-in response

---

## Authentication Endpoints

### Sign In

**Endpoint:** `POST /api/auth/signin`

**Request:**
```json
{
  "username": "string",
  "password": "string"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "john_doe",
  "roles": [
    { "id": 1, "name": "waitress", "created_at": "2024-01-01T00:00:00Z" }
  ],
  "permissions": [
    "orders:read",
    "orders:create",
    "orders:update",
    "products:read",
    "commands:read",
    "commands:update",
    "bill-owners:read",
    "sse:commands:read",
    "sse:command-items:read"
  ]
}
```

### Get Current User

**Endpoint:** `GET /api/auth/me`

**Headers:** `Authorization: Bearer <token>`

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "username": "john_doe",
  "name": "John Doe",
  "roles": [
    { "id": 1, "name": "waitress", "created_at": "2024-01-01T00:00:00Z" }
  ],
  "permissions": [
    "orders:read",
    "orders:create",
    "orders:update",
    "products:read",
    "commands:read",
    "commands:update",
    "bill-owners:read",
    "sse:commands:read",
    "sse:command-items:read"
  ]
}
```

---

## JWT Token Structure

The JWT token contains the following payload:

```typescript
interface JWTPayload {
  user_id: string;
  username: string;
  role_ids: number[];
  permissions: string[];
  exp: number;  // Expiration timestamp (Unix epoch)
  iat: number;  // Issued at timestamp (Unix epoch)
}
```

**Token expiration:** 24 hours

---

## All Permissions (Exact Names)

### Orders
| Permission | Description |
|------------|-------------|
| `orders:read` | View orders list and details |
| `orders:create` | Create new orders |
| `orders:update` | Update orders, pay orders, change product status |
| `orders:delete` | Delete orders |

### Products
| Permission | Description |
|------------|-------------|
| `products:read` | View products list and details |
| `products:create` | Create new products |
| `products:update` | Update products, manage product responsibilities |
| `products:delete` | Delete products |

### Stock
| Permission | Description |
|------------|-------------|
| `stock:read` | View stock levels |
| `stock:create` | Create stock entries, bulk stock operations |
| `stock:update` | Add or decrease stock |
| `stock:delete` | Delete stock entries |

### Invoices
| Permission | Description |
|------------|-------------|
| `invoices:read` | View invoices list |
| `invoices:create` | Create electronic invoices |
| `invoices:export` | Export invoices to CSV |

### Commands
| Permission | Description |
|------------|-------------|
| `commands:read` | View pending commands |
| `commands:update` | Complete/update commands |

### Bill Owners
| Permission | Description |
|------------|-------------|
| `bill-owners:read` | View bill owner details |

### Suppliers
| Permission | Description |
|------------|-------------|
| `suppliers:read` | View suppliers list and details |
| `suppliers:create` | Create new suppliers |
| `suppliers:update` | Update supplier information |
| `suppliers:delete` | Delete suppliers |

### Supplier Catalog
| Permission | Description |
|------------|-------------|
| `supplier-catalog:read` | View supplier products and product suppliers |
| `supplier-catalog:create` | Add products to suppliers |
| `supplier-catalog:update` | Update supplier catalog pricing |
| `supplier-catalog:delete` | Remove products from suppliers |

### Purchase Entries
| Permission | Description |
|------------|-------------|
| `purchase-entries:read` | View purchase entries list and details |
| `purchase-entries:create` | Create new purchase entries |
| `purchase-entries:upload` | Upload purchase entry documents |

### Expense Categories
| Permission | Description |
|------------|-------------|
| `expense-categories:read` | View expense categories |
| `expense-categories:create` | Create new expense categories |
| `expense-categories:update` | Update expense categories |

### Expenses
| Permission | Description |
|------------|-------------|
| `expenses:read` | View expenses list and details |
| `expenses:create` | Create new expenses |
| `expenses:update` | Update expense information |
| `expenses:delete` | Delete expenses |
| `expenses:upload` | Upload expense documents |

### Users
| Permission | Description |
|------------|-------------|
| `users:read` | View users list |
| `users:create` | Create new users (requires Admin API Key) |

### SSE (Real-time)
| Permission | Description |
|------------|-------------|
| `sse:commands:read` | Subscribe to command updates |
| `sse:command-items:read` | Subscribe to command item updates |

---

## Role Permissions Matrix

### Role IDs
| Role | ID |
|------|-----|
| waitress | 1 |
| admin | 2 |
| manager | 3 |
| cooker | 4 |
| accountant | 5 |

### Permissions by Role

#### Waitress (ID: 1)
```typescript
const waitressPermissions = [
  "orders:read",
  "orders:create",
  "orders:update",
  "products:read",
  "commands:read",
  "commands:update",
  "bill-owners:read",
  "sse:commands:read",
  "sse:command-items:read"
];
```

#### Admin (ID: 2)
Admin has **ALL permissions**.

#### Manager (ID: 3)
```typescript
const managerPermissions = [
  // Orders - full access
  "orders:read", "orders:create", "orders:update", "orders:delete",
  // Products - full access
  "products:read", "products:create", "products:update", "products:delete",
  // Stock - full access
  "stock:read", "stock:create", "stock:update", "stock:delete",
  // Invoices - full access
  "invoices:read", "invoices:create", "invoices:export",
  // Commands
  "commands:read", "commands:update",
  // Bill Owners
  "bill-owners:read",
  // Suppliers - full access
  "suppliers:read", "suppliers:create", "suppliers:update", "suppliers:delete",
  // Supplier Catalog - full access
  "supplier-catalog:read", "supplier-catalog:create", "supplier-catalog:update", "supplier-catalog:delete",
  // Purchase Entries - full access
  "purchase-entries:read", "purchase-entries:create", "purchase-entries:upload",
  // Expense Categories - full access
  "expense-categories:read", "expense-categories:create", "expense-categories:update",
  // Expenses - full access
  "expenses:read", "expenses:create", "expenses:update", "expenses:delete", "expenses:upload",
  // Users - read only
  "users:read",
  // SSE
  "sse:commands:read", "sse:command-items:read"
];
```

#### Cooker (ID: 4)
```typescript
const cookerPermissions = [
  "orders:read",
  "products:read",
  "commands:read",
  "commands:update",
  "sse:commands:read",
  "sse:command-items:read"
];
```

#### Accountant (ID: 5)
```typescript
const accountantPermissions = [
  // Invoices
  "invoices:read", "invoices:export",
  // Suppliers - read only
  "suppliers:read",
  // Purchase Entries - full access
  "purchase-entries:read", "purchase-entries:create", "purchase-entries:upload",
  // Expense Categories
  "expense-categories:read", "expense-categories:create", "expense-categories:update",
  // Expenses - full access
  "expenses:read", "expenses:create", "expenses:update", "expenses:delete", "expenses:upload"
];
```

---

## Endpoint Permission Reference

### Public Endpoints (No Auth Required)
| Method | Endpoint | Permission |
|--------|----------|------------|
| GET | `/api/health` | None |
| POST | `/api/auth/signin` | None |

### Protected Endpoints (JWT Required)
| Method | Endpoint | Permission |
|--------|----------|------------|
| GET | `/api/auth/me` | JWT only (no specific permission) |

### Orders
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/orders` | `orders:create` |
| GET | `/api/orders` | `orders:read` |
| GET | `/api/orders/:id` | `orders:read` |
| PUT | `/api/orders/:id` | `orders:update` |
| DELETE | `/api/orders/:id` | `orders:delete` |
| POST | `/api/orders/pay-order` | `orders:update` |
| PATCH | `/api/orders/:id/products/:product_id/complete` | `orders:update` |
| PATCH | `/api/orders/:id/products/:product_id/in-progress` | `orders:update` |
| PATCH | `/api/orders/:id/products/:product_id/cancel` | `orders:update` |

### Products
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/products` | `products:create` |
| GET | `/api/products` | `products:read` |
| GET | `/api/products/:id` | `products:read` |
| PUT | `/api/products/:id` | `products:update` |
| DELETE | `/api/products/:id` | `products:delete` |
| POST | `/api/product-responsibilities` | `products:update` |

### Invoices
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/invoices` | `invoices:create` |
| GET | `/api/invoices` | `invoices:read` |
| GET | `/api/invoices/export` | `invoices:export` |

### Stock
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/stock` | `stock:create` |
| GET | `/api/stock` | `stock:read` |
| PUT | `/api/stock/:product_id/add-or-decrease` | `stock:update` |
| DELETE | `/api/stock/:product_id` | `stock:delete` |
| POST | `/api/stock/bulk` | `stock:create` |

### Bill Owners
| Method | Endpoint | Permission |
|--------|----------|------------|
| GET | `/api/bill-owners/:id` | `bill-owners:read` |

### Commands
| Method | Endpoint | Permission |
|--------|----------|------------|
| PATCH | `/api/commands/:id` | `commands:update` |
| GET | `/api/commands/:area/pending` | `commands:read` |

### Suppliers
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/suppliers` | `suppliers:create` |
| GET | `/api/suppliers` | `suppliers:read` |
| GET | `/api/suppliers/:id` | `suppliers:read` |
| PUT | `/api/suppliers/:id` | `suppliers:update` |
| DELETE | `/api/suppliers/:id` | `suppliers:delete` |

### Supplier Catalog
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/suppliers/:id/products` | `supplier-catalog:create` |
| GET | `/api/suppliers/:id/products` | `supplier-catalog:read` |
| PUT | `/api/suppliers/:id/products/:product_id` | `supplier-catalog:update` |
| DELETE | `/api/suppliers/:id/products/:product_id` | `supplier-catalog:delete` |
| GET | `/api/products/:id/suppliers` | `supplier-catalog:read` |

### Purchase Entries
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/purchase-entries` | `purchase-entries:create` |
| GET | `/api/purchase-entries` | `purchase-entries:read` |
| GET | `/api/purchase-entries/:id` | `purchase-entries:read` |
| GET | `/api/suppliers/:id/purchase-entries` | `purchase-entries:read` |
| POST | `/api/purchase-entries/:id/documents` | `purchase-entries:upload` |

### Expense Categories
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/expense-categories` | `expense-categories:create` |
| GET | `/api/expense-categories` | `expense-categories:read` |
| GET | `/api/expense-categories/:id` | `expense-categories:read` |
| PUT | `/api/expense-categories/:id` | `expense-categories:update` |

### Expenses
| Method | Endpoint | Permission |
|--------|----------|------------|
| POST | `/api/expenses` | `expenses:create` |
| GET | `/api/expenses` | `expenses:read` |
| GET | `/api/expenses/:id` | `expenses:read` |
| PUT | `/api/expenses/:id` | `expenses:update` |
| DELETE | `/api/expenses/:id` | `expenses:delete` |
| POST | `/api/expenses/:id/documents` | `expenses:upload` |

### SSE (Real-time)
| Method | Endpoint | Permission |
|--------|----------|------------|
| GET | `/api/sse/commands/:area` | `sse:commands:read` |
| GET | `/api/sse/command-items/:area` | `sse:command-items:read` |

---

## Frontend Implementation Guide

### 1. Store Permissions on Login

```typescript
// lib/auth.ts
import jwtDecode from 'jwt-decode';

interface AuthResponse {
  token: string;
  username: string;
  roles: { id: number; name: string }[];
  permissions: string[];
}

export async function signIn(username: string, password: string): Promise<AuthResponse> {
  const response = await fetch('/api/auth/signin', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  
  if (!response.ok) throw new Error('Invalid credentials');
  
  const data: AuthResponse = await response.json();
  
  // Store token and permissions
  localStorage.setItem('token', data.token);
  localStorage.setItem('permissions', JSON.stringify(data.permissions));
  
  return data;
}
```

### 2. Permission Helper Functions

```typescript
// lib/permissions.ts
export function getPermissions(): string[] {
  const permissions = localStorage.getItem('permissions');
  return permissions ? JSON.parse(permissions) : [];
}

export function hasPermission(permission: string): boolean {
  return getPermissions().includes(permission);
}

export function hasAnyPermission(permissions: string[]): boolean {
  const userPermissions = getPermissions();
  return permissions.some(p => userPermissions.includes(p));
}

export function hasAllPermissions(permissions: string[]): boolean {
  const userPermissions = getPermissions();
  return permissions.every(p => userPermissions.includes(p));
}
```

### 3. Permission Constants

```typescript
// lib/permissions-constants.ts
export const PERMISSIONS = {
  // Orders
  ORDERS_READ: 'orders:read',
  ORDERS_CREATE: 'orders:create',
  ORDERS_UPDATE: 'orders:update',
  ORDERS_DELETE: 'orders:delete',
  
  // Products
  PRODUCTS_READ: 'products:read',
  PRODUCTS_CREATE: 'products:create',
  PRODUCTS_UPDATE: 'products:update',
  PRODUCTS_DELETE: 'products:delete',
  
  // Stock
  STOCK_READ: 'stock:read',
  STOCK_CREATE: 'stock:create',
  STOCK_UPDATE: 'stock:update',
  STOCK_DELETE: 'stock:delete',
  
  // Invoices
  INVOICES_READ: 'invoices:read',
  INVOICES_CREATE: 'invoices:create',
  INVOICES_EXPORT: 'invoices:export',
  
  // Commands
  COMMANDS_READ: 'commands:read',
  COMMANDS_UPDATE: 'commands:update',
  
  // Bill Owners
  BILL_OWNERS_READ: 'bill-owners:read',
  
  // Suppliers
  SUPPLIERS_READ: 'suppliers:read',
  SUPPLIERS_CREATE: 'suppliers:create',
  SUPPLIERS_UPDATE: 'suppliers:update',
  SUPPLIERS_DELETE: 'suppliers:delete',
  
  // Supplier Catalog
  SUPPLIER_CATALOG_READ: 'supplier-catalog:read',
  SUPPLIER_CATALOG_CREATE: 'supplier-catalog:create',
  SUPPLIER_CATALOG_UPDATE: 'supplier-catalog:update',
  SUPPLIER_CATALOG_DELETE: 'supplier-catalog:delete',
  
  // Purchase Entries
  PURCHASE_ENTRIES_READ: 'purchase-entries:read',
  PURCHASE_ENTRIES_CREATE: 'purchase-entries:create',
  PURCHASE_ENTRIES_UPLOAD: 'purchase-entries:upload',
  
  // Expense Categories
  EXPENSE_CATEGORIES_READ: 'expense-categories:read',
  EXPENSE_CATEGORIES_CREATE: 'expense-categories:create',
  EXPENSE_CATEGORIES_UPDATE: 'expense-categories:update',
  
  // Expenses
  EXPENSES_READ: 'expenses:read',
  EXPENSES_CREATE: 'expenses:create',
  EXPENSES_UPDATE: 'expenses:update',
  EXPENSES_DELETE: 'expenses:delete',
  EXPENSES_UPLOAD: 'expenses:upload',
  
  // Users
  USERS_READ: 'users:read',
  USERS_CREATE: 'users:create',
  
  // SSE
  SSE_COMMANDS_READ: 'sse:commands:read',
  SSE_COMMAND_ITEMS_READ: 'sse:command-items:read',
} as const;

export type Permission = typeof PERMISSIONS[keyof typeof PERMISSIONS];
```

### 4. PermissionGate Component

```tsx
// components/PermissionGate.tsx
import { hasPermission, hasAnyPermission } from '@/lib/permissions';

interface PermissionGateProps {
  permission?: string;
  permissions?: string[];
  requireAll?: boolean;
  children: React.ReactNode;
  fallback?: React.ReactNode;
}

export function PermissionGate({
  permission,
  permissions,
  requireAll = false,
  children,
  fallback = null,
}: PermissionGateProps) {
  let hasAccess = false;

  if (permission) {
    hasAccess = hasPermission(permission);
  } else if (permissions) {
    hasAccess = requireAll 
      ? permissions.every(p => hasPermission(p))
      : hasAnyPermission(permissions);
  }

  return hasAccess ? <>{children}</> : <>{fallback}</>;
}
```

### 5. Usage Examples

```tsx
import { PermissionGate } from '@/components/PermissionGate';
import { PERMISSIONS } from '@/lib/permissions-constants';

// Single permission
<PermissionGate permission={PERMISSIONS.EXPENSES_CREATE}>
  <Button onClick={handleCreateExpense}>Create Expense</Button>
</PermissionGate>

// Any of multiple permissions
<PermissionGate permissions={[PERMISSIONS.EXPENSES_UPDATE, PERMISSIONS.EXPENSES_DELETE]}>
  <ActionMenu expense={expense} />
</PermissionGate>

// All permissions required
<PermissionGate 
  permissions={[PERMISSIONS.EXPENSES_READ, PERMISSIONS.EXPENSES_CREATE]} 
  requireAll={true}
>
  <ExpenseManagementPanel />
</PermissionGate>

// With fallback
<PermissionGate 
  permission={PERMISSIONS.EXPENSES_DELETE}
  fallback={<span>You don't have permission to delete</span>}
>
  <DeleteButton />
</PermissionGate>
```

### 6. Route Protection (Next.js Middleware)

```typescript
// middleware.ts
import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';
import jwtDecode from 'jwt-decode';

const routePermissions: Record<string, string[]> = {
  '/expenses': ['expenses:read'],
  '/expenses/new': ['expenses:create'],
  '/suppliers': ['suppliers:read'],
  '/invoices': ['invoices:read'],
  // Add more route-permission mappings
};

export function middleware(request: NextRequest) {
  const token = request.cookies.get('token')?.value;
  
  if (!token) {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  try {
    const decoded = jwtDecode<{ permissions: string[] }>(token);
    const pathname = request.nextUrl.pathname;
    
    const requiredPermissions = routePermissions[pathname];
    if (requiredPermissions) {
      const hasAccess = requiredPermissions.some(p => 
        decoded.permissions.includes(p)
      );
      
      if (!hasAccess) {
        return NextResponse.redirect(new URL('/unauthorized', request.url));
      }
    }
  } catch {
    return NextResponse.redirect(new URL('/login', request.url));
  }

  return NextResponse.next();
}
```

---

## Error Responses

When a user lacks permission, the API returns:

**401 Unauthorized** - Missing or invalid token
```json
{
  "error": "Authorization header is required"
}
```

**403 Forbidden** - Insufficient permissions
```json
{
  "error": "Insufficient permissions",
  "required": "expenses:delete"
}
```

---

## Best Practices

1. **Always validate on backend** - Frontend checks are for UX only, the backend always validates permissions
2. **Refresh permissions** - Call `/api/auth/me` on app load to get fresh permissions
3. **Handle 403 gracefully** - Show user-friendly messages when actions are forbidden
4. **Hide unavailable actions** - Don't show buttons/links for actions the user can't perform
5. **Use constants** - Define permission strings as constants to avoid typos
