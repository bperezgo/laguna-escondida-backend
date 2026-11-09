## MODIFIED Requirements

### Requirement: An organization has an identity, a status and settings

An organization SHALL have:
- an id (UUID), a `name`, and a `slug` that is unique across organizations
- a `status` of `active` or `suspended`
- `timezone`, `country`, `locale` and `currency`
- optional business info: `legal_name`, `nit`, `address`, `phone`
- optional receipt text: `ticket_footer`, `ticket_legal_notice`
- optional branding: `display_name`, `logo_url`, `primary_color`, `secondary_color`
- `settings`, holding the ordering settings for delivery and takeaway (see `ordering-availability`)
- `created_at` and `updated_at` timestamps

#### Scenario: Laguna after the migration

- **WHEN** the migration has run on a database containing Laguna's organization (`bb0cad3e-be13-4bad-9a37-bba9525bfece`)
- **THEN** Laguna has slug `laguna-escondida`, status `active`, timezone `America/Bogota`, country `CO`, locale `es-CO`, currency `COP`
- **AND** its name, id and e-invoicing prefixes are unchanged

#### Scenario: Existing organizations have default ordering settings

- **WHEN** the migration that adds `settings` has run on a database with existing organizations
- **THEN** each of them reads as delivery and takeaway not paused, with empty schedules

### Requirement: The platform operator lists and reads organizations

`GET /api/platform/organizations` SHALL return every organization (active and suspended), ordered by name, each with its e-invoicing prefixes or `null` and its `settings`. `GET /api/platform/organizations/:id` SHALL return one organization in the same shape, or `404` when no organization has that id.

#### Scenario: List includes suspended organizations

- **WHEN** one organization is active and another is suspended
- **THEN** the list returns both, ordered by name, each with its status

#### Scenario: Unknown id

- **WHEN** the operator requests an id that does not exist
- **THEN** the response is `404`

#### Scenario: Settings are included

- **WHEN** an organization has paused delivery and the operator reads it
- **THEN** the response's `settings.ordering.delivery.paused` is `true`

### Requirement: Stored settings do not yet change runtime behavior

Creating, updating or suspending an organization SHALL NOT change the behavior of any existing endpoint, cron, sync or edge feature, except for the ordering settings, which gate the creation of takeaway and delivery orders (see `delivery-orders`). In particular:
- the running organization still comes from the deployment's configuration
- suspending an organization does not block its users
- business info, receipt text and branding are not yet read anywhere, except to return them on `GET /api/auth/me`
- timezone is read only to return it on `GET /api/auth/me` and to evaluate ordering schedules

#### Scenario: Suspending Laguna does not lock anyone out

- **WHEN** Laguna's status is set to `suspended`
- **THEN** Laguna's users still sign in and create orders exactly as before

#### Scenario: Changing the timezone moves the ordering hours

- **WHEN** an organization's delivery schedule has Monday 18:00–22:30 and its timezone is changed from `America/Bogota` to `America/Mexico_City`
- **THEN** delivery opens at 18:00 Mexico City time on Monday
