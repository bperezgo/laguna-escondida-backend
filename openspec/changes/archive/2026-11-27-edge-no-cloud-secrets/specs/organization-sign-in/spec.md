## MODIFIED Requirements

### Requirement: Users are created in an organization the caller is entitled to

A user created by a signed-in user SHALL belong to the creator's organization, regardless of any organization named in the request. A user created with the admin API key SHALL name its organization by slug; this is possible only in the cloud, because an edge box does not accept the admin API key.

#### Scenario: Manager creates a user

- **WHEN** a signed-in user of organization A with `users:create` creates a user
- **THEN** the new user belongs to organization A

#### Scenario: Bootstrap user without an organization in the cloud

- **WHEN** `POST /api/users` is called with the admin API key in the cloud and the body names no organization
- **THEN** the request is rejected with `400 Bad Request` and nothing is stored

#### Scenario: Bootstrap user for an unknown organization

- **WHEN** `POST /api/users` is called with the admin API key in the cloud naming a slug that does not exist
- **THEN** the request is rejected with `404 Not Found` and nothing is stored

#### Scenario: Admin key on an edge box

- **WHEN** `POST /api/users` is called on an edge box with an admin API key and no user token
- **THEN** no user is created
