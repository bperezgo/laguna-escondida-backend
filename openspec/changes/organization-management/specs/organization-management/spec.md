## Purpose

Lets the platform operator create and maintain the organizations (restaurants) that share this POS. Each organization has a settings record (identity, locale, business info, receipt text, branding) and e-invoicing prefixes, managed through platform-level endpoints.

## ADDED Requirements

### Requirement: An organization has an identity, a status and settings

An organization SHALL have:
- an id (UUID), a `name`, and a `slug` that is unique across organizations
- a `status` of `active` or `suspended`
- `timezone`, `country`, `locale` and `currency`
- optional business info: `legal_name`, `nit`, `address`, `phone`
- optional receipt text: `ticket_footer`, `ticket_legal_notice`
- optional branding: `display_name`, `logo_url`, `primary_color`, `secondary_color`
- `created_at` and `updated_at` timestamps

#### Scenario: Laguna after the migration

- **WHEN** the migration has run on a database containing Laguna's organization (`bb0cad3e-be13-4bad-9a37-bba9525bfece`)
- **THEN** Laguna has slug `laguna-escondida`, status `active`, timezone `America/Bogota`, country `CO`, locale `es-CO`, currency `COP`
- **AND** its name, id and e-invoicing prefixes are unchanged

### Requirement: Organization fields are validated

Every create and update SHALL validate each field it sets and reject the whole request with a validation error, storing nothing, when any field is invalid:
- `name`: not empty after trimming, at most 255 characters
- `slug`: matches `^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$` (3–40 lowercase letters, digits or hyphens, with no leading or trailing hyphen)
- `status`: `active` or `suspended`
- `timezone`: a valid IANA time zone name
- `country`: two uppercase letters (ISO 3166-1 alpha-2)
- `locale`: a language and region tag of the form `ll-RR`
- `currency`: three uppercase letters (ISO 4217)
- `logo_url`: an absolute `https` URL
- `primary_color` and `secondary_color`: `#RRGGBB` hex colors
- every other text field: at most 255 characters, except `ticket_footer` and `ticket_legal_notice` at 1000

#### Scenario: Unknown time zone

- **WHEN** an organization is created with timezone `America/Bogata`
- **THEN** the request fails with a validation error that names the field
- **AND** no organization is stored

#### Scenario: Invalid color

- **WHEN** an organization is updated with `primary_color` `blue`
- **THEN** the request fails with a validation error
- **AND** the organization is unchanged

### Requirement: The platform operator creates an organization

`POST /api/platform/organizations` SHALL create an organization from a `name`, a `slug` and any of the optional settings.
- Omitted fields default to `active`, `America/Bogota`, `CO`, `es-CO` and `COP`.
- The request MAY include e-invoicing prefixes. When it does, the organization and its prefixes SHALL be stored together or not at all.
- The response SHALL be `201` with the created organization, including its e-invoicing prefixes or `null`.

#### Scenario: Minimal create

- **WHEN** the operator posts `{"name": "Restaurante X S.A.S", "slug": "restaurante-x"}`
- **THEN** the response is `201` with a new id, status `active`, timezone `America/Bogota`, country `CO`, locale `es-CO`, currency `COP`, and `einvoicing: null`

#### Scenario: Create with e-invoicing prefixes

- **WHEN** the operator creates an organization with `einvoicing: {"invoice_prefix": "RX", "support_document_prefix": "DSX"}`
- **THEN** the organization and its prefixes are both stored
- **AND** the response includes the prefixes

#### Scenario: Invalid prefixes roll back the organization

- **WHEN** the operator creates an organization whose two e-invoicing prefixes are equal
- **THEN** the request fails with a validation error
- **AND** no organization is stored

#### Scenario: Slug already taken

- **WHEN** the operator creates an organization with slug `laguna-escondida`
- **THEN** the response is `409` and no organization is stored

### Requirement: The platform operator lists and reads organizations

`GET /api/platform/organizations` SHALL return every organization (active and suspended), ordered by name, each with its e-invoicing prefixes or `null`. `GET /api/platform/organizations/:id` SHALL return one organization in the same shape, or `404` when no organization has that id.

#### Scenario: List includes suspended organizations

- **WHEN** one organization is active and another is suspended
- **THEN** the list returns both, ordered by name, each with its status

#### Scenario: Unknown id

- **WHEN** the operator requests an id that does not exist
- **THEN** the response is `404`

### Requirement: The platform operator updates an organization

`PATCH /api/platform/organizations/:id` SHALL change only the fields present in the body and leave the others unchanged.
- An optional field sent as an empty string SHALL be cleared.
- `name`, `status`, `timezone`, `country`, `locale` and `currency` SHALL NOT be cleared. An empty value for any of them is a validation error.
- `slug` SHALL NOT be changeable. A body containing `slug` is a validation error.
- The response SHALL be `200` with the updated organization, or `404` when the id does not exist.

#### Scenario: Partial update

- **WHEN** the operator patches `{"display_name": "Laguna"}`
- **THEN** only `display_name` and `updated_at` change

