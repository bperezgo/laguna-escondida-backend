## MODIFIED Requirements

### Requirement: Users have a globally unique email

A user MAY have an `email`. An email SHALL be unique across all organizations, compared case-insensitively, and SHALL be stored lowercased and trimmed. The email SHALL be set when a user is created or updated in the cloud, SHALL be returned with the user, and SHALL NOT be sent to edge boxes.
- A user holding the admin, accountant, agent or courier role SHALL have an email.
- A manager MAY have none, which makes them a box-only manager, like the box user created at signup.
- A user holding only floor roles (server, cooker) MAY have none.
- A user without an email SHALL NOT sign in to the cloud.

#### Scenario: Create a manager with an email

- **WHEN** `POST /api/admin/users` is called with `email: "Ana@Example.com"` and the manager role
- **THEN** the user is created with `email: "ana@example.com"`

#### Scenario: Cloud role without an email

- **WHEN** a user is created, or updated to hold the admin, accountant, agent or courier role, without an email
- **THEN** the request is rejected with `400 Bad Request` and nothing is stored

#### Scenario: Manager without an email

- **WHEN** a user is created with only the manager role and no email
- **THEN** the user is created
- **AND** it signs in on the organization's box with its username and password
- **AND** it cannot sign in to the cloud

#### Scenario: Floor staff without an email

- **WHEN** a user is created with only the server role and no email
- **THEN** the user is created

#### Scenario: Email taken in another organization

- **WHEN** organization B creates a user with `email: "ana@example.com"` and a user of organization A already has `ANA@example.com`
- **THEN** the request is rejected with `409 Conflict` and nothing is stored

#### Scenario: Email is not synced

- **WHEN** an edge box pulls a user who has an email
- **THEN** the pulled user carries no email

#### Scenario: Courier without an email

- **WHEN** a user is created with only the courier role and no email
- **THEN** the request is rejected with `400 Bad Request` and nothing is stored

### Requirement: Only cloud roles sign in to the cloud

In the cloud, sign-in SHALL succeed only for a user holding at least one cloud role: admin, manager, accountant, agent or courier. A user holding only floor roles (server, cooker) SHALL be rejected with the opaque `401`.

#### Scenario: Server signs in to the cloud

- **WHEN** a user whose only role is server signs in to the cloud with a correct email and password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Agent service user signs in to the cloud

- **WHEN** a user holding the agent role signs in to the cloud with a correct email and password
- **THEN** the response is `200 OK`

#### Scenario: Courier signs in to the cloud

- **WHEN** a user holding the courier role signs in to the cloud with a correct email and password
- **THEN** the response is `200 OK`
