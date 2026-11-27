## MODIFIED Requirements

### Requirement: A person signs up with Google

Signup SHALL be possible only with Google; there is no password signup. `POST /api/signup/google` SHALL take a Google ID token, `restaurant_name` (1–255 characters after trimming) and `plan` (`starter`, `einvoicing` or `full`); an invalid field SHALL answer `400` naming it. The route SHALL exist only in the cloud and SHALL NOT require other authentication. The ID token SHALL be accepted only when Google's signature is valid, its audience is the cloud's configured Google client id, it is unexpired, and its email is marked verified by Google. Otherwise the response SHALL be `401`.
- When the token's email belongs to no user, the account SHALL be created at once, as described in "Signup creates the organization, its admin and its box user", with the Google name as the admin's name. The response SHALL be `201` with `{"status": "pending", "organization_name": "<the restaurant name>", "plan": "<the chosen plan>", "setup_token": "<a setup token for the new admin>"}` and SHALL carry no sign-in token. The setup token is described in `signup-billing-setup`.
- When the email already belongs to a user, nothing SHALL be created, and the request SHALL be treated as a Google sign-in (see `organization-sign-in`). For an admin of a `pending` organization that answers `403` with code `pending_approval` and a setup token.

#### Scenario: New restaurant with Google

- **WHEN** `POST /api/signup/google` is called with a valid ID token for an unregistered, Google-verified email
- **THEN** the response is `201` with status `pending`, a setup token and no sign-in token
- **AND** no email is sent

#### Scenario: Signing up again while pending

- **WHEN** the same person calls `POST /api/signup/google` again before their trial starts
- **THEN** no organization is created
- **AND** the response is `403` with code `pending_approval` and a setup token

#### Scenario: Google account already registered

- **WHEN** the Google email already belongs to a manager of Laguna
- **THEN** no organization is created
- **AND** the response is the same as a Google sign-in for that manager

#### Scenario: Token for another application

- **WHEN** the ID token's audience is not the cloud's Google client id
- **THEN** the response is `401` and nothing is created
