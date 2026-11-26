## MODIFIED Requirements

### Requirement: Sign-in failures are opaque in each mode

In the cloud, an unknown email, a suspended organization, an inactive user, a user without a cloud role and a wrong password SHALL all return `401 Unauthorized` with `{"error": "Invalid email or password"}`. On an edge box, an unknown username, an inactive user and a wrong password SHALL all return `401 Unauthorized` with `{"error": "Invalid username or password"}`. The response SHALL NOT reveal which emails, usernames or organizations exist. The specific reason SHALL be logged server-side.

There are two exceptions, both in the cloud, and both only when the email and password are correct and the user is active and holds a cloud role. That way the person learns to wait or to contact us rather than retrying their password. The same applies to a Google sign-in whose account matches such a user.
- When the organization is `pending`, the response SHALL be `403 Forbidden` with `{"error": "Approval pending", "code": "pending_approval"}`.
- When the organization's trial has expired, the response SHALL be `403 Forbidden` with `{"error": "Trial ended", "code": "trial_expired"}`.

#### Scenario: Unknown email

- **WHEN** cloud sign-in uses an email no user has
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Suspended organization

- **WHEN** a user of an organization whose status is `suspended` signs in to the cloud with a correct email and password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Pending organization with the correct password

- **WHEN** a user with a password, of an organization whose status is `pending`, signs in with a correct email and password
- **THEN** the response is `403 Forbidden` with code `pending_approval` and no token

#### Scenario: Pending organization with a wrong password

- **WHEN** someone signs in with that user's email and a wrong password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Expired trial with the correct password

- **WHEN** the admin of an organization whose trial ended yesterday signs in with a correct email and password
- **THEN** the response is `403 Forbidden` with code `trial_expired` and no token

#### Scenario: Expired trial with a wrong password

- **WHEN** someone signs in with that admin's email and a wrong password
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid email or password"}`

#### Scenario: Username of another organization

- **WHEN** sign-in on organization A's edge box uses the username and password of a user who exists only in organization B
- **THEN** the response is `401 Unauthorized` with `{"error": "Invalid username or password"}`

### Requirement: Cloud sign-in with Google

In the cloud, `POST /api/auth/google` SHALL take a Google ID token. It SHALL sign in the user whose email equals the token's email.
- **Token checks:** the token SHALL be accepted only when Google's signature is valid, its audience is the cloud's configured Google client id, it is unexpired, and its email is marked verified by Google.
- **Response:** on success, the same `200` body as `POST /api/auth/signin`.
- **Failures:** an invalid token, an unknown email, a suspended organization, an inactive user and a user without a cloud role SHALL all return `401 Unauthorized` with `{"error": "Google sign-in failed"}`. A pending organization and an expired trial follow the exceptions in "Sign-in failures are opaque in each mode".
- **Who can use it:** any user with an email, whichever way they signed up or were invited, so a person who set a password may also use Google with the same email.
- **Edge:** the route SHALL NOT exist on an edge box. Edge sign-in stays username and password.

#### Scenario: Google sign-in of a Google signup

- **WHEN** the admin who signed up with Google, whose trial has been started, calls `POST /api/auth/google` with a valid ID token for their email
- **THEN** the response is `200 OK` with a token for that admin

#### Scenario: Google sign-in before approval

- **WHEN** the admin who signed up with Google calls `POST /api/auth/google` while their organization is `pending`
- **THEN** the response is `403 Forbidden` with code `pending_approval` and no token

#### Scenario: Password user signs in with Google

- **WHEN** a manager who set a password signs in with a Google account of the same, Google-verified email
- **THEN** the response is `200 OK`

#### Scenario: Unverified Google email

- **WHEN** the ID token's email is not marked verified by Google
- **THEN** the response is `401 Unauthorized` with `{"error": "Google sign-in failed"}`

#### Scenario: No account

- **WHEN** the token's email belongs to no user
- **THEN** the response is `401 Unauthorized` with `{"error": "Google sign-in failed"}`

#### Scenario: Edge box

- **WHEN** `POST /api/auth/google` is called on an edge box
- **THEN** the route is not found
