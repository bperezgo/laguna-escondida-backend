## MODIFIED Requirements

### Requirement: An organization has an identity, a status and settings

An organization SHALL have:
- an id (UUID), a `name`, and a `slug` that is unique across organizations
- a `status` of `pending`, `trial`, `active` or `suspended`
- a `plan` of `starter`, `einvoicing` or `full` (see `organization-plans`)
- a `trial_ends_at` timestamp, set when the status is `trial`, unset when it is `pending`, and otherwise optional
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

#### Scenario: Pending organization has no trial end

- **WHEN** an organization is stored with status `pending` and a `trial_ends_at`
- **THEN** the database refuses it

### Requirement: Organization fields are validated

Every create and update SHALL validate each field it sets and reject the whole request with a validation error, storing nothing, when any field is invalid:
- `name`: not empty after trimming, at most 255 characters
- `slug`: matches `^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$` (3–40 lowercase letters, digits or hyphens, with no leading or trailing hyphen)
- `status`: `trial`, `active` or `suspended`; `trial` only together with a `trial_ends_at`, sent or already stored. `pending` SHALL NOT be accepted on a create or an update: only signup creates it.
- `status` on a `pending` organization: only `active` or `suspended`; moving it to `trial` is done by start-trial (see `organization-plans`)
- `trial_ends_at`: an RFC 3339 timestamp, and not accepted while the organization stays `pending`
- `plan`: `starter`, `einvoicing` or `full`
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

#### Scenario: Patching a pending organization into a trial

- **WHEN** the operator patches `{"status": "trial", "trial_ends_at": "2026-12-31T23:59:59-05:00"}` on a `pending` organization
- **THEN** the request fails with a validation error on `status` and the organization stays `pending`

#### Scenario: Setting pending by hand

- **WHEN** the operator patches `{"status": "pending"}` on an `active` organization, or creates one with status `pending`
- **THEN** the request fails with a validation error on `status` and nothing changes

#### Scenario: Changing a pending organization's plan

- **WHEN** the operator patches `{"plan": "full"}` on a `pending` organization
- **THEN** it stays `pending` with plan `full`

### Requirement: The platform operator lists and reads organizations

`GET /api/platform/organizations` SHALL return every organization, whatever its status, ordered by name, each with its e-invoicing prefixes or `null` and its `settings`. An optional `status` query parameter SHALL keep only the organizations with that status; an unknown value SHALL answer `400` naming `status`. `GET /api/platform/organizations/:id` SHALL return one organization in the same shape, or `404` when no organization has that id.

#### Scenario: List includes suspended organizations

- **WHEN** one organization is active and another is suspended
- **THEN** the list returns both, ordered by name, each with its status

#### Scenario: Signups waiting for approval

- **WHEN** two organizations are `pending` and Laguna is `active`, and the operator calls `GET /api/platform/organizations?status=pending`
- **THEN** the list returns the two pending organizations only, each with its plan and `created_at`

#### Scenario: Unknown status filter

- **WHEN** the operator calls `GET /api/platform/organizations?status=waiting`
- **THEN** the response is `400` with `field` `status`

#### Scenario: Unknown id

- **WHEN** the operator requests an id that does not exist
- **THEN** the response is `404`

#### Scenario: Settings are included

- **WHEN** an organization has paused delivery and the operator reads it
- **THEN** the response's `settings.ordering.delivery.paused` is `true`
