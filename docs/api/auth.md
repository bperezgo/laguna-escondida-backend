# Authentication

Handles user authentication and current user information.

## Endpoints

| Method | Endpoint            | Description                                      |
| ------ | ------------------- | ------------------------------------------------ |
| GET    | `/api/auth/context` | Which sign-in form to show (public)              |
| POST   | `/api/auth/signin`  | Authenticate user and get token                  |
| GET    | `/api/auth/me`      | Get current authenticated user                   |

---

## Sign-In Context

`GET /api/auth/context`

Public, no token. Tells the sign-in page whether it is talking to the cloud or an edge box, so it shows the right fields and never asks for the restaurant. On an edge box it also returns the box's restaurant name (its `display_name`, falling back to `name`); it never returns any other organization or user data.

### Example Response (200 OK) — cloud

```json
{
  "mode": "cloud"
}
```

### Example Response (200 OK) — edge box

```json
{
  "mode": "edge",
  "organization": {
    "name": "Laguna Escondida"
  }
}
```

| Field               | Type   | Description                                            |
| ------------------- | ------ | ------------------------------------------------------ |
| `mode`              | string | `cloud`: sign in with email. `edge`: sign in with username |
| `organization.name` | string | Edge only. The box's restaurant name                   |

An edge box running a release older than this endpoint answers `404`; treat that as `edge` with no name.

---

## Sign In

`POST /api/auth/signin`

Authenticates a user and returns a JWT token with user information and permissions. The credential depends on where it is called:

- **Cloud:** `email` + `password`. Emails are unique across all organizations, so the email alone identifies the user's organization. Only users holding a cloud role (admin, manager, accountant, agent) can sign in; floor roles (server, cooker) sign in only on an edge box.
- **Edge box:** `username` + `password`. The user signs in to the box's own organization. Any role may sign in.

Unknown fields are ignored, so an old client that still sends `organization` keeps working on an edge box.

### Request Body

| Field    | Type   | Required             | Description                                                     |
| -------- | ------ | -------------------- | --------------------------------------------------------------- |
| email    | string | Cloud: Yes; Edge: No | The user's email. Compared case-insensitively, spaces trimmed   |
| username | string | Edge: Yes; Cloud: No | The user's username within the box's organization               |
| password | string | Yes                  | User's password                                                 |

### Example Request — cloud

```json
{
  "email": "ana@lagunaescondida.co",
  "password": "securepassword123"
}
```

### Example Request — edge box

```json
{
  "username": "cajero1",
  "password": "securepassword123"
}
```

### Example Response (200 OK)

Same body in both modes.

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "ana",
  "roles": [
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

**400 Bad Request** — Cloud: no `email`

```json
{
  "error": "email is required"
}
```

**400 Bad Request** — Edge box: no `username`

```json
{
  "error": "username is required"
}
```

**401 Unauthorized** — Cloud: an unknown email, a suspended organization, an inactive user, a user without a cloud role, or a wrong password. All return the same body.

```json
{
  "error": "Invalid email or password"
}
```

**401 Unauthorized** — Edge box: a username that doesn't exist in the box's organization, an inactive user, or a wrong password. All return the same body.

```json
{
  "error": "Invalid username or password"
}
```

The responses never reveal which emails, usernames or organizations exist; the specific reason is logged server-side.

---

## Get Current User

`GET /api/auth/me`

Returns the currently authenticated user's information including roles and permissions, and the organization the user works in. Use this endpoint to validate the user's session and get fresh permissions data, and to brand and format the app for the organization.

- `organization` is always present. Its optional fields (`legal_name` … `secondary_color`) are `null` when the organization has not set them; fall back to the current defaults.
- `organization` holds settings only: no status, e-invoicing prefixes or timestamps. They are maintained through the [Platform Organizations API](./organizations.md).
- It is the organization named by the token's `org_id`. On the edge, the optional fields stay `null` until the sync carries them.

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
| org_id      | string   | The organization the user signed in to (UUID); every request acts for it |
| username    | string   | User's username                          |
| role_ids    | []int    | Array of role IDs assigned to the user   |
| permissions | []string | Array of permission strings              |
| exp         | number   | Token expiration timestamp (Unix epoch)  |
| iat         | number   | Token issued at timestamp (Unix epoch)   |

A token without `org_id` is rejected with `401 Unauthorized`. Tokens issued before organizations were added carry none, so everyone signs in once more after that release. An edge box also rejects a token whose `org_id` is not the box's organization.

### Organization API Keys (`Bearer lgk_…`)

In the cloud, every endpoint that takes `Authorization: Bearer <jwt_token>` also takes an organization API key, `Authorization: Bearer lgk_…`. A key is not a JWT and carries no claims: the server looks it up on every request and acts as the key's user in the key's organization, with that user's current roles. It stops working as soon as it is revoked, its user is deactivated or deleted, or its organization is suspended; each of those returns the same `401` body as an invalid token. Edge boxes refuse every key with `401`. Keys are issued only for agent users; see [Organization API Keys](./api-keys.md).

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
| agent      | The customer-facing WhatsApp agent's service user  |

admin, manager, accountant and agent are **cloud roles**: only users holding one can sign in to the cloud, and they must have an email. server and cooker are floor roles and sign in only on an edge box.

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
