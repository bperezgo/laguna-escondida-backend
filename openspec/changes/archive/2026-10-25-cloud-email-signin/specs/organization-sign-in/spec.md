## ADDED Requirements

### Requirement: Users have a globally unique email

A user MAY have an `email`. An email SHALL be unique across all organizations, compared case-insensitively, and SHALL be stored lowercased and trimmed. A user holding any cloud role (admin, manager, accountant, agent) SHALL have an email; a user holding only floor roles (server, cooker) MAY have none. The email SHALL be set when a user is created or updated in the cloud, SHALL be returned with the user, and SHALL NOT be sent to edge boxes.

#### Scenario: Create a manager with an email

- **WHEN** `POST /api/admin/users` is called with `email: "Ana@Example.com"` and the manager role
- **THEN** the user is created with `email: "ana@example.com"`

#### Scenario: Cloud role without an email

- **WHEN** a user is created, or updated to hold a cloud role, without an email
- **THEN** the request is rejected with `400 Bad Request` and nothing is stored

#### Scenario: Floor staff without an email

- **WHEN** a user is created with only the server role and no email
- **THEN** the user is created

#### Scenario: Email taken in another organization

- **WHEN** organization B creates a user with `email: "ana@example.com"` and a user of organization A already has `ANA@example.com`
- **THEN** the request is rejected with `409 Conflict` and nothing is stored

#### Scenario: Email is not synced

- **WHEN** an edge box pulls a user who has an email
- **THEN** the pulled user carries no email

### Requirement: Cloud sign-in uses email

In the cloud, `POST /api/auth/signin` SHALL take `email` and `password`. The organization SHALL be the user's own. The organization SHALL be active.

#### Scenario: Successful sign-in in the cloud

- **WHEN** `POST /api/auth/signin` is called in the cloud with `{"email": "ana@example.com", "password": "<correct>"}` for a manager of Laguna
- **THEN** the response is `200 OK` with a token for that user in Laguna
- **AND** the response body has the same fields as before this change

#### Scenario: Email in a different case

- **WHEN** the same manager signs in with `"email": "ANA@Example.com "`
- **THEN** the response is `200 OK`

#### Scenario: Missing email in the cloud

- **WHEN** `POST /api/auth/signin` is called in the cloud without `email`
- **THEN** the response is `400 Bad Request`

### Requirement: Only cloud roles sign in to the cloud

In the cloud, sign-in SHALL succeed only for a user holding at least one cloud role: admin, manager, accountant or agent. A user holding only floor roles (server, cooker) SHALL be rejected with the opaque `401`.

#### Scenario: Server signs in to the cloud

- **WHEN** a user whose only role is server signs in to the cloud with a correct email and password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Agent service user signs in to the cloud

- **WHEN** a user holding the agent role signs in to the cloud with a correct email and password
- **THEN** the response is `200 OK`

### Requirement: Edge sign-in uses the box's organization

On an edge box, `POST /api/auth/signin` SHALL take `username` and `password` and SHALL sign the user in to the box's organization. Any role MAY sign in on the box.

#### Scenario: Successful sign-in on the edge

- **WHEN** `POST /api/auth/signin` is called on an edge box with `{"username": "cajero1", "password": "<correct>"}`
- **THEN** the user is signed in to the box's organization

#### Scenario: Missing username on the edge

- **WHEN** `POST /api/auth/signin` is called on an edge box without `username`
- **THEN** the response is `400 Bad Request`

### Requirement: Sign-in context is public

`GET /api/auth/context` SHALL require no authentication. In the cloud it SHALL return `{"mode": "cloud"}`. On an edge box it SHALL return `{"mode": "edge", "organization": {"name": "<the box's organization name>"}}`. It SHALL NOT return any other organization or user data.

#### Scenario: Context in the cloud

- **WHEN** `GET /api/auth/context` is called in the cloud without a token
- **THEN** the response is `200 OK` with `{"mode": "cloud"}`

#### Scenario: Context on an edge box

- **WHEN** `GET /api/auth/context` is called on Laguna's edge box without a token
- **THEN** the response is `200 OK` with `{"mode": "edge", "organization": {"name": "Laguna Escondida"}}`

### Requirement: Sign-in failures are opaque in each mode

In the cloud, an unknown email, a suspended organization, an inactive user, a user without a cloud role and a wrong password SHALL all return `401 Unauthorized` with `{"error": "Invalid email or password"}`. On an edge box, an unknown username, an inactive user and a wrong password SHALL all return `401 Unauthorized` with `{"error": "Invalid username or password"}`. The response SHALL NOT reveal which emails, usernames or organizations exist. The specific reason SHALL be logged server-side.

#### Scenario: Unknown email

- **WHEN** cloud sign-in uses an email no user has
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Suspended organization

- **WHEN** a user of an organization whose status is `suspended` signs in to the cloud with a correct email and password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Username of another organization

- **WHEN** sign-in on organization A's edge box uses the username and password of a user who exists only in organization B
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid username or password"}`

## MODIFIED Requirements

### Requirement: Every user belongs to exactly one organization

A user SHALL belong to exactly one organization, fixed when the user is created. A username SHALL be unique within its organization; the same username MAY exist in different organizations. After the migration, every existing user SHALL belong to Laguna (`bb0cad3e-be13-4bad-9a37-bba9525bfece`).

#### Scenario: Same username in two organizations

- **WHEN** organization A has a user `cajero1` and a user `cajero1` is created in organization B
- **THEN** the user is created
- **AND** each `cajero1` signs in only on its own organization's edge box, with its own password

#### Scenario: Duplicate username in one organization

- **WHEN** a user `cajero1` is created in an organization that already has a `cajero1`
- **THEN** the request is rejected with `409 Conflict` and nothing is stored

#### Scenario: Existing users after the migration

- **WHEN** the migration has run on a database with existing users
- **THEN** every existing user belongs to Laguna and signs in on Laguna's edge box with their current username and password
- **AND** an existing user with a cloud role signs in to the cloud once an email has been set for them

## REMOVED Requirements

### Requirement: Sign-in names the organization

**Reason**: A person belongs to exactly one organization, so a globally unique email identifies the organization in the cloud, and an edge box already knows its own. Asking for the slug added friction and exposed which slugs exist.

**Migration**: Cloud clients send `{email, password}` instead of `{organization, username, password}`. Edge clients drop `organization` and send `{username, password}`. Clients call `GET /api/auth/context` to decide which form to show.

### Requirement: Sign-in failures are opaque

**Reason**: Replaced by "Sign-in failures are opaque in each mode". Sign-in no longer names an organization, so "unknown organization" can't happen, and the cloud's opaque body now refers to email.

**Migration**: Clients that match on the `401` body accept `"Invalid email or password"` from the cloud. The edge body is unchanged.
