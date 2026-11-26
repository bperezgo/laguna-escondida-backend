## Purpose

Lets an admin install their organization's box without typing their cloud password into the installer: the cloud gives them a short-lived, single-use code, and the installer enrolls the box with it. This works the same for admins who signed up with Google and have no password.

## ADDED Requirements

### Requirement: An admin gets an enrollment code

`POST /api/edge/enrollment-codes` SHALL issue a code for the caller's organization and answer `201` with `code` and `expires_at`.
- It SHALL require the `edge:enroll` permission.
- The code SHALL be 8 characters from an alphabet without look-alike characters (no `0`/`O`, `1`/`I`/`L`), shown to the admin as `XXXX-XXXX`. It SHALL be compared ignoring case, spaces and the hyphen.
- It SHALL be valid for 30 minutes.
- Only a hash SHALL be stored.
- Issuing a code SHALL invalidate the organization's earlier unused codes.
- An organization that is suspended or whose trial has expired SHALL NOT get a code.
- The route SHALL exist only in the cloud.

#### Scenario: Admin opens "Instalar caja"

- **WHEN** an admin calls `POST /api/edge/enrollment-codes`
- **THEN** the response is `201` with a code like `K7F2-9QXM` and an `expires_at` 30 minutes ahead

#### Scenario: Manager

- **WHEN** a manager calls it
- **THEN** the response is `403`

#### Scenario: New code replaces the old one

- **WHEN** an admin issues a code, then another, then enrolls with the first
- **THEN** the first code is refused

### Requirement: The installer enrolls a box with a code

`POST /api/edge/enroll-with-code` SHALL take a `code`, a box `name` and an optional `replace`, and SHALL require no other authentication. It SHALL behave exactly like `POST /api/edge/enroll` called by that organization's admin (see `edge-box-enrollment`):
- the same `201` body with the node's credentials
- the same `409` with `box_already_enrolled` and `active_box`
- the same replacement

A code SHALL be used up only by a `201`. A `409` SHALL leave it valid, so the installer can ask the admin and retry with `replace: true`.

An unknown, expired or used code SHALL answer `401` with `{"error": "invalid or expired code"}`, without saying which. When the code's organization has become suspended or its trial has expired, the response SHALL be `403` with code `organization_inactive`. The route SHALL be limited per client address, answering `429` with `Retry-After` past the limit, and SHALL exist only in the cloud.

#### Scenario: First box with a code

- **WHEN** the installer posts a valid code and `{"name": "Caja principal"}` for an organization with no box
- **THEN** the response is `201` with `node_id`, `organization_id`, `organization_name`, `node_key` and `cloud_sync_url`
- **AND** the code no longer works

#### Scenario: Ask before replacing

- **WHEN** the organization already has an active box and the installer posts a valid code without `replace`
- **THEN** the response is `409` with `active_box`
- **AND** posting the same code again with `replace: true` answers `201`

#### Scenario: Code typed in lowercase with spaces

- **WHEN** the code `K7F2-9QXM` is entered as `k7f2 9qxm`
- **THEN** it is accepted

#### Scenario: Expired code

- **WHEN** a code is used 31 minutes after it was issued
- **THEN** the response is `401` with `{"error": "invalid or expired code"}`

#### Scenario: Guessing codes

- **WHEN** one address posts codes past the limit
- **THEN** the extra requests answer `429` and none is checked

### Requirement: The installer asks for a code, not a password

The edge installer SHALL ask for the enrollment code and the box name, and SHALL NOT ask for the admin's email or password. It SHALL tell the admin where to get the code ("Instalar caja" in the cloud app). It SHALL handle `409` by showing the active box's name and last-sync time and asking before retrying with `replace: true`, as it does today.

#### Scenario: Install with a code

- **WHEN** the admin runs the installer and enters a valid code
- **THEN** the box is enrolled without the admin's password ever being asked for or stored
