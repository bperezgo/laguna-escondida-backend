## MODIFIED Requirements

### Requirement: An organization has an identity, a status and settings

An organization SHALL have:
- an id (UUID), a `name`, and a `slug` that is unique across organizations
- a `status` of `trial`, `active` or `suspended`
- a `plan` of `starter`, `einvoicing` or `full` (see `organization-plans`)
- a `trial_ends_at` timestamp, set when the status is `trial` and otherwise optional
- `timezone`, `country`, `locale` and `currency`
- optional business info: `legal_name`, `nit`, `address`, `phone`
- optional receipt text: `ticket_footer`, `ticket_legal_notice`
- optional branding: `display_name`, `logo_url`, `primary_color`, `secondary_color`
- `settings`, holding the ordering settings for delivery and takeaway (see `ordering-availability`)
- `created_at` and `updated_at` timestamps

#### Scenario: Laguna after the migration

- **WHEN** the migration has run on a database containing Laguna's organization (`bb0cad3e-be13-4bad-9a37-bba9525bfece`)
- **THEN** Laguna has slug `laguna-escondida`, status `active`, plan `full`, no `trial_ends_at`, timezone `America/Bogota`, country `CO`, locale `es-CO`, currency `COP`
- **AND** its name, id and e-invoicing prefixes are unchanged

#### Scenario: Existing organizations have default ordering settings

- **WHEN** the migration that adds `settings` has run on a database with existing organizations
- **THEN** each of them reads as delivery and takeaway not paused, with empty schedules

### Requirement: Organization fields are validated

Every create and update SHALL validate each field it sets and reject the whole request with a validation error, storing nothing, when any field is invalid:
- `name`: not empty after trimming, at most 255 characters
- `slug`: matches `^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$` (3–40 lowercase letters, digits or hyphens, with no leading or trailing hyphen)
- `status`: `trial`, `active` or `suspended`; `trial` only together with a `trial_ends_at`, sent or already stored
- `plan`: `starter`, `einvoicing` or `full`
- `trial_ends_at`: an RFC 3339 timestamp
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

#### Scenario: Trial without an end

- **WHEN** an organization with no `trial_ends_at` is updated with `status` `trial` and no `trial_ends_at`
- **THEN** the request fails with a validation error and the organization is unchanged

#### Scenario: Unknown plan

- **WHEN** an organization is updated with `plan` `premium`
- **THEN** the request fails with a validation error

### Requirement: The platform operator creates an organization

`POST /api/platform/organizations` SHALL create an organization from a `name`, a `slug` and any of the optional settings.
- Omitted fields default to `active`, plan `full`, `America/Bogota`, `CO`, `es-CO` and `COP`.
- The request MAY include e-invoicing prefixes. When it does, the organization and its prefixes SHALL be stored together or not at all.
- The response SHALL be `201` with the created organization, including its e-invoicing prefixes or `null`.

#### Scenario: Minimal create

- **WHEN** the operator posts `{"name": "Restaurante X S.A.S", "slug": "restaurante-x"}`
- **THEN** the response is `201` with a new id, status `active`, plan `full`, timezone `America/Bogota`, country `CO`, locale `es-CO`, currency `COP`, and `einvoicing: null`

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

### Requirement: The platform operator updates an organization

`PATCH /api/platform/organizations/:id` SHALL change only the fields present in the body and leave the others unchanged.
- An optional field sent as an empty string SHALL be cleared.
- `name`, `status`, `plan`, `timezone`, `country`, `locale` and `currency` SHALL NOT be cleared. An empty value for any of them is a validation error.
- `slug` SHALL NOT be changeable. A body containing `slug` is a validation error.
- This is how the operator activates a paying organization (`status: active`), changes its plan, and extends a trial (a later `trial_ends_at`).
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

#### Scenario: Activate a paying customer

- **WHEN** the operator patches `{"status": "active", "plan": "einvoicing"}` on an organization in `trial`
- **THEN** its status is `active` and its plan `einvoicing`, and it no longer expires

#### Scenario: Extend a trial

- **WHEN** the operator patches a later `trial_ends_at` on an expired trial
- **THEN** the organization is no longer expired until the new date

### Requirement: The signed-in user's organization is returned on `/auth/me`

`GET /api/auth/me` SHALL include an `organization` object for the organization the request runs as, so the frontend can show its branding and formatting and hide what its plan excludes:
- `id`, `slug`, `name`
- `timezone`, `country`, `locale`, `currency`
- `status`, `plan`, and `trial_ends_at` (`null` when unset)
- `legal_name`, `nit`, `address`, `phone`, `ticket_footer`, `ticket_legal_notice`, `display_name`, `logo_url`, `primary_color`, `secondary_color`, each `null` when unset

It SHALL NOT include the e-invoicing prefixes or the timestamps other than `trial_ends_at`. The organization is the one named by the token's `org_id`. The edge returns its own database's row, whose optional settings stay `null` until the sync pull carries them.

#### Scenario: Laguna's user

- **WHEN** a signed-in user of Laguna calls `GET /api/auth/me`
- **THEN** the response has the user's fields as before
- **AND** `organization` has slug `laguna-escondida`, display name `Laguna Escondida`, timezone `America/Bogota`, locale `es-CO`, currency `COP`, status `active` and plan `full`

#### Scenario: A restaurant on trial

- **WHEN** the admin of an organization created by signup 3 days ago on `starter` calls `GET /api/auth/me`
- **THEN** `organization.status` is `trial`, `organization.plan` is `starter`, and `organization.trial_ends_at` is 11 days ahead

#### Scenario: Settings changed by the platform operator

- **WHEN** the operator PATCHes Laguna's `primary_color` to `#0E7C86`
- **THEN** the next `GET /api/auth/me` in the cloud returns `organization.primary_color` `#0E7C86`

#### Scenario: Configured organization missing

- **WHEN** the token's organization does not exist
- **THEN** `GET /api/auth/me` responds `500` without the database error

## REMOVED Requirements

### Requirement: Stored settings do not yet change runtime behavior

**Reason**: Status and plan now gate cloud access, sync and DIAN submission, so the requirement's "suspending an organization does not block its users" no longer holds.

**Migration**: Replaced by "Only ordering settings, status and plan change runtime behavior", which keeps every other guarantee of the old requirement.

## ADDED Requirements

### Requirement: Only ordering settings, status and plan change runtime behavior

Creating, updating or suspending an organization SHALL NOT change the behavior of any existing endpoint, cron, sync or edge feature, with three exceptions:
- the ordering settings, which gate the creation of takeaway and delivery orders (see `delivery-orders`)
- the status and trial end, which stop a suspended or expired organization in the cloud (see `organization-plans`, `organization-sign-in` and `edge-sync-refusal-backoff`)
- the plan, which decides which cloud features the organization reaches and whether its invoices are sent to DIAN (see `organization-plans`)

In particular:
- the running organization still comes from the token or the box's configuration, as today
- business info, receipt text and branding are not yet read anywhere, except to return them on `GET /api/auth/me`
- timezone is read only to return it on `GET /api/auth/me` and to evaluate ordering schedules

#### Scenario: Suspending Laguna locks its users out of the cloud

- **WHEN** Laguna's status is set to `suspended`
- **THEN** Laguna's users are refused in the cloud within a minute, and its box's sync is refused
- **AND** Laguna's box keeps taking orders offline

#### Scenario: Changing the timezone moves the ordering hours

- **WHEN** an organization's delivery schedule has Monday 18:00–22:30 and its timezone is changed from `America/Bogota` to `America/Mexico_City`
- **THEN** delivery opens at 18:00 Mexico City time on Monday
