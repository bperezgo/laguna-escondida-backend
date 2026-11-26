## Purpose

Decides what each organization may use in the cloud: its plan sets the features it can reach, a 14-day trial lets it use that plan before paying, and an expired trial or a suspension stops it entirely.

## ADDED Requirements

### Requirement: Each plan includes a fixed set of features

An organization's `plan` SHALL be `starter`, `einvoicing` or `full`.

| Feature | Routes | starter | einvoicing | full |
|---|---|---|---|---|
| Core: orders, kitchen, products, stock, suppliers, purchases, expenses, bill owners, users, reports, financial summary, organization settings, box enrollment | every route not listed below | yes | yes | yes |
| E-invoicing | `/api/invoices*`, `/api/support-documents*` | no | yes | yes |
| Customer channels | `/api/reservations*`, `/api/sse/reservations`, `/api/contacts*`, `/api/feedback*`, `/api/delivery-zones*`, `/api/deliveries*`, `/api/notification-channels*`, `/api/notification-rules*` | no | no | yes |
| Payments and integrations | `/api/organization/integrations*`, `/api/organization/payment-settings*` | no | no | yes |
| Agents and MCP | `/api/organization/api-keys*`, any request authenticated with an organization API key, any request by a user holding the agent role | no | no | yes |

A request to a feature its organization's plan excludes SHALL answer `403` with `{"error": "...", "code": "plan_required", "required_plan": "<the cheapest plan that includes it>"}`, after authentication and before permission checks.

#### Scenario: Starter reaches invoices

- **WHEN** a manager of a `starter` organization calls `GET /api/invoices`
- **THEN** the response is `403` with code `plan_required` and `required_plan` `einvoicing`

#### Scenario: E-invoicing plan reaches reservations

- **WHEN** a manager of an `einvoicing` organization calls `GET /api/reservations`
- **THEN** the response is `403` with code `plan_required` and `required_plan` `full`

#### Scenario: API key on a non-full plan

- **WHEN** a request with a valid organization API key reaches any route for an `einvoicing` organization
- **THEN** the response is `403` with code `plan_required` and `required_plan` `full`

#### Scenario: Core features on every plan

- **WHEN** a manager of a `starter` organization creates a product
- **THEN** the product is created as today

#### Scenario: Full plan

- **WHEN** a manager of a `full` organization calls any route their permissions allow
- **THEN** no plan check refuses it

### Requirement: Bills of a plan without e-invoicing are not sent to DIAN

When the cloud receives a pending invoice from a box, and the organization's plan does not include e-invoicing at that moment, the cloud SHALL NOT submit it to DIAN. It SHALL mark it `skipped`, and SHALL never submit it later, even after an upgrade. Bills themselves SHALL sync as usual.

#### Scenario: Starter box pays a bill

- **WHEN** a box of a `starter` organization pushes a paid bill and its pending invoice
- **THEN** the bill is stored in the cloud
- **AND** the pending invoice is marked `skipped` and is never submitted

#### Scenario: Upgrade does not submit old bills

- **WHEN** that organization later moves to `einvoicing`
- **THEN** the `skipped` invoices stay `skipped`
- **AND** pending invoices received after the change are submitted as usual

### Requirement: A trial lasts 14 days, then stops the organization

An organization with status `trial` SHALL have a `trial_ends_at`. It SHALL be **expired** from `trial_ends_at` on, for as long as its status is still `trial`. Expiry SHALL be decided from the current time whenever access is checked, with no job needed to change the status.

An expired organization SHALL be stopped in the cloud exactly as a suspended one is, except for the codes it answers with:
- Its boxes' sync and telemetry are refused as for a suspended organization (`organization_suspended`, see `edge-sync-refusal-backoff`).
- Every request by its users, with a token issued before or after expiry, answers `403` with code `trial_expired`.
- Every request with its API keys answers `403` with code `trial_expired`.
- Sign-in answers as described in `organization-sign-in`.

The box keeps working offline as today. The platform operator SHALL be able to read and change the organization as usual.

#### Scenario: Day 13

- **WHEN** an organization created 13 days ago on `full` uses any feature
- **THEN** it works as on an active `full` organization

#### Scenario: Day 15 with an existing token

- **WHEN** a manager whose token was issued on day 13 calls `GET /api/products` on day 15
- **THEN** the response is `403` with code `trial_expired`

#### Scenario: Expired box

- **WHEN** a box of an expired organization pulls
- **THEN** it is refused as a box of a suspended organization is

### Requirement: The operator activates and changes plans

The platform operator SHALL activate an organization by setting its status to `active`, and SHALL change its plan through the platform organization endpoints (see `organization-management`).
- An `active` organization SHALL never expire, whatever its `trial_ends_at`.
- Changing the plan SHALL keep every stored record. Features the new plan excludes are refused, as for any organization on that plan.
- API keys SHALL be kept, and they work again if the plan includes them again.

#### Scenario: Customer pays after expiry

- **WHEN** the operator sets an expired organization's status to `active`
- **THEN** its users and box are accepted again within a minute in the cloud (the box resumes after its pause; see `edge-sync-refusal-backoff`)

#### Scenario: Downgrade from full to starter

- **WHEN** the operator changes a `full` organization to `starter`
- **THEN** its reservations and API keys are still stored
- **AND** `GET /api/reservations` and requests with its API keys answer `403` with code `plan_required`

### Requirement: Access changes take effect within a minute

A change to an organization's status, plan or trial end SHALL affect requests by its users and API keys within 60 seconds on every cloud replica.

#### Scenario: Operator suspends an organization

- **WHEN** the operator suspends an organization whose admin is signed in
- **THEN** the admin's requests are refused with `403` no later than 60 seconds afterwards

### Requirement: Existing organizations keep everything

The migration SHALL give every existing organization the plan `full`. An existing organization's status (`active` or `suspended`) SHALL be unchanged, and it SHALL have no `trial_ends_at`.

#### Scenario: Laguna after the migration

- **WHEN** the migration has run
- **THEN** Laguna is `active` on plan `full`, and every route it used before still works

### Requirement: Plans are enforced only in the cloud

An edge box SHALL NOT check plans or trials. Its POS SHALL work the same on every plan. What the cloud does with its synced data follows the organization's plan.

#### Scenario: Starter box

- **WHEN** a box of a `starter` organization takes and pays an order
- **THEN** it does so exactly as a `full` organization's box does
