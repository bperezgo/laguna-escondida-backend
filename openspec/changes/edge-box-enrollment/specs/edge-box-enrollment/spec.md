## Purpose

Lets an organization's admin register the restaurant's edge box with the cloud and gives that box its own revocable credential, so a box can sync only for its own organization and no secret is shared between restaurants.

## ADDED Requirements

### Requirement: An admin enrolls a box

In the cloud, `POST /api/edge/enroll` SHALL register an edge box for the organization of the signed-in user. It SHALL require a user token whose user holds the `edge:enroll` permission, which only the admin role grants. The body SHALL carry a `name` (1 to 255 characters after trimming) and MAY carry `replace` (boolean, default `false`). On success it SHALL respond `201 Created` with `node_id`, `organization_id`, `organization_name`, `node_key` and `cloud_sync_url`. The organization SHALL always be the caller's own; the body SHALL NOT be able to name another. The route SHALL NOT exist on an edge box.

#### Scenario: First box of an organization

- **WHEN** an admin of organization A, which has no active box, posts `{"name": "Caja principal"}`
- **THEN** the response is `201` with a new `node_id`, A's id and name, and a `node_key`

#### Scenario: Not an admin

- **WHEN** a signed-in manager posts a valid enrollment
- **THEN** the response is `403 Forbidden` and no box is registered

#### Scenario: Not signed in

- **WHEN** an enrollment arrives without a valid user token
- **THEN** the response is `401 Unauthorized` and no box is registered

#### Scenario: Missing name

- **WHEN** an admin posts an enrollment with an empty `name`
- **THEN** the response is `400 Bad Request` and no box is registered

#### Scenario: Called on an edge box

- **WHEN** `POST /api/edge/enroll` is sent to an edge box
- **THEN** the response is `404 Not Found`

### Requirement: The cloud tells a box where to sync

The cloud SHALL take its public address from the setting `PUBLIC_API_URL` and SHALL return it, unchanged apart from removing a trailing slash, as `cloud_sync_url` in every successful enrollment response. The value SHALL be an absolute `http` or `https` URL with a host and no path, query or fragment. The cloud binary SHALL refuse to boot, before opening the database or serving requests, when the setting is missing or not such a URL. The edge binary SHALL NOT read the setting. A box SHALL be able to sync by using `cloud_sync_url` as its cloud address without altering it.

#### Scenario: Address returned at enrollment

- **WHEN** the cloud runs with `PUBLIC_API_URL` set to `https://api.example.com` and an enrollment succeeds
- **THEN** the response's `cloud_sync_url` is `https://api.example.com`

#### Scenario: Trailing slash

- **WHEN** the cloud runs with `PUBLIC_API_URL` set to `https://api.example.com/`
- **THEN** `cloud_sync_url` is `https://api.example.com`

#### Scenario: Setting changed in the deployment

- **WHEN** the cloud is redeployed with a different `PUBLIC_API_URL` and a box enrolls afterwards
- **THEN** that box receives the new address
- **AND** boxes enrolled earlier keep the address they received

#### Scenario: Cloud without the setting

- **WHEN** the cloud binary boots with `PUBLIC_API_URL` unset
- **THEN** it exits with an error naming the setting

#### Scenario: Not a usable address

- **WHEN** the cloud binary boots with `PUBLIC_API_URL` set to `api.example.com` or to `https://api.example.com/api`
- **THEN** it exits with an error naming the setting

#### Scenario: Edge ignores the setting

- **WHEN** the edge binary boots with `PUBLIC_API_URL` unset, or set to an invalid value
- **THEN** it boots in edge mode

### Requirement: The node key is shown once and stored only as a hash

A node key SHALL start with `lgn_` followed by at least 256 bits of random data. The cloud SHALL return the plain key only in the enrollment response and SHALL store only a one-way hash of it plus its first characters for recognition. No endpoint SHALL return a plain node key afterwards, and a node key SHALL NOT appear in logs or traces.

#### Scenario: Key returned at enrollment

- **WHEN** an enrollment succeeds
- **THEN** `node_key` starts with `lgn_`
- **AND** the stored record of the box does not contain that value

#### Scenario: Two enrollments

- **WHEN** two organizations each enroll a box
- **THEN** the two node keys differ

### Requirement: One active box per organization

An organization SHALL have at most one active edge box, where active means enrolled and not revoked. An enrollment without `replace` for an organization that has an active box SHALL be refused with `409 Conflict`, a `code` of `box_already_enrolled`, and an `active_box` object carrying that box's `name`, `enrolled_at` and `last_seen_at` (`null` when the box has never called). Two enrollments racing for the same organization SHALL NOT both succeed.

#### Scenario: Second box without replace

- **WHEN** organization A has an active box named "Caja principal" and its admin posts `{"name": "PC nuevo"}`
- **THEN** the response is `409` with `code` `box_already_enrolled` and `active_box.name` `"Caja principal"`
- **AND** the first box keeps syncing

#### Scenario: Concurrent enrollments

- **WHEN** two enrollments for organization A, which has no active box, arrive at the same time
- **THEN** exactly one responds `201` and the other responds `409`

### Requirement: Replacing a box revokes the old one

