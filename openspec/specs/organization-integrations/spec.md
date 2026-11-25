# organization-integrations Specification

## Purpose
Keeps each organization's credentials for third-party providers (Wompi first) in the cloud, encrypted,
set and removed by the platform operator and never returned in plaintext, so a restaurant can be
connected to a provider without changing the cloud's environment.

## Requirements

### Requirement: An organization has at most one integration per provider

An organization SHALL have at most one integration per provider. The only provider SHALL be `wompi`.
A Wompi integration SHALL hold a private key and an events secret, and SHALL record its Wompi
environment (`sandbox` or `production`), a hint of each secret (its last 4 characters) and when it
was created and last updated. Integrations SHALL exist only in the cloud: the edge box SHALL NOT
store them, receive them by sync or serve their routes.

#### Scenario: Setting Wompi twice replaces it

- **WHEN** the operator sets Laguna's Wompi integration, then sets it again with other keys
- **THEN** Laguna has one Wompi integration, holding the second keys

#### Scenario: Unknown provider

- **WHEN** the operator calls `PUT /api/platform/organizations/:id/integrations/stripe`
- **THEN** the response is `404` and nothing is stored

#### Scenario: No route on an edge box

- **WHEN** an edge box receives `GET /api/platform/organizations/:id/integrations`
- **THEN** the route does not exist there

### Requirement: The platform operator sets an organization's Wompi integration

`PUT /api/platform/organizations/:id/integrations/wompi` with `{"private_key", "events_secret"}`
SHALL create or replace the organization's Wompi integration and respond `200` with the integration
as it is read (see the read requirement). It SHALL be refused, with nothing stored, when:
- either value is missing or empty after trimming: `400` naming the field
- the private key does not start with `prv_test_` or `prv_prod_`, or the events secret does not start
  with `test_events_` or `prod_events_`: `400` naming the field
- the two values are for different environments (one `test`, the other `prod`): `400`
- the cloud's Wompi API is Wompi's sandbox and the keys are `prod`, or it is Wompi's production
  API and the keys are `test`: `400`. A Wompi API URL that is neither (a local stand-in) accepts
  either environment
- the organization does not exist: `404`
- the cloud has no key to encrypt integrations: `503` with code `INTEGRATIONS_UNAVAILABLE`

The environment SHALL be `sandbox` for `test` keys and `production` for `prod` keys.

#### Scenario: Laguna's sandbox keys

- **WHEN** the cloud uses Wompi's sandbox and the operator puts `{"private_key": "prv_test_abc…wxyz", "events_secret": "test_events_…9876"}` for Laguna
- **THEN** the response is `200` with `provider: wompi`, `environment: sandbox`, `private_key_hint: wxyz`, `events_secret_hint: 9876`
- **AND** Laguna has payment links configured

#### Scenario: Production keys on the sandbox

- **WHEN** the cloud uses Wompi's sandbox and the operator puts `prv_prod_…` and `prod_events_…`
- **THEN** the response is `400` saying the keys are for production and the cloud uses the sandbox
- **AND** nothing is stored

#### Scenario: Mixed environments

- **WHEN** the operator puts a `prv_test_…` private key with a `prod_events_…` events secret
- **THEN** the response is `400` and nothing is stored

#### Scenario: A public key by mistake

- **WHEN** the operator puts `pub_test_…` as the private key
- **THEN** the response is `400` naming `private_key`

#### Scenario: No encryption key

- **WHEN** the cloud runs without an integrations encryption key and the operator puts valid keys
- **THEN** the response is `503` with code `INTEGRATIONS_UNAVAILABLE` and nothing is stored

### Requirement: The platform operator reads an organization's integrations without their secrets

`GET /api/platform/organizations/:id/integrations` SHALL return the organization's integrations,
ordered by provider. `GET /api/platform/organizations/:id/integrations/wompi` SHALL return one, or
`404` when the organization has none. Each SHALL carry `provider`, `environment`, a hint per secret,
`created_at` and `updated_at`, and the organization's webhook path
(`/api/webhooks/wompi/:organization_id`) for Wompi. No response, log line or error SHALL contain a
secret in plaintext or in ciphertext. An unknown organization SHALL be `404`.

#### Scenario: Reading Laguna's integrations

- **WHEN** Laguna has a Wompi integration and the operator lists its integrations
- **THEN** the list has one item with the hints, the environment, the timestamps and the webhook path, and neither secret

#### Scenario: None yet

- **WHEN** an organization has no integration
- **THEN** the list is empty and `GET …/integrations/wompi` is `404`

