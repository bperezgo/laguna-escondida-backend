## Purpose

Keeps each organization's credentials for third-party providers (Wompi first) in the cloud, encrypted,
set and removed by the platform operator and never returned in plaintext, so a restaurant can be
connected to a provider without changing the cloud's environment.

## ADDED Requirements

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

### Requirement: Integration routes need the admin API key

Every `/api/platform/organizations/:id/integrations` route SHALL require the admin API key, as the
other platform organization routes do. A staff JWT or an organization's API key SHALL be refused.

#### Scenario: An agent key is refused

- **WHEN** a request to `GET /api/platform/organizations/:id/integrations` carries an organization's API key
- **THEN** it is refused and nothing is returned

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
