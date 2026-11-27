## MODIFIED Requirements

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

## ADDED Requirements

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
