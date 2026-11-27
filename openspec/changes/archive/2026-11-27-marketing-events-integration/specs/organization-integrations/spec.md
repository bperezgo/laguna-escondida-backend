## MODIFIED Requirements

### Requirement: An organization has at most one integration per provider

An organization SHALL have at most one integration per provider. The providers SHALL be `wompi` and
`marketing_app`. A Wompi integration SHALL hold a private key and an events secret, and SHALL record
its Wompi environment (`sandbox` or `production`). A marketing-app integration SHALL hold one signing
secret and SHALL have no environment. Every integration SHALL record a hint of each secret (its last
4 characters) and when it was created and last updated. Integrations SHALL exist only in the cloud:
the edge box SHALL NOT store them, receive them by sync or serve their routes.

#### Scenario: Setting Wompi twice replaces it

- **WHEN** the operator sets Laguna's Wompi integration, then sets it again with other keys
- **THEN** Laguna has one Wompi integration, holding the second keys

#### Scenario: Both providers side by side

- **WHEN** Laguna has a Wompi integration and its admin generates a marketing-app secret
- **THEN** Laguna has two integrations, and its Wompi keys are unchanged

#### Scenario: Unknown provider

- **WHEN** the operator calls `PUT /api/platform/organizations/:id/integrations/stripe`
- **THEN** the response is `404` and nothing is stored

#### Scenario: No route on an edge box

- **WHEN** an edge box receives `GET /api/platform/organizations/:id/integrations`
- **THEN** the route does not exist there

### Requirement: The platform operator reads an organization's integrations without their secrets

`GET /api/platform/organizations/:id/integrations` SHALL return the organization's integrations,
ordered by provider. `GET /api/platform/organizations/:id/integrations/:provider` SHALL return one, or
`404` when the organization has none for that provider. Each SHALL carry `provider`, `environment`
(null for `marketing_app`), a hint per secret, `created_at` and `updated_at`, and, for Wompi only, the
organization's webhook path (`/api/webhooks/wompi/:organization_id`). No response, log line or error
SHALL contain a secret in plaintext or in ciphertext. The only exception is the response to generating
a marketing-app secret, which carries the new secret once. An unknown organization SHALL be `404`.

#### Scenario: Reading Laguna's integrations

- **WHEN** Laguna has a Wompi and a marketing-app integration and the operator lists its integrations
- **THEN** the list has two items with their hints and timestamps, the Wompi item has its environment and webhook path, and no item has a secret

#### Scenario: None yet

- **WHEN** an organization has no integration
- **THEN** the list is empty and `GET …/integrations/wompi` and `GET …/integrations/marketing_app` are `404`

### Requirement: The organization's admin manages its integrations

`GET /api/organization/integrations`, `GET|PUT|DELETE /api/organization/integrations/:provider` and
`POST /api/organization/integrations/marketing_app/secret` SHALL behave as the platform routes do
(same validation, responses and errors) for the organization of the caller's token. Reads SHALL need
`integrations:read`, and changes, including generating a secret, SHALL need `integrations:manage`.
Both permissions SHALL be held only by the admin role. The routes SHALL NOT exist on an edge box.

#### Scenario: An admin puts Laguna's sandbox keys

- **WHEN** Laguna's admin, on a cloud using Wompi's sandbox, puts `prv_test_…wxyz` and `test_events_…9876`
- **THEN** the response is `200` with `environment: sandbox` and the hints, and Laguna has payment links configured

#### Scenario: An admin generates the marketing-app secret

- **WHEN** Laguna's admin calls `POST /api/organization/integrations/marketing_app/secret`
- **THEN** the response carries the new secret once, and Laguna's events are signed with it from then on

#### Scenario: A manager is refused

- **WHEN** Laguna's manager puts, deletes or generates an integration secret
- **THEN** the response is `403` and nothing changes

#### Scenario: The agent's API key is refused

- **WHEN** a request to `GET /api/organization/integrations` carries Laguna's agent API key
- **THEN** the response is `403` and nothing is returned

### Requirement: Integration secrets are encrypted at rest

Integration secrets SHALL be stored encrypted with AES-256-GCM, under a 32-byte key the cloud is
configured with (base64), never in plaintext. A malformed key SHALL fail startup. Without a key, the
cloud SHALL start, integrations SHALL be neither saved, generated nor read, no organization SHALL have
payment links configured, and no organization SHALL record marketing events. A stored secret that
cannot be decrypted (the key changed) SHALL be treated as absent and logged as an error naming the
organization and provider, never the value.

