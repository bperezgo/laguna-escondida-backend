# Authentication

Handles user authentication and current user information.

## Endpoints

| Method | Endpoint            | Description                                      |
| ------ | ------------------- | ------------------------------------------------ |
| GET    | `/api/auth/context` | Which sign-in form to show (public)              |
| POST   | `/api/auth/signin`  | Authenticate user and get token                  |
| POST   | `/api/auth/google`  | Sign in with a Google ID token (cloud only)      |
| GET    | `/api/auth/me`      | Get current authenticated user                   |

Self-serve signup is in [Signup](./signup.md); accepting an invitation is in [Invitations](./invitations.md).

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

**403 Forbidden** — Cloud: the email and password are correct, the user is active and holds a cloud role, but the organization is `pending`: its trial has not started. No sign-in token is returned. When the user holds the admin role the body also carries `setup_token`, a [setup token](./billing.md#the-setup-token) to continue signup with; any other user gets none. A wrong password for the same email still gets the `401` above.

```json
{
  "error": "Approval pending",
  "code": "pending_approval",
  "setup_token": "<setup token>"
}
```

**403 Forbidden** — Cloud: the email and password are correct, the user is active and holds a cloud role, but the organization's trial has ended. No sign-in token is returned. When the user holds the admin role the body also carries `setup_token`, a [setup token](./billing.md#the-setup-token) that opens the [recovery routes](./billing.md#recovery); any other user gets none. A wrong password for the same email still gets the `401` above.

```json
{
  "error": "Trial ended",
  "code": "trial_expired",
  "setup_token": "<setup token>"
}
```

**403 Forbidden** — Cloud: the same, for an organization that is unpaid: its paid period ended and no charge was approved (see [Charges](./billing.md#charges)). `setup_token` only for an admin.

```json
{
  "error": "Payment required",
  "code": "payment_required",
  "setup_token": "<setup token>"
}
```

The responses never reveal which emails, usernames or organizations exist; the specific reason is logged server-side. A user without an email (a box-only manager) can't sign in to the cloud.

---

## Sign In with Google

Signs in the cloud user whose email equals the Google account's. Any user with an email can use it, whether they signed up with Google, accepted an invitation or set a password. Cloud only, and only when the cloud has a Google client id (`GOOGLE_CLIENT_ID`); otherwise `404`. Rate-limited per client address (30 per 10 minutes).

`POST /api/auth/google`

The ID token comes from Google Identity Services in the browser. It is accepted only when Google's signature is valid, its audience is the cloud's client id, it is unexpired and its email is verified by Google.

### Request Body

| Field      | Type   | Required | Description         |
| ---------- | ------ | -------- | ------------------- |
| `id_token` | string | Yes      | Google ID token (JWT) |

```json
{
  "id_token": "<ID token from Google>"
}
```

### Example Response (200 OK)

The same body as `POST /api/auth/signin`.

### Error Responses

**400 Bad Request** — no `id_token`

```json
{
  "error": "id_token is required"
}
```

**401 Unauthorized** — an invalid token, an unknown email, a suspended organization, an inactive user or a user without a cloud role. All return the same body.

```json
{
  "error": "Google sign-in failed"
}
```

**403 Forbidden** — the account matches an active cloud user whose organization is `pending`. `setup_token` is present only for an admin (see [the setup token](./billing.md#the-setup-token)).

```json
{
  "error": "Approval pending",
  "code": "pending_approval",
  "setup_token": "<setup token>"
}
```

**403 Forbidden** — the account matches an active cloud user whose organization's trial has ended. `setup_token` is present only for an admin; it opens the [recovery routes](./billing.md#recovery).

```json
{
  "error": "Trial ended",
  "code": "trial_expired",
  "setup_token": "<setup token>"
}
```

**403 Forbidden** — the same, for an organization that is unpaid. `setup_token` only for an admin.

```json
{
  "error": "Payment required",
  "code": "payment_required",
  "setup_token": "<setup token>"
}
```

**429 Too Many Requests** — over the per-address limit, with `Retry-After`.

---

## Get Current User

`GET /api/auth/me`

Returns the currently authenticated user's information including roles and permissions, and the organization the user works in. Use this endpoint to validate the user's session and get fresh permissions data, and to brand and format the app for the organization.

- `organization` is always present. Its optional fields (`legal_name` … `secondary_color`) are `null` when the organization has not set them; fall back to the current defaults.
- `organization` holds settings only: no status, e-invoicing prefixes or timestamps. They are maintained through the [Platform Organizations API](./organizations.md).
- It is the organization named by the token's `org_id`. On the edge, the optional fields stay `null` until the sync carries them.
- `billing` is present **only in the cloud and only for a user with `billing:manage`** (the admin role). Everyone else gets no `billing` key and shows the trial's days left from `organization.status` and `organization.trial_ends_at`, as before.

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
    "status": "active",
    "plan": "full",
    "trial_ends_at": null,
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
| `status`              | string         | `trial`, `active` or `suspended`                        |
| `plan`                | string         | `starter`, `einvoicing` or `full`; hide what it excludes (see [Plans](./organizations.md#plans)) |
| `trial_ends_at`       | string \| null | RFC 3339; set while `status` is `trial`. Show the days left |
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

### Billing Fields

What the app's subscription notice needs. The full subscription is [`GET /api/billing`](./billing.md#get-the-subscription).

| Field                | Type           | Description |
| -------------------- | -------------- | ----------- |
| `period`             | string         | `monthly` or `annual` |
| `has_card`           | boolean        | A card is saved |
| `card_entry`         | boolean        | A card can be saved or changed in this cloud |
| `cancelled`          | boolean        | The subscription is cancelled and works until `due_date` |
| `due_date`           | string \| null | The trial's end during the trial, the end of the paid period after. `null` for an organization activated by hand |
| `next_charge_amount` | number \| null | Pesos to be charged on `due_date`; `null` when nothing will be charged |
| `payment_failed`     | boolean        | The latest charge failed and the organization still works |
| `access_ends_at`     | string \| null | When the organization stops unless a charge is approved |

```json
"billing": {
  "period": "monthly",
  "has_card": true,
  "card_entry": true,
  "cancelled": false,
  "due_date": "2026-12-15T14:00:00Z",
  "next_charge_amount": 40000,
  "payment_failed": false,
  "access_ends_at": "2026-12-18T14:00:00Z"
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

In the cloud, every endpoint that takes `Authorization: Bearer <jwt_token>` also takes an organization API key, `Authorization: Bearer lgk_…`. A key works only for an organization on the `full` plan (otherwise `403 plan_required`). A key is not a JWT and carries no claims: the server looks it up on every request and acts as the key's user in the key's organization, with that user's current roles. It stops working as soon as it is revoked, its user is deactivated or deleted, or its organization is not `active` (suspended or pending included); each of those returns the same `401` body as an invalid token. Edge boxes refuse every key with `401`. Keys are issued only for agent users; see [Organization API Keys](./api-keys.md).

### Organization Access Refusals

In the cloud, every request with a JWT or an API key is checked against its organization after authentication and before permissions, in this order. Edge boxes never check plans or trials.

**403 Forbidden** — the organization is suspended

```json
{ "error": "Organization suspended", "code": "organization_suspended" }
```

**403 Forbidden** — the organization is waiting for its signup to be approved. This refusal of an authenticated request never carries a `setup_token`; only a sign-in does

```json
{ "error": "Approval pending", "code": "pending_approval" }
```

A [setup token](./billing.md#the-setup-token) sent to any route outside `/api/billing/setup` and `/api/billing/recovery` gets the `401` of an invalid token.

**403 Forbidden** — the organization's trial has ended, whenever the token was issued. A trial with a saved card and a subscription that is not cancelled gets three more days, while its first charge and the retries run; any other trial stops at `trial_ends_at`

```json
{ "error": "Trial ended", "code": "trial_expired" }
```

**403 Forbidden** — the organization is unpaid: it is `active` with a paid period that ended, three days ago when its subscription is not cancelled, or at that moment when it is. An approved charge lifts it within a minute. An `active` organization without a paid period never gets this

```json
{ "error": "Payment required", "code": "payment_required" }
```

Neither refusal of an authenticated request carries a `setup_token`; only a sign-in does. Boxes of an expired or unpaid organization are refused as those of a suspended one.

**403 Forbidden** — the plan doesn't include the route's feature, or the request uses an API key or comes from the agent role on a plan without agents. `required_plan` is the cheapest plan that includes it.

```json
{ "error": "Your plan does not include this feature", "code": "plan_required", "required_plan": "einvoicing" }
```

A change to an organization's status, plan or trial end applies within 60 seconds.

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

### Billing
- `billing:manage` - Read and change the organization's own subscription: card, cancel, resume (admin only, cloud)

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
