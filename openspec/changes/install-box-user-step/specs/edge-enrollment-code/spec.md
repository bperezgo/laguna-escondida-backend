## ADDED Requirements

### Requirement: Instalar caja shows who signs in on the box

The cloud's "Instalar caja" page SHALL list the organization's active users that have no email, which are the users who sign in only on the box. For each, it SHALL show the username and offer to change the password, which the admin types and confirms. It SHALL NOT generate a password. When there is no such user, it SHALL offer to create one, which opens the user form ready for a manager without an email. The page SHALL say that the box signs in with the username, not the email, and that a Google account cannot sign in on the box.

Changing a password from this page SHALL use the same rule as Usuarios: it requires `users:update` and acts only on users of the caller's organization.

#### Scenario: A box user exists

- **WHEN** the organization has an active manager `rosa` without an email
- **THEN** the page shows the username `rosa` and a "Cambiar contraseña" action
- **AND** after the admin sets a password there, `rosa` signs in on the box with that password once the box has pulled it

#### Scenario: No box user yet

- **WHEN** the organization's only user is its admin
- **THEN** the page offers "Crear usuario de caja"
- **AND** following it opens the user form with the manager role selected and no email

#### Scenario: Users with an email are not listed

- **WHEN** the organization has an admin and an accountant, both with an email
- **THEN** neither appears in the list of box users

#### Scenario: Another organization's user

- **WHEN** an admin of organization A sends a password change for a user of organization B
- **THEN** the cloud answers `404` and B's user keeps its password

#### Scenario: Caller without users:update

- **WHEN** a user without `users:update` sends a password change for a box user
- **THEN** the cloud answers `403` and the password is unchanged

#### Scenario: The generator is gone

- **WHEN** anyone calls `POST /api/admin/users/:id/generate-password`
- **THEN** the cloud answers `404` and no password changes
