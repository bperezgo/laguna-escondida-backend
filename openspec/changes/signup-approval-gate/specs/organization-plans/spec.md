## ADDED Requirements

### Requirement: A pending organization is stopped until its trial starts

An organization with status `pending` SHALL be stopped in the cloud exactly as a suspended one is, except for the code it answers with:
- Every request by its users answers `403` with `{"error": "Approval pending", "code": "pending_approval"}`.
- Every request with its API keys answers `401` with the same body as an invalid token, as for a suspended organization (see `org-api-keys`). A pending organization can't be issued a key, so this only guards against one it already holds.
- It SHALL NOT get an enrollment code, and an enrollment code or invitation of it SHALL be refused as for a suspended organization (`organization_inactive`).
- A box sync or telemetry request with its credentials is refused as for a suspended organization.
- Sign-in answers as described in `organization-sign-in`.

A pending organization SHALL have no `trial_ends_at` and SHALL never expire. Its chosen `plan` SHALL be stored and SHALL be the plan its trial starts on. The platform operator SHALL be able to read and change it as usual.

#### Scenario: Pending admin with a token issued elsewhere

- **WHEN** any request reaches the cloud with a valid token whose organization is `pending`
- **THEN** the response is `403` with code `pending_approval`

#### Scenario: Pending organization waits indefinitely

- **WHEN** an organization has been `pending` for 30 days
- **THEN** it is still `pending`, with no `trial_ends_at`, and is refused with `pending_approval`

### Requirement: Starting the trial is one operation

`POST /api/platform/organizations/:id/start-trial` SHALL move an organization from `pending` to `trial`, with `trial_ends_at` 14 days after the moment it runs and the stored plan unchanged, and SHALL answer `200` with the updated organization.
- An organization that is not `pending` SHALL answer `409` with `{"error": "...", "code": "not_pending"}` and SHALL be unchanged.
- An unknown id SHALL answer `404`.
- The route SHALL require the platform admin API key and SHALL exist only in the cloud, like the other platform organization endpoints.

Every way of starting a trial, including a future payment confirmation, SHALL go through this same operation, so its rules hold whoever starts it.

#### Scenario: Operator approves a signup

- **WHEN** the operator calls start-trial on a `pending` organization on plan `einvoicing` at 2026-12-01T10:00:00-05:00
- **THEN** the response is `200` with status `trial`, plan `einvoicing` and `trial_ends_at` 2026-12-15T10:00:00-05:00
- **AND** its admin can sign in within a minute

#### Scenario: Starting twice

- **WHEN** the operator calls start-trial on an organization already in `trial`
- **THEN** the response is `409` with code `not_pending` and its `trial_ends_at` is unchanged

#### Scenario: Active organization

- **WHEN** the operator calls start-trial on an `active` organization
- **THEN** the response is `409` with code `not_pending`

### Requirement: The operator can activate a pending organization without a trial

The platform operator SHALL be able to move a `pending` organization straight to `active` or to `suspended` through the platform update endpoint (see `organization-management`). Nothing else SHALL move an organization into `pending`, and only start-trial SHALL move a `pending` organization into `trial`.

#### Scenario: Paying customer from day one

- **WHEN** the operator patches `{"status": "active"}` on a `pending` organization
- **THEN** its status is `active`, it never expires, and its users can sign in within a minute

#### Scenario: Rejecting a signup

- **WHEN** the operator patches `{"status": "suspended"}` on a `pending` organization
- **THEN** its status is `suspended` and its users are refused as for any suspended organization
