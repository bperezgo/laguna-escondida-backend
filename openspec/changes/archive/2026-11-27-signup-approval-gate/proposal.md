## Why

Self-serve signup gives anyone 14 days of the full product the moment they click a plan. The owner wants to give a trial only to restaurants that leave card details, and online payment is a later change. Until then, a signup must wait for us before it can use anything. The landing change `landing-pricing-and-product-proof` makes "Prueba 14 días" its primary action, so this gate must ship first or with it.

## What Changes

- **New organization status `pending`.** Signup creates the organization, its admin and its box user as today, but with status `pending`, the chosen plan stored, and no `trial_ends_at`. The trial clock does not run.
- **A pending organization is stopped like a suspended one**: no cloud requests, no box enrollment, no invitations, no sync. Its users get `403` with code `pending_approval` instead of a token.
- **BREAKING (signup API):** `POST /api/signup/google` answers `201` with the organization's status and no token when it creates an account. A Google signup or sign-in of an existing pending account answers `403 pending_approval`. The frontend ships with it.
- **"Solicitud recibida" page** in the operations app (`/registro/pendiente`). Signup sends the person there, and so does any sign-in that gets `pending_approval`. It says an operator will approve the subscription before the 14-day trial starts.
- **Starting the trial is one domain operation.** `POST /api/platform/organizations/:id/start-trial` moves a `pending` organization to `trial` with `trial_ends_at` 14 days from that moment. The future payment-gateway change calls the same operation when a card is saved, without reworking it. The operator can also skip the trial with `PATCH {"status": "active"}`.
- **The operator finds waiting signups** with `GET /api/platform/organizations?status=pending`. Nothing notifies anyone.

Out of scope: payment, card capture, operator notifications, and self-serve trial start. No edge change: a pending organization can never enroll a box.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `organization-plans`: a `pending` status that stops the organization, and the start-trial operation that ends it.
- `self-serve-signup`: signup creates a `pending` organization and returns no token.
- `organization-management`: `pending` joins the statuses. Its transitions are limited: only signup creates it, only start-trial turns it into `trial`. The list can be filtered by status.
- `organization-sign-in`: a correct sign-in of a pending organization's user answers `403 pending_approval`.

These four come from `self-serve-signup-trial`, which has merged but is not archived yet. Its specs must be archived into `openspec/specs/` before this change is.

## Impact

- **Backend (cloud):** a migration that widens the status constraint, the status constant and access checks, signup and sign-in services, the new platform route and the list filter. `routes_cloud.golden` gains the route.
- **Frontend:** the `/registro/pendiente` page, plus signup and sign-in handling of `201` without a token and of `pending_approval`.
- **API docs:** `signup.md`, `auth.md` (sign-in and access refusals), `organizations.md` (status, start-trial, list filter).
- **Edge:** none. The edge status constraint stays as it is.
- **Data:** none to migrate. Production has no trial organizations.
- **Landing:** `landing-pricing-and-product-proof` records that it depends on this change.
