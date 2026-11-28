## ADDED Requirements

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
