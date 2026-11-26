## Purpose

Lets a restaurant create its own account from the internet with its Google account, so no email has to be sent or verified by us. Signing up creates its organization on a 14-day trial of the chosen plan, its first admin, and a user ready to operate the box.

## ADDED Requirements

### Requirement: A person signs up with Google

Signup SHALL be possible only with Google; there is no password signup. `POST /api/signup/google` SHALL take a Google ID token, `restaurant_name` (1–255 characters after trimming) and `plan` (`starter`, `einvoicing` or `full`); an invalid field SHALL answer `400` naming it. The route SHALL exist only in the cloud and SHALL NOT require other authentication. The ID token SHALL be accepted only when Google's signature is valid, its audience is the cloud's configured Google client id, it is unexpired, and its email is marked verified by Google. Otherwise the response SHALL be `401`.
- When the token's email belongs to no user, the account SHALL be created at once, as described in "Signup creates the organization, its admin and its box user", with the Google name as the admin's name. The response SHALL be `201` with a sign-in token.
- When the email already belongs to a user, nothing SHALL be created, and the request SHALL be treated as a Google sign-in (see `organization-sign-in`).

#### Scenario: New restaurant with Google

- **WHEN** `POST /api/signup/google` is called with a valid ID token for an unregistered, Google-verified email
- **THEN** the response is `201` with a token for the new admin
- **AND** no email is sent

#### Scenario: Google account already registered

- **WHEN** the Google email already belongs to a manager of Laguna
- **THEN** no organization is created
- **AND** the response is the same as a Google sign-in for that manager

#### Scenario: Token for another application

- **WHEN** the ID token's audience is not the cloud's Google client id
- **THEN** the response is `401` and nothing is created

### Requirement: Signup creates the organization, its admin and its box user

Creating an account from a signup SHALL store, together or not at all, the organization, the admin and the box user described below.

**The organization:**
- `name` and `display_name` are the restaurant name.
- Its `slug` is derived from the restaurant name: lowercased, accents removed, non-alphanumeric runs replaced by `-`, cut to 40 characters, and with `-2`, `-3`, … appended when already taken.
- `status` is `trial` and `trial_ends_at` is 14 days after creation.
- `plan` is the chosen plan.
- `timezone`, `country`, `locale` and `currency` are `America/Bogota`, `CO`, `es-CO` and `COP`.
- Business info, receipt text, branding and e-invoicing prefixes are unset.

**The admin:** the Google account's name and email, with the admin role and no password; they sign in with Google. Its username is its email.

**The box user:** a user with the manager role and no email.
- Its name is "Caja".
- Its username is the part of the email before `@`, lowercased and keeping only letters, digits, `.`, `_` and `-`. When fewer than 3 characters remain, the username is `caja`.
- Its password is random and is never returned at creation.

#### Scenario: Organization created from a signup

- **WHEN** "Restaurante Doña Rosa" signs up on plan `einvoicing` with `rosa.perez@example.com`
- **THEN** the organization has slug `restaurante-dona-rosa`, status `trial`, plan `einvoicing` and `trial_ends_at` 14 days ahead
- **AND** its admin has username `rosa.perez@example.com`
- **AND** its box user is a manager with username `rosa.perez` and no email

#### Scenario: Slug taken

- **WHEN** a second restaurant named "Restaurante Doña Rosa" signs up
- **THEN** its slug is `restaurante-dona-rosa-2`

#### Scenario: Short email name

- **WHEN** the email is `jo@example.com`
- **THEN** the box user's username is `caja`

### Requirement: An admin generates a new password for a user without an email

`POST /api/admin/users/:id/generate-password` SHALL set a new random password of at least 12 characters for a user of the caller's organization who has no email, and SHALL return it in the response. That response is the only place the password ever appears. It SHALL require `users:update`, and SHALL answer `404` for a user of another organization and `400` for a user who has an email.

#### Scenario: Admin prepares the box

- **WHEN** the admin generates a password for the box user
- **THEN** the response contains the new password
- **AND** the box user signs in on the box with its username and that password once the box has pulled it
- **AND** the previous password no longer works

#### Scenario: Password is never shown again

- **WHEN** the admin reads the box user afterwards
- **THEN** the response carries no password

### Requirement: Signup is rate-limited

`POST /api/signup/google` SHALL be limited per client address. Over the limit it SHALL answer `429` with a `Retry-After` header and create nothing.

#### Scenario: Signup flood

- **WHEN** one address calls `POST /api/signup/google` past the limit
- **THEN** the extra requests answer `429` and no organization is created

### Requirement: There is no password signup

The cloud SHALL NOT offer a route that creates an organization from an email and password, and SHALL send no email during signup.

#### Scenario: Password signup attempt

- **WHEN** `POST /api/signup` is called with an email and password
- **THEN** the route is not found and nothing is created