### Requirement: The platform operator removes an organization's integration

`DELETE /api/platform/organizations/:id/integrations/wompi` SHALL delete the organization's Wompi
integration and respond `204`, or `404` when it has none. From then on the organization SHALL NOT
have payment links configured. Payment links already created SHALL be left as they are.

#### Scenario: Turning Wompi off for a restaurant

- **WHEN** the operator deletes Laguna's Wompi integration
- **THEN** the response is `204`, and Laguna's next agent order is refused with `PAYMENT_LINKS_NOT_CONFIGURED`

### Requirement: Platform integration routes need the admin API key

Every `/api/platform/organizations/:id/integrations` route SHALL require the admin API key, as the
other platform organization routes do. A staff JWT or an organization's API key SHALL be refused on
these routes; staff use the organization routes below.

#### Scenario: An agent key is refused

- **WHEN** a request to `GET /api/platform/organizations/:id/integrations` carries an organization's API key
- **THEN** it is refused and nothing is returned

### Requirement: The organization's admin manages its integrations

`GET /api/organization/integrations` and `GET|PUT|DELETE /api/organization/integrations/wompi` SHALL
behave as the platform routes do (same validation, responses and errors) for the organization of the
caller's token. Reads SHALL need `integrations:read` and changes SHALL need `integrations:manage`.
Both permissions SHALL be held only by the admin role. The routes SHALL NOT exist on an edge box.

#### Scenario: An admin puts Laguna's sandbox keys

- **WHEN** Laguna's admin, on a cloud using Wompi's sandbox, puts `prv_test_…wxyz` and `test_events_…9876`
- **THEN** the response is `200` with `environment: sandbox` and the hints, and Laguna has payment links configured

#### Scenario: A manager is refused

- **WHEN** Laguna's manager puts or deletes the Wompi integration
- **THEN** the response is `403` and nothing changes

#### Scenario: The agent's API key is refused

- **WHEN** a request to `GET /api/organization/integrations` carries Laguna's agent API key
- **THEN** the response is `403` and nothing is returned

### Requirement: The Ajustes page shows and sets the Wompi integration without exposing it

The ops UI SHALL have an Ajustes page whose Integraciones section shows whether Wompi is configured,
its environment, the secret hints, when it was last updated and the webhook path. It SHALL let a
user with `integrations:manage` put, replace and remove the keys, and SHALL be read-only without it.
It SHALL never display a stored secret, SHALL clear the entered values after saving, and SHALL show
a refused save's reason next to the named field (or as the form's message when no field is named).
It SHALL be hidden on an edge box and for users without `integrations:read`.

#### Scenario: Configuring Wompi from scratch

- **WHEN** Laguna's admin opens Ajustes with no Wompi integration and saves sandbox keys
- **THEN** the card shows Sandbox, `••••wxyz`, `••••9876`, the update time and the webhook path, and the inputs are empty

#### Scenario: Production keys on the sandbox

- **WHEN** the admin saves production keys on a cloud using Wompi's sandbox
- **THEN** the form shows that the keys are for production and the cloud uses the sandbox, and the card still shows the previous state

#### Scenario: Removing Wompi

- **WHEN** the admin chooses Quitar and confirms in the page
- **THEN** the card shows Wompi as not configured

#### Scenario: Server without an encryption key

- **WHEN** the cloud runs without an integrations encryption key
- **THEN** the section says integrations are not enabled on this server and offers no form

#### Scenario: Hidden on the edge

- **WHEN** an admin signs in on an edge box
- **THEN** Ajustes is not in the navigation and its page is not shown

### Requirement: Integration secrets are encrypted at rest

Integration secrets SHALL be stored encrypted with AES-256-GCM, under a 32-byte key the cloud is
configured with (base64), never in plaintext. A malformed key SHALL fail startup. Without a key, the
cloud SHALL start, integrations SHALL be neither saved nor read, and no organization SHALL have
payment links configured. A stored secret that cannot be decrypted (the key changed) SHALL be
treated as absent and logged as an error naming the organization and provider, never the value.

#### Scenario: The key was replaced

- **WHEN** the cloud starts with a different encryption key than the one Laguna's Wompi integration was saved with
- **THEN** Laguna does not have payment links configured, an error is logged naming Laguna and `wompi`, and putting the keys again restores it

#### Scenario: Malformed key

- **WHEN** the cloud starts with an encryption key that does not decode to 32 bytes
- **THEN** startup fails, naming the setting
