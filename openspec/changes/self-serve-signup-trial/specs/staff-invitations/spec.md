## Purpose

Lets an organization bring the people who use the cloud app (managers, accountants, couriers, other admins) with an invitation link the admin shares themselves (for example on WhatsApp), so each one sets their own password or signs in with Google. The cloud sends no email. Box staff are not invited; they are still created directly.

## ADDED Requirements

### Requirement: A user invites a person to the cloud app with a link

`POST /api/admin/invitations` SHALL take an `email`, a `name` and `role_ids`, and SHALL create an invitation in the caller's organization.
- It SHALL answer `201` with the invitation and its `link`, valid for 7 days. That response is the only place the link appears; only a hash of its token is stored. The cloud SHALL send no email.
- It SHALL require `users:create`.
- Every role SHALL be one of admin, manager, accountant or courier. The agent role and floor-only roles SHALL be rejected with `400`.
- Inviting with the admin role SHALL require the caller to hold the admin role, and SHALL be `403` otherwise.
- An email that already belongs to a user in any organization SHALL answer `409`. An email with a pending invitation in the same organization SHALL answer `409` too: get a new link for that invitation instead.
- There SHALL be no limit on the number of users or invitations of an organization.

#### Scenario: Admin invites an accountant

- **WHEN** an admin invites `contadora@example.com` with the accountant role
- **THEN** the response is `201` with a `link` the admin can copy and share
- **AND** no email is sent

#### Scenario: Manager tries to invite an admin

- **WHEN** a manager with `users:create` invites someone with the admin role
- **THEN** the response is `403` and nothing is stored

#### Scenario: Agent role

- **WHEN** an invitation names the agent role
- **THEN** the response is `400`

#### Scenario: Email already has an account

- **WHEN** the email belongs to a user of any organization
- **THEN** the response is `409` and nothing is stored

### Requirement: Invitations are listed, renewed and revoked

- `GET /api/admin/invitations` SHALL list the caller's organization's invitations that are neither accepted nor revoked, with email, name, roles, inviter, creation time and expiry.
- `POST /api/admin/invitations/:id/link` SHALL return a new link valid for 7 days and invalidate the previous one, for when the admin lost it or it expired.
- `DELETE /api/admin/invitations/:id` SHALL revoke the invitation, so its link no longer works.
- These routes SHALL require `users:create`, and SHALL answer `404` for an invitation of another organization.

#### Scenario: Revoke before acceptance

- **WHEN** an admin revokes an invitation and the invitee then opens the link
- **THEN** the link is refused and no user is created

#### Scenario: New link

- **WHEN** an admin asks for a new link
- **THEN** the old link is refused and the new one works

### Requirement: The invitee accepts with a password or with Google

Both routes are public and cloud only:
- `GET /api/invitations/:token` SHALL return the organization name, the invitee's email and name, and the roles, so the acceptance page can show them.
- `POST /api/invitations/:token/accept` SHALL take either a `password` of at least 8 characters or a Google ID token.
  - A Google token SHALL be valid as for Google sign-in, and its email SHALL equal the invitation's email. Otherwise the response is `400`. Accepting with a password takes the invitation's email as given by the admin; the link is the proof of the invitation.
  - On success it SHALL create the user in the inviting organization with the invitation's name, email and roles, and the username equal to the email. It SHALL mark the invitation accepted and answer `201` with a sign-in token.

An unknown, expired, revoked or accepted token SHALL answer `400` with code `invalid_or_expired_link`. An invitation whose organization is suspended or expired SHALL answer `403` with code `organization_inactive` and create nothing. When the email has been taken by another account since the invitation was created, accepting SHALL answer `409`.

#### Scenario: Accept with a password

- **WHEN** the invitee opens a valid link and sets a password
- **THEN** a user is created with the invited roles in the inviting organization
- **AND** the response carries a token for that user

#### Scenario: Accept with a different Google account

- **WHEN** the invitee accepts with a Google account whose email is not the invited one
- **THEN** the response is `400` and no user is created

#### Scenario: Expired invitation

- **WHEN** the link is opened 8 days after it was created
- **THEN** the response is `400` with code `invalid_or_expired_link`

#### Scenario: Organization's trial ended

- **WHEN** an invitation of an organization whose trial has expired is accepted
- **THEN** the response is `403` with code `organization_inactive` and no user is created

### Requirement: Invitation routes are rate-limited and exist only in the cloud

The public invitation routes SHALL be limited per client address and answer `429` with `Retry-After` past the limit. Every invitation route SHALL be served only in cloud mode.

#### Scenario: Edge mode

- **WHEN** the backend runs in edge mode
- **THEN** no invitation route is served
