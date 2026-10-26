# org-api-keys Specification

## Purpose
Lets a system outside the POS, such as the WhatsApp agent, hold a long-lived and revocable credential for one organization's agent user, so that it can act for that organization without holding a password.

## Requirements

### Requirement: A platform operator issues an API key for an organization's agent user

`POST /api/platform/organizations/:id/api-keys`, authenticated with the admin API key, SHALL take a `user_id` and a `name`, and SHALL create an API key for that user in that organization. The response SHALL contain the key's plaintext exactly once. No later response SHALL contain it. The plaintext SHALL start with `lgk_` and carry at least 256 bits of randomness. Only a hash of the key SHALL be stored.

A key SHALL be issued only for an active user of that organization whose only role is `agent`. A user of another organization, an unknown user, or one with any other role SHALL be refused with `400 Bad Request`. An unknown or suspended organization SHALL be refused with `404 Not Found`. The route SHALL exist only in the cloud.

#### Scenario: Issuing Laguna's agent key

- **WHEN** a platform operator issues a key named `whatsapp-agent` for Laguna's user that holds only the `agent` role
- **THEN** the response is `201 Created` with the key id, name, user, creation time and the plaintext `lgk_…`, and the stored record holds no plaintext

#### Scenario: Plaintext is never shown again

- **WHEN** the organization's keys are listed after a key was issued
- **THEN** that key is listed without its plaintext

#### Scenario: A staff user is refused

- **WHEN** a key is requested for a user that holds the `manager` role
- **THEN** the response is `400 Bad Request` and no key is created

#### Scenario: A user of another organization is refused

- **WHEN** a key is requested under organization A for a user of organization B
- **THEN** the response is `400 Bad Request` and no key is created

#### Scenario: No route on an edge box

- **WHEN** the route is called on an edge box
- **THEN** the response is `404 Not Found`

### Requirement: A platform operator lists and revokes an organization's API keys

`GET /api/platform/organizations/:id/api-keys`, authenticated with the admin API key, SHALL list that organization's keys. Each entry SHALL include the id, name, user, the first characters of the key for recognition, the creation time, the last-used time and the revocation time, and SHALL NOT include the plaintext or the hash. `DELETE /api/platform/organizations/:id/api-keys/:keyId` SHALL revoke the key, and revocation SHALL be permanent. Revoking a key that is unknown or belongs to another organization SHALL return `404 Not Found`.

#### Scenario: Revoking a leaked key

- **WHEN** a platform operator revokes an organization's key
- **THEN** the next request bearing that key is refused with `401 Unauthorized`, and the key is listed with its revocation time

#### Scenario: Revoking another organization's key

- **WHEN** a key of organization B is revoked through organization A's path
- **THEN** the response is `404 Not Found` and the key still works

### Requirement: A request bearing an API key acts as the key's user in the key's organization

In the cloud, a protected endpoint SHALL accept `Authorization: Bearer lgk_…` wherever it accepts a user's token. Such a request SHALL act as the key's user in the key's organization, with that user's current roles, exactly as a token of that user would. This includes which orders the user created and the limits on agent-only users. Each successful use SHALL update the key's last-used time, at most once per minute.

#### Scenario: The agent creates an order with its key

- **WHEN** a delivery order is created with Laguna's agent key
- **THEN** the order is created in Laguna, and its `created_by` is the key's user

#### Scenario: The key's limits are the user's limits

- **WHEN** the pay-order endpoint is called with an agent key
- **THEN** the response is `403 Forbidden` and nothing changes

#### Scenario: Another organization's data

- **WHEN** organization A's key requests organization B's order by id
- **THEN** the order is not found, as if it did not exist

### Requirement: An API key stops working with its user or organization

A request bearing a key that is unknown, revoked, belongs to a deactivated or deleted user, or belongs to a suspended organization SHALL be refused with `401 Unauthorized`. The body SHALL be the same in every case, so the response cannot reveal which keys, users or organizations exist. The specific reason SHALL be logged server-side, without the key.

#### Scenario: The user is deactivated

- **WHEN** the agent user is deactivated and its key is then used
- **THEN** the response is `401 Unauthorized`

#### Scenario: The organization is suspended

- **WHEN** an organization is suspended and its key is then used
- **THEN** the response is `401 Unauthorized` with the same body as for an unknown key

#### Scenario: A malformed key

- **WHEN** a request bears `Bearer lgk_` followed by a value that was never issued
- **THEN** the response is `401 Unauthorized` with the same body as for a revoked key

### Requirement: API keys stay in the cloud

API keys SHALL NOT be sent to edge boxes by any sync. An edge box SHALL refuse every `lgk_` bearer with `401 Unauthorized`.

#### Scenario: A key on an edge box

- **WHEN** Laguna's agent key is presented to Laguna's edge box
- **THEN** the response is `401 Unauthorized`

#### Scenario: Nothing to pull

- **WHEN** an edge box pulls from the cloud after keys were issued for its organization
- **THEN** no key, hash or key metadata reaches the box
