# Platform Organizations API

Create and maintain the organizations (restaurants) that share this POS, their settings and their e-invoicing prefixes. These are **platform-operator** endpoints, not tenant endpoints.

- **Cloud only.** The edge does not serve these routes.
- **Admin API key.** Every route requires `X-API-Key: <ADMIN_API_KEY>`. A user JWT, of any role, is refused with `401`. (Tenancy step 3 replaces the key with a `platform_admin` check on the same paths.)
- **No delete.** An organization's invoices are legal records. Retire an organization with `PATCH {"status": "suspended"}`.

> ⚠️ **New organizations can't be used yet.** Creating an organization stores its settings, but nobody can sign in to it and no data can belong to it until tenancy steps 2–4 land (memberships, tenant columns, row-level security). See `docs/multi-tenancy-plan.md` §8.

> ⚠️ **Settings are stored, not yet enforced.** They are returned to the frontend on [`GET /api/auth/me`](./auth.md#get-current-user), but the running organization still comes from `ORGANIZATION_ID`; the edge still prints from `BUSINESS_*` / `TICKET_*`; the timezone is read only to evaluate the ordering schedules in `settings` (and returned on `/api/auth/me`); a `suspended` organization is not blocked. Each consumer switches over in its own tenancy step.

## Organization Fields

| Field                 | Type           | Required | Default          | Validation                                                                 |
| --------------------- | -------------- | -------- | ---------------- | -------------------------------------------------------------------------- |
| `id`                  | string (UUID)  | —        | generated (v7)   | Read-only                                                                  |
| `name`                | string         | Yes      | —                | Not blank, at most 255 characters (trimmed)                                |
| `slug`                | string         | Yes      | —                | `^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`, unique, **immutable** after create |
| `status`              | string         | No       | `active`         | `active` or `suspended`                                                    |
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

## Endpoints

| Method | Endpoint                                        | Description                               |
| ------ | ----------------------------------------------- | ----------------------------------------- |
| POST   | `/api/platform/organizations`                   | Create an organization                    |
| GET    | `/api/platform/organizations`                   | List all organizations                    |
| GET    | `/api/platform/organizations/:id`               | Get an organization                       |
| PATCH  | `/api/platform/organizations/:id`               | Partially update an organization          |
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

Returns every organization, active and suspended, ordered by name, each with its e-invoicing prefixes or `null` and its `settings`. Not paginated.

```
GET /api/platform/organizations
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
- `name`, `status`, `timezone`, `country`, `locale` and `currency` cannot be cleared: `""` is a `400`.
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

**404 Not Found** - No organization has that id

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
