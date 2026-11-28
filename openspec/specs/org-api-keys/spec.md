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

### Requirement: An organization admin issues an API key for their own organization

`POST /api/organization/api-keys`, authenticated with a user's token, SHALL take a `name` and SHALL act on the token's organization only. In one atomic operation it SHALL create a new active user of that organization whose only role is `agent`, dedicated to this key, and an API key for that user. The response SHALL be `201 Created` with the key's id, name, user, prefix, creation time and the plaintext `lgk_…`, exactly once. If either the user or the key cannot be created, neither SHALL exist afterwards.

Only a user holding the admin role SHALL be allowed. Any other role SHALL be refused with `403 Forbidden`, and an organization API key used as the bearer SHALL be refused too. A blank name or one longer than 255 characters SHALL be refused with `400 Bad Request`, and nothing SHALL be created. A suspended organization SHALL be refused. The route SHALL exist only in the cloud.

#### Scenario: Admin issues a key for a console

- **WHEN** Laguna's admin issues a key named `console-local`
- **THEN** the response is `201 Created` with the plaintext `lgk_…`, a new agent-only user of Laguna exists for that key, and the stored record holds no plaintext

#### Scenario: Each key gets its own user

- **WHEN** the admin issues two keys
- **THEN** each key belongs to a different user

#### Scenario: A manager is refused

- **WHEN** a user holding the `manager` role issues a key
- **THEN** the response is `403 Forbidden` and no user or key is created

#### Scenario: An API key cannot issue keys

- **WHEN** the issue route is called with an `lgk_` bearer
- **THEN** the request is refused and no user or key is created

#### Scenario: Blank name

- **WHEN** the admin issues a key with a blank name
- **THEN** the response is `400 Bad Request` and no user or key is created

#### Scenario: No route on an edge box

- **WHEN** the route is called on an edge box
- **THEN** the response is `404 Not Found`

### Requirement: An organization admin lists and revokes their own organization's API keys

`GET /api/organization/api-keys` SHALL list every key of the token's organization, newest first, including revoked keys and keys issued by the platform operator. Each entry SHALL carry the same fields as the operator listing, without the plaintext or the hash. `DELETE /api/organization/api-keys/:keyId` SHALL revoke the key permanently. When the key's user was created for that key, revoking SHALL also deactivate that user. When it was not, only the key SHALL be revoked. Revoking a key that is unknown or belongs to another organization SHALL return `404 Not Found` and change nothing. Only the admin role SHALL be allowed, as for issuing.

#### Scenario: Revoking a key issued from Ajustes

- **WHEN** the admin revokes a key issued from the organization routes
- **THEN** the next request with that key is refused with `401 Unauthorized`, the key is listed with its revocation time, and its user is inactive

#### Scenario: Rotating a key

- **WHEN** the admin issues a new key, then revokes the old one
- **THEN** the new key keeps working and the old one is refused with `401 Unauthorized`

#### Scenario: Revoking an operator-issued key

- **WHEN** the admin revokes a key that the platform operator issued for an existing agent user
- **THEN** the key is revoked, the user stays active, and that user's other keys keep working

#### Scenario: Another organization's key

- **WHEN** the admin of organization A revokes a key of organization B
- **THEN** the response is `404 Not Found` and the key still works

### Requirement: A user created for an API key is not managed as a staff user

A user created for a key SHALL NOT appear in the users list. Reading, updating, deleting or resetting the password of that user through the user management endpoints SHALL respond as if the user did not exist (`404 Not Found`). That user SHALL NOT be able to sign in, and SHALL NOT be sent to edge boxes by any sync. Orders created with its key SHALL still record it as `created_by`. When its key is revoked through either the organization route or the platform operator route, the user SHALL be deactivated.

#### Scenario: Operator revokes a key issued from Ajustes

- **WHEN** the platform operator revokes a key that an admin issued from Ajustes
- **THEN** the key is revoked and its user is inactive

#### Scenario: Hidden from the users list

- **WHEN** an admin lists users after issuing a key from Ajustes
- **THEN** the key's user is not in the list

#### Scenario: Cannot be edited

- **WHEN** an admin tries to update the key's user by id, for example to add a role
- **THEN** the response is `404 Not Found` and the user is unchanged

#### Scenario: Cannot sign in

- **WHEN** a sign-in is attempted for the key's user
- **THEN** the sign-in is refused like any invalid credential

#### Scenario: Not synced to the edge

- **WHEN** an edge box pulls users after a key was issued from Ajustes
- **THEN** the key's user is not among the users it receives

### Requirement: The admin manages keys from Ajustes

The integrations settings page SHALL show the organization's API keys to an admin, with name, prefix, creation time, last-used time and revocation state, an action to issue a key by name, and an action to revoke each active key after a confirmation. After issuing, the page SHALL show the plaintext once with a copy action and a notice that it will not be shown again. When an MCP URL is configured, the page SHALL show it next to the new key. Users without the admin role SHALL NOT see the key management section.

#### Scenario: Issuing from the page

- **WHEN** the admin issues a key named `console-local` from the page
- **THEN** the page shows the plaintext with a copy action and the configured MCP URL, and after the dialog is closed the key appears in the list without its plaintext

#### Scenario: No MCP URL configured

- **WHEN** no MCP URL is configured and the admin issues a key
- **THEN** the page shows the key without an MCP URL

#### Scenario: Revoking from the page

- **WHEN** the admin confirms revoking a key
- **THEN** the key is shown as revoked and offers no revoke action

#### Scenario: Manager does not see keys

- **WHEN** a manager opens the integrations settings page
- **THEN** no API key section is shown
