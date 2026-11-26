## Why

Only the operator can create an organization and its first user. A restaurant must be able to sign up alone, try any plan for 14 days, install its box and bring its staff. Payment is manual.

## What Changes

- **Public signup in the cloud, with Google only.** Creates the organization and its first user as **admin**. Business info, branding and e-invoicing prefixes stay `null`; only the operator sets them.
- **Default box user.** Signup also creates a manager with no email. Username: the admin's email local part (sanitized); random password; box-only. Admins generate its password in the cloud, shown once.
- **No email sending.** Google verifies the signup email, and there is no password signup yet.
- **Google sign-in.** Cloud only, matched by verified email. Edge sign-in stays username + password.
- **Enrollment code.** A signed-in admin gets a single-use code, valid 30 minutes, from "Instalar caja". The installer enrolls with it instead of the admin's password, so Google-only admins can install.
- **Plans.**
  - `starter` is the core POS; `einvoicing` adds DIAN; `full` adds reservations, delivery, notifications, Wompi, agents and MCP. `starter` bills are never sent to DIAN.
  - The cloud refuses the routes a plan excludes.
  - `/auth/me` returns the plan.
- **Trial.**
  - Status becomes `trial | active | suspended`, with `trial_ends_at`.
  - Signup starts 14 days on the chosen plan.
  - The operator activates and changes plans through the existing platform endpoints.
- **Hard stop at expiry**: refused like a suspended organization (no cloud sign-in, no box sync).
- **Staff invitations, for cloud users only.** The admin gets a link to share (e.g. WhatsApp); the invitee accepts with a password or Google. Box staff are still created directly.

Not in this change: password signup and email sending (SES, later), payments, self-serve organization settings, a free tier, data export.

## Capabilities

### New Capabilities

- `self-serve-signup`: Google signup and the organization, admin and default box user it creates.
- `organization-plans`: plans, the 14-day trial, expiry, and per-plan route gating.
- `staff-invitations`: inviting cloud users, accepting and expiring invitations.
- `edge-enrollment-code`: issuing a single-use code and enrolling a box with it.

### Modified Capabilities

- `organization-management`: status gains `trial`, plus `plan` and `trial_ends_at`; status and plan now change runtime behavior.
- `organization-sign-in`:
  - Google sign-in in the cloud.
  - An expired trial, with the correct password, gets `403 trial_expired`.
  - A cloud role needs an email only to sign in to the cloud, so a manager without one is allowed.

## Impact

- **Backend (cloud)**:
  - Migrations for organization status and plan, invitations and enrollment codes.
  - New endpoints, rate-limited.
  - Plan-gating middleware.
  - A Google OAuth client id setting.
- **Edge installer**: asks for the code (changes the in-flight `edge-installer`).
- **Frontend**: signup, accept-invitation and "Instalar caja" pages, an invite form, and plan-aware menus.
- **Landing**: a "Prueba 14 días" call to action.
- **API docs**: `auth.md`, `organizations.md`, `edge-enrollment.md`, and a new `invitations.md`.
- **Depends on**: `expired-box-backoff`, on the edge before signup opens.
