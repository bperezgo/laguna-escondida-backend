## MODIFIED Requirements

### Requirement: Sign-in context is public

`GET /api/auth/context` SHALL require no authentication. In the cloud it SHALL return `{"mode": "cloud"}`. On an edge box it SHALL return `{"mode": "edge", "organization": {"name": "<the box's organization name>"}, "users_ready": <bool>, "sync": <state or null>}`.
- `users_ready` SHALL be `true` when the box has at least one active, undeleted user of its organization, and `false` otherwise. It SHALL be computed from the box's own database, so it does not depend on the cloud being reachable.
- `sync` SHALL be `null` when `users_ready` is `true`. Otherwise it SHALL be one of:
  - `waiting`: the last pull succeeded, or none has finished yet
  - `offline`: the last pull could not reach the cloud
  - `failed`: the last pull reached the cloud but its data could not be applied on the box
  - `replaced` or `suspended`: the cloud refused the box, as in `sync_blocked`
- When the box's organization cannot be read, the response SHALL still be `200 OK` with `mode`, `users_ready` and `sync`, and without `organization`.
- It SHALL NOT return any other organization or user data: no user counts, usernames, error messages, cloud addresses or timestamps.

#### Scenario: Context in the cloud

- **WHEN** `GET /api/auth/context` is called in the cloud without a token
- **THEN** the response is `200 OK` with exactly `{"mode": "cloud"}`

#### Scenario: Context on an edge box

- **WHEN** `GET /api/auth/context` is called on Laguna's edge box without a token, and the box has active users
- **THEN** the response is `200 OK` with `{"mode": "edge", "organization": {"name": "Laguna Escondida"}, "users_ready": true, "sync": null}`

#### Scenario: Fresh box still waiting for its users

- **WHEN** the context is called on a box with no users whose last pull succeeded
- **THEN** the response has `users_ready: false` and `sync: "waiting"`

#### Scenario: Fresh box whose pulls fail on the box

- **WHEN** the context is called on a box with no users whose last pull reached the cloud but failed to apply (for example a foreign-key violation)
- **THEN** the response has `users_ready: false` and `sync: "failed"`
- **AND** the body contains no error text, table or constraint name

#### Scenario: Fresh box without internet

- **WHEN** the context is called on a box with no users whose last pull could not reach the cloud
- **THEN** the response has `users_ready: false` and `sync: "offline"`

#### Scenario: Box with users and no internet

- **WHEN** the context is called on a box that has users and cannot reach the cloud
- **THEN** the response has `users_ready: true` and `sync: null`

#### Scenario: Box without its organization row

- **WHEN** the context is called on a box whose organization row does not exist
- **THEN** the response is `200 OK` with `mode: "edge"`, `users_ready` and `sync`, and no `organization`

#### Scenario: Unauthenticated caller learns nothing about users

- **WHEN** an unauthenticated caller on the LAN calls the context on a box with several users
- **THEN** the body has `users_ready: true` and contains no count, username, name or role of any user

## ADDED Requirements

### Requirement: The box's sign-in page waits for the first user

On an edge box, while the sign-in context reports `users_ready: false`, the sign-in page SHALL show a waiting screen instead of the sign-in form. The screen SHALL say "Esperando a que lleguen los usuarios desde la nube…" and SHALL give guidance that depends on `sync`:
- `waiting`: the users arrive within a minute or two; if it takes longer, check that a box user exists in the cloud under Usuarios
- `offline`: the box has no internet connection; check the network
- `failed`: the box could not receive the data from the cloud; contact support, and the logs are in `C:\kuentapos\logs`
- `replaced` or `suspended`: the same reason the POS already shows for that state

The page SHALL check the context again at most every 5 seconds and SHALL switch to the form by itself when `users_ready` becomes `true`. The waiting screen SHALL offer a link that shows the form anyway.

The page SHALL show the form when `users_ready` is `true`, when the response has no `users_ready` (a cloud, or a box running an older release), or when the context cannot be read. It SHALL NOT show the waiting screen in the cloud.

#### Scenario: First user arrives while the page is open

- **WHEN** the sign-in page on a fresh box shows the waiting screen and the box's next pull brings its first user
- **THEN** within 5 seconds of the context reporting `users_ready: true`, the page shows the sign-in form without a reload

#### Scenario: Pull failing on a fresh box

- **WHEN** the context reports `users_ready: false` and `sync: "failed"`
- **THEN** the waiting screen says the box could not receive the data from the cloud, tells the person to contact support, and names `C:\kuentapos\logs`
- **AND** it does not say that the password is wrong or that there is no internet

#### Scenario: Box with users goes offline

- **WHEN** a box with users loses its internet connection and the sign-in page is opened
- **THEN** the sign-in form is shown

#### Scenario: Context unreadable

- **WHEN** `GET /api/auth/context` fails or answers `404`
- **THEN** the sign-in form is shown

#### Scenario: Show the form anyway

- **WHEN** the waiting screen is shown and the person chooses to show the form anyway
- **THEN** the sign-in form is shown, and signing in behaves as before this change

#### Scenario: Cloud sign-in page

- **WHEN** the sign-in page is opened in the cloud
- **THEN** the cloud sign-in form is shown and no waiting screen appears