An enrollment with `replace` set to `true` for an organization that has an active box SHALL revoke that box and register the new one in one step: either both happen or neither. The new box SHALL receive a new `node_id` and a new key. A revoked box's record SHALL be kept, never deleted. `replace` set to `true` when no box is active SHALL behave as a first enrollment.

#### Scenario: Replace a dead PC

- **WHEN** organization A has an active box and its admin posts `{"name": "PC nuevo", "replace": true}`
- **THEN** the response is `201` with a `node_id` different from the old box's
- **AND** the old box's next sync request is refused

#### Scenario: Replace with nothing to replace

- **WHEN** organization A has no active box and its admin posts `{"name": "Caja", "replace": true}`
- **THEN** the response is `201`

### Requirement: A box registered before node keys keeps its node id

A box that the cloud registered before this capability existed has no key of its own. An enrollment for an organization whose only active box has never had a key SHALL give that box a key and SHALL return its existing `node_id`, with or without `replace`, updating its name to the one sent. Until then that box SHALL be refused like any box without a valid key.

#### Scenario: Laguna's existing box enrolls

- **WHEN** Laguna's admin enrolls and Laguna's box is registered with no key
- **THEN** the response is `201` and `node_id` equals the box's existing node id

#### Scenario: Existing box before it enrolls

- **WHEN** a box registered with no key sends a sync request with the old shared key
- **THEN** the request is refused with `401 Unauthorized`

### Requirement: The cloud accepts a box only with its own key

Every sync and telemetry request from a box SHALL present `X-Node-ID` and `X-Node-Key`. The cloud SHALL accept it only when the key is the one issued to that node, the node is not revoked, and the node's organization is active; it SHALL then act for that node's organization. A missing key, a wrong key, an unknown node id and a key issued to a different node SHALL all be refused with `401 Unauthorized` and the same body, so the response does not reveal which node ids exist. The cloud SHALL NOT accept any key shared between boxes.

#### Scenario: Box with its own key

- **WHEN** organization A's box pulls with its own node id and key
- **THEN** the pull succeeds and contains only A's data

#### Scenario: Key of another box

- **WHEN** a request presents organization A's node key with organization B's node id
- **THEN** the response is `401 Unauthorized` and nothing is read or applied for B

#### Scenario: Unknown node and wrong key look the same

- **WHEN** one request names an unregistered node id and another names a registered node with a wrong key
- **THEN** both receive `401 Unauthorized` with identical bodies

#### Scenario: Telemetry uses the same key

- **WHEN** a box's telemetry sidecar sends logs with the box's node id and key
- **THEN** the ingest accepts them

### Requirement: Revoked boxes and suspended organizations are refused with a reason

A request that presents the correct key of a revoked node SHALL be refused with `401 Unauthorized` and a `code` of `node_revoked`. A request that presents the correct key of an active node whose organization is `suspended` SHALL be refused with `403 Forbidden` and a `code` of `organization_suspended`. Neither reason SHALL be given to a caller that does not present the node's correct key. Nothing SHALL be read, applied or forwarded for a refused request.

#### Scenario: Replaced box pushes

- **WHEN** a box that was replaced pushes with its old key
- **THEN** the response is `401` with `code` `node_revoked` and no operation is applied

#### Scenario: Suspended organization

- **WHEN** organization A is suspended and its box pulls with its valid key
- **THEN** the response is `403` with `code` `organization_suspended`

#### Scenario: Organization reactivated

- **WHEN** organization A is set back to `active` and its box pulls again with the same key
- **THEN** the pull succeeds

#### Scenario: Wrong key against a revoked node

- **WHEN** a request names a revoked node with a wrong key
- **THEN** the response is the plain `401` with no `code`

### Requirement: The cloud records when a box last called

The cloud SHALL record the time of a box's most recent accepted request as its `last_seen_at`, accurate to within one minute. Refused requests SHALL NOT change it.

#### Scenario: Box syncs

- **WHEN** a box's sync request is accepted at 10:00
- **THEN** its `last_seen_at`, as reported in a later `409` enrollment response, is no earlier than 09:59

#### Scenario: Refused request

- **WHEN** a revoked box keeps calling
- **THEN** its `last_seen_at` does not advance

### Requirement: A refused box says why

On an edge box, `GET /api/edge/status` SHALL include `sync_blocked`: `"replaced"` after the cloud last refused the box with `node_revoked`, `"suspended"` after it last refused it with `organization_suspended`, and `null` otherwise. It SHALL return to `null` after the next accepted sync. In the cloud the field SHALL be `null`. The POS SHALL show an admin or manager a message naming the reason while `sync_blocked` is not `null`. A blocked box SHALL keep serving the restaurant locally.

#### Scenario: Box learns it was replaced

- **WHEN** the cloud refuses a box's sync with `node_revoked`
- **THEN** the box's status reports `sync_blocked` `"replaced"` and `online` `false`
- **AND** the POS shows that this box was replaced by another

#### Scenario: Suspension lifted

- **WHEN** a box reporting `sync_blocked` `"suspended"` completes a sync
- **THEN** its status reports `sync_blocked` `null`

#### Scenario: Blocked box keeps selling

- **WHEN** a box reports `sync_blocked` `"replaced"` and a server creates an order on it
- **THEN** the order is created on the box as usual

#### Scenario: Plain network failure

- **WHEN** a box cannot reach the cloud
- **THEN** its status reports `online` `false` and `sync_blocked` `null`