#### Scenario: Clear an optional field

- **WHEN** the operator patches `{"ticket_footer": ""}` on an organization that has a ticket footer
- **THEN** the organization's ticket footer is empty

#### Scenario: Required field cannot be cleared

- **WHEN** the operator patches `{"timezone": ""}`
- **THEN** the request fails with a validation error and the organization is unchanged

#### Scenario: Slug is immutable

- **WHEN** the operator patches `{"slug": "new-slug"}`
- **THEN** the request fails with a validation error and the slug is unchanged

#### Scenario: Suspend

- **WHEN** the operator patches `{"status": "suspended"}`
- **THEN** the organization's status is `suspended` and every other field is unchanged

### Requirement: The platform operator sets an organization's e-invoicing prefixes

`PUT /api/platform/organizations/:id/einvoicing` SHALL create or replace the organization's invoice and support-document prefixes.
- Each prefix SHALL match `^[A-Z0-9]{1,10}$`, and the two SHALL differ.
- The response SHALL be `200` with the stored prefixes, or `404` when the organization does not exist.
- Invoice numbering is counted per organization and prefix, so a new prefix SHALL start at consecutive 1 when its first document is submitted. Counters already reached for the old prefix SHALL be kept.

#### Scenario: First configuration

- **WHEN** the operator puts `{"invoice_prefix": "RX", "support_document_prefix": "DSX"}` on an organization without e-invoicing settings
- **THEN** the settings are stored
- **AND** reading the organization returns them

#### Scenario: Replace the prefixes

- **WHEN** the operator puts new prefixes on an organization that already has some
- **THEN** the new prefixes replace the old ones
- **AND** the old prefix's counter is kept

#### Scenario: Equal prefixes

- **WHEN** both prefixes are `RX`
- **THEN** the request fails with a validation error and the stored settings are unchanged

#### Scenario: Lowercase prefix

- **WHEN** the invoice prefix is `rx`
- **THEN** the request fails with a validation error

### Requirement: Organizations are never deleted

The system SHALL NOT offer a way to delete an organization. An organization is retired by setting its status to `suspended`.

#### Scenario: No delete route

- **WHEN** the operator sends `DELETE /api/platform/organizations/:id`
- **THEN** the response is `404` (no such route)
- **AND** the organization still exists

### Requirement: Organization endpoints require the admin API key and exist only in the cloud

Every `/api/platform/organizations` route SHALL require a valid admin API key in the `X-API-Key` header and SHALL respond `401` without one. A user JWT, of any role, SHALL NOT grant access. These routes SHALL be served only in cloud mode.

#### Scenario: Missing key

- **WHEN** a request to `GET /api/platform/organizations` has no `X-API-Key`
- **THEN** the response is `401`

#### Scenario: A signed-in admin is not enough

- **WHEN** a user with the admin role calls the endpoint with a valid JWT and no API key
- **THEN** the response is `401`

#### Scenario: Edge mode

- **WHEN** the backend runs in edge mode
- **THEN** `/api/platform/organizations` is not served

### Requirement: Stored settings do not yet change runtime behavior

Creating, updating or suspending an organization SHALL NOT change the behavior of any existing endpoint, cron, sync or edge feature. In particular:
- the running organization still comes from the deployment's configuration
- suspending an organization does not block its users
- timezone, business info, receipt text and branding are not yet read anywhere, except to return them on `GET /api/auth/me`

#### Scenario: Suspending Laguna does not lock anyone out

- **WHEN** Laguna's status is set to `suspended`
- **THEN** Laguna's users still sign in and create orders exactly as before

### Requirement: The signed-in user's organization is returned on `/auth/me`

`GET /api/auth/me` SHALL include an `organization` object for the organization the request runs as, so the frontend can show its branding and formatting:
- `id`, `slug`, `name`
- `timezone`, `country`, `locale`, `currency`
- `legal_name`, `nit`, `address`, `phone`, `ticket_footer`, `ticket_legal_notice`, `display_name`, `logo_url`, `primary_color`, `secondary_color`, each `null` when unset

It SHALL NOT include the status, the e-invoicing prefixes or the timestamps. Until tenancy step 3 puts the organization in the token, the organization is the deployment's configured one (`ORGANIZATION_ID`). The edge returns its own database's row, whose optional settings stay `null` until the sync pull carries them (step 6).

#### Scenario: Laguna's user

- **WHEN** a signed-in user calls `GET /api/auth/me` on a deployment configured with Laguna's organization
- **THEN** the response has the user's fields as before
- **AND** `organization` has slug `laguna-escondida`, display name `Laguna Escondida`, timezone `America/Bogota`, locale `es-CO` and currency `COP`

#### Scenario: Settings changed by the platform operator

- **WHEN** the operator PATCHes Laguna's `primary_color` to `#0E7C86`
- **THEN** the next `GET /api/auth/me` in the cloud returns `organization.primary_color` `#0E7C86`

#### Scenario: Configured organization missing

- **WHEN** the configured organization does not exist
- **THEN** `GET /api/auth/me` responds `500` without the database error
