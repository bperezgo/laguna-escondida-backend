# Platform Organizations API

Create and maintain the organizations (restaurants) that share this POS, their settings and their e-invoicing prefixes. These are **platform-operator** endpoints, not tenant endpoints.

- **Cloud only.** The edge does not serve these routes.
- **Admin API key.** Every route requires `X-API-Key: <ADMIN_API_KEY>`. A user JWT, of any role, is refused with `401`. (Tenancy step 3 replaces the key with a `platform_admin` check on the same paths.)
- **No delete.** An organization's invoices are legal records. Retire an organization with `PATCH {"status": "suspended"}`.

Restaurants can also create their own organization through [self-serve signup](./signup.md). It waits as `pending` until the operator [approves it](#approving-a-signup); its 14-day trial starts then.

> **What changes runtime behavior.** Only the ordering `settings`, `status`, `trial_ends_at` and `plan`. A `suspended` or `pending` organization, or one whose trial has ended, is refused in the cloud (users, API keys, box sync; see [Organization access refusals](./auth.md#organization-access-refusals)); `plan` decides the cloud features it reaches (see [Plans](#plans)) and whether its bills are sent to DIAN. Business info, receipt text and branding are returned on [`GET /api/auth/me`](./auth.md#get-current-user) and not read anywhere else yet.

## Organization Fields

| Field                 | Type           | Required | Default          | Validation                                                                 |
| --------------------- | -------------- | -------- | ---------------- | -------------------------------------------------------------------------- |
| `id`                  | string (UUID)  | —        | generated (v7)   | Read-only                                                                  |
| `name`                | string         | Yes      | —                | Not blank, at most 255 characters (trimmed)                                |
| `slug`                | string         | Yes      | —                | `^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`, unique, **immutable** after create |
| `status`              | string         | No       | `active`         | `trial`, `active` or `suspended`; `trial` needs a `trial_ends_at`, sent or stored. Reads may also return `pending` (a signup waiting for approval), which can't be sent |
| `plan`                | string         | No       | `full`           | `starter`, `einvoicing` or `full`                                          |
| `trial_ends_at`       | string \| null | No       | `null`           | RFC 3339. From this instant a `trial` organization is expired; an `active` one never expires. Always `null` and not accepted while `pending` |
| `timezone`            | string         | No       | `America/Bogota` | A valid IANA time zone name                                                |
| `country`             | string         | No       | `CO`             | Two uppercase letters (ISO 3166-1 alpha-2)                                 |
| `locale`              | string         | No       | `es-CO`          | `ll-RR`, e.g. `es-CO`                                                      |
| `currency`            | string         | No       | `COP`            | Three uppercase letters (ISO 4217)                                         |
| `legal_name`          | string \| null | No       | `null`           | At most 255 characters                                                     |
| `nit`                 | string \| null | No       | `null`           | At most 255 characters                                                     |
| `address`             | string \| null | No       | `null`           | At most 255 characters                                                     |
| `phone`               | string \| null | No       | `null`           | At most 255 characters                                                     |
| `ticket_footer`       | string \| null | No       | `null`           | At most 1000 characters                                                    |
| `ticket_legal_notice` | string \| null | No       | `null`           | At most 1000 characters                                                    |
| `display_name`        | string \| null | No       | `null`           | At most 255 characters                                                     |
| `logo_url`            | string \| null | No       | `null`           | Absolute `https` URL, at most 255 characters                               |
| `primary_color`       | string \| null | No       | `null`           | `#RRGGBB`                                                                  |
| `secondary_color`     | string \| null | No       | `null`           | `#RRGGBB`                                                                  |
| `einvoicing`          | object \| null | —        | `null`           | See [Set E-Invoicing Prefixes](#set-e-invoicing-prefixes)                  |
| `settings`            | object         | —        | see below        | Read-only here; changed through the tenant's [`PATCH /api/organization/settings`](./organization-settings.md) |
| `created_at`          | string         | —        | —                | RFC 3339, read-only                                                        |
| `updated_at`          | string         | —        | —                | RFC 3339, read-only                                                        |

`settings.ordering` holds the delivery and takeaway pause and schedule (see [Organization Settings](./organization-settings.md#model)). An organization that never saved any reads as:

```json
{
  "ordering": {
    "delivery": { "paused": false, "paused_reason": null, "schedule": [] },
    "takeaway": { "paused": false, "paused_reason": null, "schedule": [] }
  }
}
```

## Plans

| Feature | Routes | `starter` | `einvoicing` | `full` |
|---|---|---|---|---|
| Core: orders, kitchen, products, stock, suppliers, purchases, expenses, bill owners, users, reports, financial summary, organization settings, box enrollment | every route not listed below | yes | yes | yes |
| E-invoicing | `/api/invoices*`, `/api/support-documents*` | no | yes | yes |
| Customer channels | `/api/reservations*`, `/api/sse/reservations`, `/api/contacts*`, `/api/feedback*`, `/api/delivery-zones*`, `/api/deliveries*`, `/api/notification-channels*`, `/api/notification-rules*` | no | no | yes |
| Payments and integrations | `/api/organization/integrations*`, `/api/organization/payment-settings*` | no | no | yes |
| Agents and MCP | `/api/organization/api-keys*`, any request with an organization API key, any request by a user with the agent role | no | no | yes |

A refused request answers `403 {"code": "plan_required", "required_plan": "<plan>"}`. Bills received from a box while the plan excludes e-invoicing are stored but their pending invoice is marked `skipped` and never sent to DIAN, even after an upgrade. Changing the plan keeps every record and API key.

## Endpoints

| Method | Endpoint                                        | Description                               |
| ------ | ----------------------------------------------- | ----------------------------------------- |
| POST   | `/api/platform/organizations`                   | Create an organization                    |
| GET    | `/api/platform/organizations`                   | List all organizations                    |
| GET    | `/api/platform/organizations/:id`               | Get an organization                       |
| PATCH  | `/api/platform/organizations/:id`               | Partially update an organization          |
| POST   | `/api/platform/organizations/:id/start-trial`   | Start the 14-day trial of a pending signup |
| PUT    | `/api/platform/organizations/:id/einvoicing`    | Create or replace the e-invoicing prefixes |

### Common Errors

**401 Unauthorized** - Missing or wrong `X-API-Key` (a JWT is not accepted)

```json
{
  "error": "API key is required"
}
```

**400 Bad Request** - A field failed validation. `field` names it; nothing is stored.

```json
{
  "error": "invalid timezone: must be an IANA time zone name",
  "field": "timezone"
}
```

**404 Not Found** - No organization has that id (including a malformed id)

```json
{
  "error": "Organization not found"
}
```

---

## Create Organization

Creates an organization, optionally with its e-invoicing prefixes. The organization and its prefixes are stored together or not at all.

```
POST /api/platform/organizations
```

### Request Body

Any field from [Organization Fields](#organization-fields) except the read-only ones. Omitted or empty settings take their default (required settings) or stay `null` (optional ones).

| Field        | Type   | Required | Description                                                            |
| ------------ | ------ | -------- | ---------------------------------------------------------------------- |
| `name`       | string | Yes      | Legal or commercial name                                               |
| `slug`       | string | Yes      | Unique, URL-safe identifier; cannot be changed later                   |
| `einvoicing` | object | No       | `{"invoice_prefix": "...", "support_document_prefix": "..."}`         |
| …            |        | No       | Any other setting                                                      |

### Example Request (minimal)

```json
{
  "name": "Restaurante X S.A.S",
  "slug": "restaurante-x"
}
```

### Example Response (201 Created)

```json
{
  "id": "01a16616-deea-772b-957c-0a5a916903cc",
  "name": "Restaurante X S.A.S",
  "slug": "restaurante-x",
  "status": "active",
  "plan": "full",
  "trial_ends_at": null,
  "timezone": "America/Bogota",
  "country": "CO",
  "locale": "es-CO",
  "currency": "COP",
  "legal_name": null,
  "nit": null,
  "address": null,
  "phone": null,
  "ticket_footer": null,
  "ticket_legal_notice": null,
  "display_name": null,
  "logo_url": null,
  "primary_color": null,
  "secondary_color": null,
  "einvoicing": null,
  "settings": {
    "ordering": {
      "delivery": { "paused": false, "paused_reason": null, "schedule": [] },
      "takeaway": { "paused": false, "paused_reason": null, "schedule": [] }
    }
  },
  "created_at": "2026-10-23T00:08:57.322452Z",
  "updated_at": "2026-10-23T00:08:57.322452Z"
}
```

### Example Request (with settings and e-invoicing prefixes)

```json
{
  "name": "El Muelle S.A.S",
  "slug": "el-muelle",
  "display_name": "El Muelle",
  "primary_color": "#0E7C86",
  "einvoicing": {
    "invoice_prefix": "MUE",
    "support_document_prefix": "DSM"
  }
}
```

### Example Response (201 Created)

```json
{
  "id": "01a16616-df0c-776a-a887-622fef3405be",
  "name": "El Muelle S.A.S",
  "slug": "el-muelle",
  "status": "active",
  "plan": "full",
  "trial_ends_at": null,
  "timezone": "America/Bogota",
  "country": "CO",
  "locale": "es-CO",
  "currency": "COP",
  "legal_name": null,
  "nit": null,
  "address": null,
  "phone": null,
  "ticket_footer": null,
  "ticket_legal_notice": null,
  "display_name": "El Muelle",
  "logo_url": null,
  "primary_color": "#0E7C86",
  "secondary_color": null,
  "einvoicing": {
    "invoice_prefix": "MUE",
    "support_document_prefix": "DSM"
  },
  "settings": {
    "ordering": {
      "delivery": { "paused": false, "paused_reason": null, "schedule": [] },
      "takeaway": { "paused": false, "paused_reason": null, "schedule": [] }
    }
  },
  "created_at": "2026-10-23T00:08:57.356478Z",
  "updated_at": "2026-10-23T00:08:57.356478Z"
}
```

### Error Responses

**400 Bad Request** - Invalid field (see [Common Errors](#common-errors)), e.g. equal prefixes:

```json
{
  "error": "invalid support_document_prefix: must differ from invoice_prefix",
  "field": "support_document_prefix"
}
```

**409 Conflict** - Another organization already has the slug

```json
{
  "error": "Organization slug is already taken",
  "field": "slug"
}
```

---

## List Organizations

Returns every organization, whatever its status, ordered by name, each with its e-invoicing prefixes or `null` and its `settings`. Not paginated.

```
GET /api/platform/organizations
GET /api/platform/organizations?status=pending
```

| Query    | Description |
| -------- | ----------- |
| `status` | Optional. `pending`, `trial`, `active` or `suspended`: only organizations with that status. `?status=pending` lists the signups waiting for approval, each with its `plan` and `created_at`. |

**400 Bad Request** — an unknown `status`

```json
{
  "error": "invalid status: must be pending, trial, active or suspended",
  "field": "status"
}
```

### Example Response (200 OK)

```json
{
  "organizations": [
    {
      "id": "bb0cad3e-be13-4bad-9a37-bba9525bfece",
      "name": "Laguna Escondida, Restaurante y Pesca S.A.S",
      "slug": "laguna-escondida",
      "status": "active",
      "plan": "full",
      "trial_ends_at": null,
      "timezone": "America/Bogota",
      "country": "CO",
      "locale": "es-CO",
      "currency": "COP",
      "legal_name": null,
      "nit": null,
      "address": null,
      "phone": null,
      "ticket_footer": null,
      "ticket_legal_notice": null,
      "display_name": "Laguna Escondida",
      "logo_url": null,
      "primary_color": null,
      "secondary_color": null,
      "einvoicing": {
        "invoice_prefix": "LAG",
        "support_document_prefix": "DS"
      },
      "settings": {
        "ordering": {
          "delivery": { "paused": true, "paused_reason": "cocina saturada", "schedule": [ { "day": "fri", "opens": "18:00", "closes": "02:00" } ] },
          "takeaway": { "paused": false, "paused_reason": null, "schedule": [] }
        }
      },
      "created_at": "2025-03-14T15:02:11.512034Z",
      "updated_at": "2025-03-14T15:02:11.512034Z"
    },
    {
      "id": "01a16616-deea-772b-957c-0a5a916903cc",
      "name": "Restaurante X S.A.S",
      "slug": "restaurante-x",
      "status": "suspended",
      "plan": "full",
      "trial_ends_at": null,
      "timezone": "America/Bogota",
      "country": "CO",
      "locale": "es-CO",
      "currency": "COP",
      "legal_name": null,
      "nit": null,
      "address": null,
      "phone": null,
      "ticket_footer": null,
      "ticket_legal_notice": null,
      "display_name": null,
      "logo_url": null,
      "primary_color": null,
      "secondary_color": null,
      "einvoicing": null,
      "settings": {
        "ordering": {
          "delivery": { "paused": false, "paused_reason": null, "schedule": [] },
          "takeaway": { "paused": false, "paused_reason": null, "schedule": [] }
        }
      },
      "created_at": "2026-10-23T00:08:57.322452Z",
      "updated_at": "2026-10-23T00:10:02.118901Z"
    }
  ]
}
```

---

## Get Organization

```
GET /api/platform/organizations/:id
```

### Example Response (200 OK)

Same shape as one element of the list.

### Error Responses

**404 Not Found** - No organization has that id

---

## Update Organization

Changes only the fields present in the body.

```
PATCH /api/platform/organizations/:id
```

- A field that is absent is left unchanged.
- An **optional** field sent as `""` is cleared (becomes `null`).
- `name`, `status`, `plan`, `timezone`, `country`, `locale` and `currency` cannot be cleared: `""` is a `400`.
- `trial_ends_at` sent as `""` is cleared, unless the organization stays in `trial` (`400`).
- **Activate** a paying customer with `{"status": "active", "plan": "einvoicing"}`; an active organization never expires.
- **Extend** a trial with a later `trial_ends_at`, e.g. `{"trial_ends_at": "2026-12-31T23:59:59-05:00"}`.
- **`pending`** can't be sent, on create or update: only signup creates it. A `pending` organization can move to `active` or `suspended` here, and its `plan` and other fields can change (it stays `pending`). It can't be moved to `trial` here, nor given a `trial_ends_at`: use [Start a Trial](#start-a-trial).
- Changes apply within a minute to users and API keys, and within about 5 minutes to the box (its refusal backoff).
- `slug` cannot be changed: a body containing `slug` is a `400`.
- `updated_at` is always bumped.

### Example Request

```json
{
  "legal_name": "Laguna Escondida, Restaurante y Pesca S.A.S",
  "nit": "901234567-8",
  "ticket_footer": "¡Gracias por su visita!",
  "logo_url": "https://cdn.lagunaescondida.co/logo.png",
  "primary_color": "#0E7C86"
}
```

### Example Request (clear a field / suspend)

```json
{
  "ticket_footer": "",
  "status": "suspended"
}
```

### Example Response (200 OK)

The full updated organization, same shape as [Get Organization](#get-organization).

### Error Responses

**400 Bad Request** - Slug in the body

```json
{
  "error": "invalid slug: cannot be changed",
  "field": "slug"
}
```

**400 Bad Request** - Clearing a required field

```json
{
  "error": "invalid timezone: is required and cannot be cleared",
  "field": "timezone"
}
```

**400 Bad Request** - Trial without an end, or an unknown plan

```json
{
  "error": "invalid trial_ends_at: is required while the status is trial",
  "field": "trial_ends_at"
}
```

```json
{
  "error": "invalid plan: must be starter, einvoicing or full",
  "field": "plan"
}
```

**400 Bad Request** - A pending organization patched into a trial, or `status: pending`

```json
{
  "error": "invalid status: a pending organization's trial is started with start-trial",
  "field": "status"
}
```

```json
{
  "error": "invalid status: must be trial, active or suspended",
  "field": "status"
}
```

**400 Bad Request** - A `trial_ends_at` on an organization that stays pending

```json
{
  "error": "invalid trial_ends_at: is not accepted while the organization is pending",
  "field": "trial_ends_at"
}
```

**404 Not Found** - No organization has that id

---

## Start a Trial

Moves a `pending` organization (a signup) to `trial`, with `trial_ends_at` 14 days after this call and its stored `plan` unchanged. This is the only way a pending organization becomes a trial; a future payment confirmation will go through the same operation. Its users can sign in at once on this replica, and within a minute on the others.

```
POST /api/platform/organizations/:id/start-trial
```

No request body.

### Example Response (200 OK)

The full updated organization, same shape as [Get Organization](#get-organization).

```json
{
  "id": "019a1b2c-3d4e-7f00-8a1b-2c3d4e5f6a7b",
  "slug": "restaurante-dona-rosa",
  "name": "Restaurante Doña Rosa",
  "status": "trial",
  "plan": "einvoicing",
  "trial_ends_at": "2026-12-15T15:00:00Z"
}
```

(other fields omitted)

### Error Responses

**409 Conflict** - The organization is not `pending` (already in trial, active or suspended); nothing changes

```json
{
  "error": "Organization is not pending approval",
  "code": "not_pending"
}
```

**404 Not Found** - No organization has that id

### Approving a Signup

1. Find waiting signups: `GET /api/platform/organizations?status=pending`.
2. Either start the trial: `POST /api/platform/organizations/:id/start-trial`,
3. or skip the trial for a paying customer: `PATCH /api/platform/organizations/:id` with `{"status": "active"}`,
4. or reject it: `PATCH` with `{"status": "suspended"}`.

Nobody is notified; the restaurant finds out by signing in.

---

## Set E-Invoicing Prefixes

Creates or replaces the organization's electronic-invoice and support-document prefixes.

```
PUT /api/platform/organizations/:id/einvoicing
```

> ⚠️ **Change prefixes only for a new DIAN resolution.** Numbering is counted per organization and prefix: a new prefix starts at consecutive **1** on its first submitted document, and invoices still pending submission take the new prefix. The old prefix's counter is kept, so switching back resumes where it left off.

### Request Body

| Field                     | Type   | Required | Description                                      |
| ------------------------- | ------ | -------- | ------------------------------------------------ |
| `invoice_prefix`          | string | Yes      | `^[A-Z0-9]{1,10}$`                               |
| `support_document_prefix` | string | Yes      | `^[A-Z0-9]{1,10}$`, must differ from the invoice prefix |

### Example Request

```json
{
  "invoice_prefix": "MUE",
  "support_document_prefix": "DSM"
}
```

### Example Response (200 OK)

```json
{
  "invoice_prefix": "MUE",
  "support_document_prefix": "DSM"
}
```

### Error Responses

**400 Bad Request** - Invalid or equal prefixes

```json
{
  "error": "invalid invoice_prefix: must be 1-10 uppercase letters or digits",
  "field": "invoice_prefix"
}
```

**404 Not Found** - No organization has that id