#### Scenario: The key was replaced

- **WHEN** the cloud starts with a different encryption key than the one Laguna's Wompi integration was saved with
- **THEN** Laguna does not have payment links configured, an error is logged naming Laguna and `wompi`, and putting the keys again restores it

#### Scenario: The key was replaced, marketing-app

- **WHEN** the cloud starts with a different encryption key than the one Laguna's marketing-app secret was saved with
- **THEN** Laguna's agent orders and reservations record no events, an error is logged naming Laguna and `marketing_app`, and generating or putting the secret again restores it

#### Scenario: Malformed key

- **WHEN** the cloud starts with an encryption key that does not decode to 32 bytes
- **THEN** startup fails, naming the setting

## ADDED Requirements

### Requirement: The platform operator sets or generates an organization's marketing-app secret

`POST /api/platform/organizations/:id/integrations/marketing_app/secret` SHALL generate a random
secret of 32 bytes, encoded as 43 URL-safe base64 characters without padding. It SHALL create or
replace the organization's marketing-app integration with it, and SHALL respond `200` with the
integration as it is read plus `signing_secret`, the new secret. It is the only time that secret is
returned.

`PUT /api/platform/organizations/:id/integrations/marketing_app` with `{"signing_secret"}` SHALL create
or replace the integration with the given value, trimmed, and respond `200` with the integration as it
is read (no secret).

Either SHALL be refused, with nothing stored, when:
- the given secret is missing, shorter than 32 characters, or contains whitespace: `400` naming
  `signing_secret` (PUT only)
- the organization does not exist: `404`
- the cloud has no key to encrypt integrations: `503` with code `INTEGRATIONS_UNAVAILABLE`

`POST …/integrations/wompi/secret`, or that path for any other provider, SHALL be `404`.
`DELETE /api/platform/organizations/:id/integrations/marketing_app` SHALL remove the integration and
respond `204`, or `404` when there is none. From then on the organization SHALL record no marketing
events.

#### Scenario: Generating Laguna's secret

- **WHEN** the operator generates Laguna's marketing-app secret
- **THEN** the response is `200` with `provider: marketing_app`, `signing_secret` of 43 characters, and `signing_secret_hint` equal to its last 4
- **AND** a following `GET …/integrations/marketing_app` returns the same hint and no secret

#### Scenario: Generating again rotates

- **WHEN** the operator generates Laguna's secret twice
- **THEN** the two secrets differ, and Laguna's events are signed with the second

#### Scenario: Moving Laguna's existing secret over

- **WHEN** the operator puts Laguna's current 48-character secret
- **THEN** the response is `200` with its hint and no secret, and events are signed with that same value

#### Scenario: A secret too short

- **WHEN** the operator puts a 20-character secret
- **THEN** the response is `400` naming `signing_secret`, and nothing is stored

#### Scenario: Generate for Wompi

- **WHEN** the operator calls `POST …/integrations/wompi/secret`
- **THEN** the response is `404` and Laguna's Wompi integration is unchanged

### Requirement: The Ajustes page generates the marketing-app secret and shows it once

The Integraciones section SHALL have a marketing-app card next to Wompi's. It SHALL show whether a
secret exists, its hint and when it was last updated. A user with `integrations:manage` SHALL be able
to generate it, generate it again and remove it. After generating, the page SHALL show the new secret
once with a copy button and a note to paste it into each of the organization's channels in
marketing-app. Once the user dismisses it or leaves the page, the secret SHALL be gone. Generating
again and removing SHALL each ask for in-page confirmation. Generating again SHALL warn that events
fail until the new secret is pasted on every channel. The card SHALL offer no field to type a
secret, and SHALL be read-only without `integrations:manage`. On an edge box, and for users without
`integrations:read`, it SHALL be hidden like the rest of the page.

#### Scenario: First secret

- **WHEN** Laguna's admin opens Ajustes with no marketing-app secret and chooses Generar
- **THEN** the card shows the full secret with a copy button and the paste-on-every-channel note

#### Scenario: Shown only once

- **WHEN** the admin dismisses the generated secret and reloads the page
- **THEN** the card shows only `••••` and the last 4 characters, and the update time

#### Scenario: Regenerating asks first

- **WHEN** the admin chooses Regenerar
- **THEN** the page warns that events fail until the new secret is pasted on every channel, and generates only after confirmation

#### Scenario: Read-only access

- **WHEN** a user with `integrations:read` but not `integrations:manage` opens Ajustes
- **THEN** the marketing-app card shows its state and offers no action
