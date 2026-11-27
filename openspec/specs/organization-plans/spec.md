# organization-plans Specification

## Purpose
Decides what each organization may use in the cloud: its plan sets the features it can reach, a 14-day trial lets it use that plan before paying, and an expired trial or a suspension stops it entirely.

## Requirements

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

An organization with status `trial` SHALL have a `trial_ends_at`. It SHALL be **expired** for as long as its status is still `trial`, from this moment on:
- three days after `trial_ends_at`, when it has a subscription that is not cancelled (see `subscription-charges`), so the first charge and its retries can finish;
- `trial_ends_at` itself, otherwise.

Expiry SHALL be decided from the current time whenever access is checked, with no job needed to change the status.

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

- **WHEN** a manager of an organization without a saved card, whose token was issued on day 13, calls `GET /api/products` on day 15
- **THEN** the response is `403` with code `trial_expired`

#### Scenario: Day 15 with a saved card and a declined charge

- **WHEN** a manager of an organization with a saved card, whose first charge was declined on day 14, calls `GET /api/products` on day 15
- **THEN** the request works

#### Scenario: Day 18 with a saved card and no approved charge

- **WHEN** that manager calls `GET /api/products` on day 18
- **THEN** the response is `403` with code `trial_expired`

#### Scenario: Cancelled trial

- **WHEN** an organization with a saved card cancelled its subscription on day 10 and a manager calls `GET /api/products` on day 15
- **THEN** the response is `403` with code `trial_expired`

#### Scenario: Expired box

- **WHEN** a box of an expired organization pulls
- **THEN** it is refused as a box of a suspended organization is

### Requirement: The operator activates and changes plans

The platform operator SHALL activate an organization by setting its status to `active`, and SHALL change its plan through the platform organization endpoints (see `organization-management`).
- An `active` organization SHALL never expire because of its `trial_ends_at`. One without a paid period SHALL never stop for lack of payment. One with a paid period follows "A paid organization stops when its payment lapses".
- Changing the plan SHALL keep every stored record. Features the new plan excludes are refused, as for any organization on that plan.
- API keys SHALL be kept, and they work again if the plan includes them again.
- Setting a stopped organization that has a paid period back to `active` SHALL clear that paid period and cancel its subscription, so the operator's decision is not undone by the next access check or followed by a charge.

#### Scenario: Customer pays after expiry

- **WHEN** the operator sets an expired organization's status to `active`
- **THEN** its users and box are accepted again within a minute in the cloud (the box resumes after its pause; see `edge-sync-refusal-backoff`)

#### Scenario: Downgrade from full to starter

- **WHEN** the operator changes a `full` organization to `starter`
- **THEN** its reservations and API keys are still stored
- **AND** `GET /api/reservations` and requests with its API keys answer `403` with code `plan_required`

#### Scenario: Operator reactivates a lapsed organization

- **WHEN** the operator sets to `active` an organization that stopped after three failed renewals
- **THEN** it works again within a minute, is never charged, and does not stop again for lack of payment

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

### Requirement: A paid organization stops when its payment lapses

An `active` organization with a paid period SHALL be **unpaid** from this moment on:
- three days after the period's end, when its subscription is not cancelled;
- the period's end itself, when it is cancelled.

It SHALL be decided from the current time whenever access is checked, with no job needed to change the status.

An unpaid organization SHALL be stopped in the cloud exactly as an expired trial is, with code `payment_required` in place of `trial_expired`. Its stored records, plan, API keys and saved card SHALL be kept. An approved charge SHALL make it work again within a minute.

#### Scenario: Renewal declined, within grace

- **WHEN** an organization paid until 15 January has its renewal declined and a manager calls `GET /api/products` on 17 January
- **THEN** the request works

#### Scenario: Grace over

- **WHEN** that manager calls `GET /api/products` on 19 January with no approved charge
- **THEN** the response is `403` with code `payment_required`

#### Scenario: Cancelled subscription

- **WHEN** an organization paid until 15 January cancelled on 3 January and a manager calls `GET /api/products` on 16 January
- **THEN** the response is `403` with code `payment_required`

#### Scenario: Paid after stopping

- **WHEN** an unpaid organization's recovery charge is approved
- **THEN** its users and boxes are accepted again within a minute
