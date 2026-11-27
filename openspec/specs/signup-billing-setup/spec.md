# signup-billing-setup Specification

## Purpose
TBD - created by archiving change signup-card-step. Update Purpose after archive.

## Requirements

### Requirement: A pending admin gets a setup session

The cloud SHALL issue a setup token to the admin of a `pending` organization in two places: the `201` of a signup, and the `403 pending_approval` of a sign-in that would otherwise have succeeded (see `self-serve-signup` and `organization-sign-in`).

- The token SHALL expire 60 minutes after it is issued.
- Only the routes under `/api/billing/setup` SHALL accept it. Every other route SHALL refuse it with `401`, as it refuses an invalid token.
- The routes under `/api/billing/setup` SHALL accept nothing else: no sign-in token, API key or platform key.
- Those routes SHALL exist only in the cloud.
- Once the organization is no longer `pending`, those routes SHALL answer `409` with code `not_pending`.

#### Scenario: Setup token on another route

- **WHEN** a setup token is sent to `GET /api/orders`
- **THEN** the response is `401`

#### Scenario: Sign-in token on a setup route

- **WHEN** a trial admin's sign-in token is sent to `GET /api/billing/setup`
- **THEN** the response is `401`

#### Scenario: Expired setup token

- **WHEN** a setup token issued 61 minutes ago is sent to `GET /api/billing/setup`
- **THEN** the response is `401`

#### Scenario: Trial already started

- **WHEN** a valid setup token is used after the operator started the organization's trial
- **THEN** the response is `409` with code `not_pending`

### Requirement: The setup state can be read

`GET /api/billing/setup` SHALL answer `200` with the organization's name, its plan, its billing period (`monthly` until one is chosen), and `card_entry`, which is `true` only when the cloud is configured with its billing keys.

When `card_entry` is `true`, the response SHALL also carry what the browser needs to send a card to Wompi and record the person's consent: Wompi's public key and address, the addresses of Wompi's terms and of its personal-data authorization, and a fresh acceptance token for each. The response SHALL never carry a private key.

#### Scenario: Card entry is on

- **WHEN** the cloud has its billing keys and a pending admin reads the setup state
- **THEN** the response has `card_entry: true`, the public key, the two document addresses and the two acceptance tokens

#### Scenario: Card entry is off

- **WHEN** the cloud has no billing keys
- **THEN** the response has `card_entry: false` and no Wompi fields

#### Scenario: Wompi cannot be reached

- **WHEN** card entry is on and Wompi does not answer
- **THEN** the response is `502` with code `payment_provider_unavailable`

### Requirement: The admin chooses plan and period

`PUT /api/billing/setup/selection` SHALL take `plan` (`starter`, `einvoicing` or `full`) and `period` (`monthly` or `annual`), store both for the organization, and answer `200` with the plan, the period and the price from the price table. An invalid field SHALL answer `400` naming it. The organization SHALL stay `pending`.

#### Scenario: Switching to the annual period

- **WHEN** a pending admin on `einvoicing` sends `{"plan": "einvoicing", "period": "annual"}`
- **THEN** the response is `200` with amount 400000
- **AND** the organization is still `pending`

#### Scenario: Changing the plan

- **WHEN** a pending admin who signed up on `starter` sends `{"plan": "full", "period": "monthly"}`
- **THEN** the organization's plan is `full`

#### Scenario: Unknown period

- **WHEN** the period is `weekly`
- **THEN** the response is `400` with `field: period` and nothing is stored

### Requirement: Saving a card starts the trial

`POST /api/billing/setup/card` SHALL take a Wompi card token and the two acceptance tokens. It SHALL exist only when card entry is on; otherwise the route SHALL answer `404`.

On success the cloud SHALL, in this order:
1. Create a Wompi payment source for the card, with the admin's email. No charge SHALL be made.
2. Store, together or not at all, the payment source's identifier, the card's brand and last four digits, the moment of consent, and the start of the trial (see `organization-plans`, "Starting the trial is one operation").
3. Answer `200` with the same body as a successful sign-in, so the person is signed in.

The cloud SHALL never receive, log or store a card number, security code or expiry date.

Failures SHALL leave the organization `pending` and store no card:
- A missing token answers `400` naming the field.
- A token Wompi refuses, or a payment source that is not available, answers `402` with code `card_refused`.
- Wompi not answering answers `502` with code `payment_provider_unavailable`.

#### Scenario: Card saved

- **WHEN** a pending admin on `einvoicing`, monthly, sends a valid card token and both acceptance tokens
- **THEN** the response is `200` with a sign-in token
- **AND** the organization's status is `trial` with `trial_ends_at` 14 days ahead
- **AND** the stored card shows its brand and last four digits
- **AND** no transaction was created at Wompi

#### Scenario: Card refused

- **WHEN** Wompi refuses the card token
- **THEN** the response is `402` with code `card_refused`
- **AND** the organization is still `pending`

#### Scenario: Card entry is off

- **WHEN** the cloud has no billing keys and `POST /api/billing/setup/card` is called
- **THEN** the response is `404`

#### Scenario: Two saves at once

- **WHEN** the same pending admin sends two card saves at the same moment
- **THEN** one answers `200` and the other `409` with code `not_pending`
- **AND** the trial started once

### Requirement: Card attempts are limited

An organization SHALL be allowed 5 card save attempts per hour, counting refused ones. Past that, `POST /api/billing/setup/card` SHALL answer `429` with a `Retry-After` header and SHALL NOT contact Wompi.

#### Scenario: Card testing

- **WHEN** a sixth card is sent for one organization within an hour
- **THEN** the response is `429` and Wompi receives nothing

### Requirement: The operator still approves by hand

Starting a pending organization's trial from the platform route SHALL work whether or not card entry is on, and whether or not the organization has a saved card. The platform's organization responses SHALL carry the organization's billing period and whether it has a saved card, and SHALL never carry the payment source's identifier.

#### Scenario: Approval without a card

- **WHEN** card entry is off, a pending admin chose `annual`, and the operator starts the trial
- **THEN** the organization is on trial
- **AND** the operator's view of it shows the period `annual` and no saved card
