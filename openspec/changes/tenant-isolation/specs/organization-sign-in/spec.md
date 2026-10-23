## Purpose

Lets a person sign in to the organization they belong to, by naming the organization alongside their username and password, so that organizations sharing one deployment can reuse the same usernames without being able to sign in to each other.

## ADDED Requirements

### Requirement: Every user belongs to exactly one organization

A user SHALL belong to exactly one organization, fixed when the user is created. A username SHALL be unique within its organization; the same username MAY exist in different organizations. After the migration, every existing user SHALL belong to Laguna (`bb0cad3e-be13-4bad-9a37-bba9525bfece`).

#### Scenario: Same username in two organizations

- **WHEN** organization A has a user `cajero1` and a user `cajero1` is created in organization B
- **THEN** the user is created
- **AND** each `cajero1` signs in only to its own organization, with its own password

#### Scenario: Duplicate username in one organization

- **WHEN** a user `cajero1` is created in an organization that already has a `cajero1`
- **THEN** the request is rejected with `409 Conflict` and nothing is stored

#### Scenario: Existing users after the migration

- **WHEN** the migration has run on a database with existing users
- **THEN** every existing user belongs to Laguna and signs in with `organization: "laguna-escondida"` and their current username and password

### Requirement: Users are created in an organization the caller is entitled to

A user created by a signed-in user SHALL belong to the creator's organization, regardless of any organization named in the request. A user created with the admin API key in the cloud SHALL name its organization by slug; on the edge the box's organization is used, and a different named organization is rejected.

#### Scenario: Manager creates a user

- **WHEN** a signed-in user of organization A with `users:create` creates a user
- **THEN** the new user belongs to organization A

#### Scenario: Bootstrap user without an organization in the cloud

- **WHEN** `POST /api/users` is called with the admin API key in the cloud and the body names no organization
- **THEN** the request is rejected with `400 Bad Request` and nothing is stored

#### Scenario: Bootstrap user for an unknown organization

- **WHEN** `POST /api/users` is called with the admin API key in the cloud naming a slug that does not exist
- **THEN** the request is rejected with `404 Not Found` and nothing is stored

### Requirement: Sign-in names the organization

`POST /api/auth/signin` SHALL take `organization` (the organization's slug), `username` and `password`. In the cloud, `organization` SHALL be required. On the edge, `organization` SHALL be optional; when present it SHALL equal the box's organization's slug.

#### Scenario: Successful sign-in in the cloud

- **WHEN** `POST /api/auth/signin` is called in the cloud with `{"organization": "laguna-escondida", "username": "cajero1", "password": "<correct>"}`
- **THEN** the response is `200 OK` with a token for that user in Laguna
- **AND** the response body has the same fields as before this change

#### Scenario: Missing organization in the cloud

- **WHEN** `POST /api/auth/signin` is called in the cloud without `organization`
- **THEN** the response is `400 Bad Request`

#### Scenario: Sign-in on the edge without an organization

- **WHEN** `POST /api/auth/signin` is called on an edge box with a correct username and password and no `organization`
- **THEN** the user is signed in to the box's organization

#### Scenario: Sign-in on the edge naming another organization

- **WHEN** `POST /api/auth/signin` is called on an edge box with `organization` different from the box's slug
- **THEN** the response is the opaque `401 Unauthorized`

### Requirement: Sign-in failures are opaque

An unknown organization, a suspended organization, an unknown username, an inactive user and a wrong password SHALL all return the same `401 Unauthorized` status and body, so the response cannot reveal which organizations or usernames exist. The specific reason SHALL be logged server-side.

#### Scenario: Unknown organization

- **WHEN** sign-in names a slug that does not exist
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid username or password"}`

#### Scenario: Suspended organization

- **WHEN** sign-in names an organization whose status is `suspended`, with a correct username and password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid username or password"}`

#### Scenario: Username of another organization

- **WHEN** sign-in names organization A with the username and password of a user who exists only in organization B
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid username or password"}`

### Requirement: The token carries the organization

A token issued at sign-in SHALL carry the user's organization id as the claim `org_id`. Any request presenting a token without `org_id` SHALL be rejected with `401 Unauthorized`, so tokens issued before this change stop working and their holders must sign in again.

#### Scenario: Token claims

- **WHEN** a user of Laguna signs in
- **THEN** the token's claims include `org_id: "bb0cad3e-be13-4bad-9a37-bba9525bfece"` alongside `user_id`, `username`, `role_ids` and `permissions`

#### Scenario: Token issued before the change

- **WHEN** a request presents a valid, unexpired token that has no `org_id`
- **THEN** the response is `401 Unauthorized`

### Requirement: The current user's organization comes from the token

`GET /api/auth/me` SHALL return the organization named by the token's `org_id`, with the same response shape as before this change.

#### Scenario: Two organizations' users

- **WHEN** a user of organization B calls `GET /api/auth/me`
- **THEN** the `organization` object is organization B's, not Laguna's
