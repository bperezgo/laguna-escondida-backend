## 1. Foundations (backend)

- [x] 1.1 Add the cloud migration in design "Migration Plan" step 1, with its down migration. Verify with migration acceptance tests:
  - Laguna reads `active` + `full`
  - `trial` without `trial_ends_at` is rejected by the check
  - `plan = 'premium'` is rejected
  - the down migration refuses while an organization is in `trial`
  - `DB_HOST= make test-acceptance` passes from `backend/`
- [x] 1.2 Create `domain/plans`. It holds the plans, the features, the route-prefix → feature table exactly as in the `organization-plans` spec, `Includes(plan, feature)`, `RequiredPlan(feature)` and `Expired(status, trialEndsAt, now)`. Verify: table-driven tests cover every row of the spec table, expiry exactly at `trial_ends_at`, `active` never expiring, and a prefix matching nested paths.
- [x] 1.3 Add `plan` and `trial_ends_at` to the organization DTOs and repository. Add `PendingInvoiceStatusSkipped`. Add the domain errors: `trial_expired`, `organization_suspended` (for users), `plan_required` (carrying the required plan), invitation errors and code errors. Verify: `go build ./...` passes, and repository tests read and write the new fields.

## 2. Organization access in the cloud (backend)

- [x] 2.1 Add the `OrganizationAccessReader` port, its Postgres implementation (reads by id with no tenant in context) and the 30s `platform/cache` decorator (design decision 2). Run `make generate-mocks`. Verify: decorator tests show a hit skips the inner reader, a cache error falls through, and entries expire after 30s.
- [x] 2.2 Add `OrganizationAccessService.Check(ctx, orgID, principal, routePath)` with the order of design decision 1. Verify with service tests for:
  - suspended, then expired
  - `starter` on invoices and on reservations, each with its `required_plan`
  - an API key and the agent role on `einvoicing`
  - core routes on every plan
  - `full` passing everything
- [x] 2.3 Call the service from `JWTAuthMiddleware` when it is configured (cloud only), and map the three refusals to `403` with their codes. Verify: middleware tests show each code, the order (an expired `starter` gets `trial_expired`, not `plan_required`), that a token issued before expiry is refused after it, and that an edge-mode router makes no call.
- [x] 2.4 Add the plan-gating route test in `internal/bootstrap`, modeled on `agent_routes_test.go`. Verify:
  - every table prefix matches at least one cloud route
  - a `starter` token is refused with `plan_required` on every non-core route and not on any core route
  - the existing `agent_routes_test.go` still passes
- [x] 2.5 Accept `plan` and `trial_ends_at` on `POST`/`PATCH /api/platform/organizations`. Default to `full` + `active` on create, and add the validation rules of the `organization-management` delta. Verify: handler and service tests for every new scenario of that delta (trial without an end, unknown plan, activate, extend).
- [x] 2.6 Return `status`, `plan` and `trial_ends_at` in `/auth/me`'s `organization`. Verify: the `/auth/me` tests cover Laguna (`active`, `full`) and an organization on trial.

## 3. Sign-in (backend)

- [x] 3.1 In `UserService` sign-in, after a correct password, return `ErrTrialExpired` for an expired trial. Keep suspended opaque. The handler answers `403 {"error":"Trial ended","code":"trial_expired"}`. Verify: tests for an expired trial with the correct password (`403`), with a wrong password (`401`), suspended (`401`) and in trial (`200`).
- [x] 3.2 Relax the email rule: a manager may have no email, while admin, accountant, agent and courier still need one, and a user without an email never signs in to the cloud. Verify: user service tests for every scenario of the modified "Users have a globally unique email". The existing "Cloud role without an email" tests are updated only to name the four roles, after confirming with the developer.
- [x] 3.3 Add `GoogleIdentityVerifier` (port) and `platform/googleauth` on `coreos/go-oidc/v3` (design decision 4), plus `GOOGLE_CLIENT_ID` in cloud config (optional). Verify: adapter tests with a local JWKS and signed tokens cover the wrong audience, an expired token, an unverified email, the wrong issuer and a valid token.
- [x] 3.4 Add `POST /api/auth/google`. It's registered only in the cloud and only when `GOOGLE_CLIENT_ID` is set, and is rate-limited per address. Failures are opaque, except `trial_expired`. Verify: handler tests for every scenario of "Cloud sign-in with Google", and that the edge router answers `404`.

## 4. Signup (backend)

- [x] 4.1 Add `SignupService.SignUpWithGoogle` (design decisions 5 and 6): verify the token, fall back to Google sign-in for an existing email, and otherwise create the organization, admin, box user and roles in one unit of work, with the slug helper and its retries. Verify with service tests for:
  - the "Restaurante Doña Rosa" scenario (slug, status, plan, trial end, usernames, box user role and no email)
  - a taken slug becoming `-2`
  - the `jo@` → `caja` username
  - an existing email answering as a sign-in
  - a failure at the box-user step leaving nothing stored
  - an email race answered as a sign-in
- [x] 4.2 Add `POST /api/signup/google`, registered only when `SIGNUP_ENABLED` is true and `GOOGLE_CLIENT_ID` is set, and rate-limited. Add `SIGNUP_ENABLED` (default `false`) to cloud config. Verify: handler tests for `201`, `400` naming an invalid field, `401` on a bad token and `429`; a route test showing `404` when disabled and `POST /api/signup` always `404`.
- [x] 4.3 Add `UserService.GeneratePassword` and `POST /api/admin/users/:id/generate-password` (`users:update`, the user must have no email). Verify:
  - service and handler tests: a password of at least 12 characters is returned once, the old one stops working, `400` for a user with an email, `404` for another organization
  - reading the user afterwards carries no password

## 5. Box and invoices under plans (backend)

- [x] 5.1 Add `TrialEndsAt` to `NodeAuthRecord`, and have `NodeService.Authenticate` treat an expired trial as `ErrNodeOrganizationSuspended`. Verify: node service tests for an expired trial (refused as suspended, so it also gets `expired-box-backoff`'s `Retry-After`) and an active organization with a past `trial_ends_at` (accepted).
- [x] 5.2 Make `PendingInvoiceSyncApplier` insert `skipped` when the plan excludes e-invoicing (design decision 8). Verify:
  - applier integration tests: `starter` → `skipped`, `einvoicing` → `pending`
  - the submit cron's tests show `skipped` rows are never selected
  - the bill itself is stored in both cases

## 6. Staff invitations (backend)

- [x] 6.1 Add the invitation repository (table from 1.1) and `InvitationService`: create (role rules, the admin-only admin invite, `409` cases), list, new link, revoke, look up by token and accept with a password or Google, consumed under a row lock (design decision 7). Add `PUBLIC_APP_URL` to cloud config for the link. Verify: service tests for every scenario of the `staff-invitations` spec, including an expired organization (`403 organization_inactive`), a mismatched Google email and an email taken since creation.
- [x] 6.2 Add the handlers and routes: the `/api/admin/invitations*` routes (`users:create`) and the public `/api/invitations/:token*` routes (rate-limited), cloud only. Verify: handler tests for each status code, that the token appears only in the create and new-link responses, and that the edge router serves none of them.

## 7. Enrollment codes and the installer (backend + edge)

- [x] 7.1 Add the enrollment-code repository and `EnrollmentCodeService`: issue (`edge:enroll`, invalidates earlier codes, refused for inactive organizations) and enroll with a code, running `NodeService.Enroll` in the same transaction and marking the code used only on `201` (design decision 7). Verify with service tests for:
  - the format and normalization (`k7f2 9qxm`)
  - 31 minutes → invalid
  - the second issue invalidating the first
  - a `409` keeping the code and `replace: true` with it succeeding
  - a used code refused
  - an inactive organization refused
- [x] 7.2 Add `POST /api/edge/enrollment-codes` (JWT, `edge:enroll`) and `POST /api/edge/enroll-with-code` (public, rate-limited), cloud only. Verify: handler tests for `201`, `401 invalid or expired code`, `403` for a manager, `403 organization_inactive`, `409` with `active_box` and `429`.
- [ ] 7.3 Switch `edge/scripts/enroll.ps1` and the `edge/installer/kuenta.iss` wizard page to the code (design decision 10). Coordinate with `edge-installer`: if it hasn't merged, update its tasks instead. Verify: the installer's script tests under `edge/installer/test` pass with a stubbed cloud for a valid code, an invalid code (the existing bad-credentials exit code) and a `409` → replace. The wizard no longer asks for an email or password.

## 8. Backend acceptance, docs and checks

- [x] 8.1 Add acceptance tests under `backend/test/acceptance/signup/`, with Google verification stubbed:
  - sign up → `/auth/me` shows trial and plan
  - a `starter` token on invoices → `403 plan_required`
  - move the clock past the trial → requests and sign-in answer `trial_expired`, and the box pull is refused as suspended
  - the operator activates → access returns
  - invite → accept with a password → sign in
  - issue a code → enroll with it → pull with the node key

  Verify with `DB_HOST= make test-acceptance` from `backend/`.
- [x] 8.2 Document every new and changed endpoint, following `docs/api/products.md`:
  - `docs/api/auth.md` (Google, `trial_expired`)
  - `organizations.md` (plan, trial, `/auth/me`)
  - `edge-enrollment.md` (codes)
  - `users.md` (generate-password, managers without an email)
  - new `signup.md` and `invitations.md`
  - the `plan_required` / `trial_expired` / `organization_suspended` refusals wherever authenticated routes are described
  - working examples in `docs/examples/curl-examples.md`

  Verify: each new route has request, response and errors with realistic JSON.
- [x] 8.3 From `backend/`, run `make lint`, `go build ./...` and `make test`, and fix findings. Verify: all three pass.

## 9. Frontend

- [x] 9.1 Add a Google sign-in button component (Google Identity Services, `NEXT_PUBLIC_GOOGLE_CLIENT_ID`) and the route handlers `app/api/auth/google`, `app/api/signup/google` and `app/api/invitations/[token]/accept`, which set the session like `app/api/auth/signin/route.ts`. Add Google sign-in to `/signin`. Verify: route-handler tests for success, a `401` passed through and a `403 trial_expired` passed through; the button renders nothing when the client id is unset.
- [ ] 9.2 Add `/registro` (restaurant name, plan cards with what each includes, Google button) and `/invitacion/[token]` (organization and role, then a password or Google; invalid and expired link states). Verify: component tests for validation and each error state, and a manual run against a local backend with `SIGNUP_ENABLED=true`.
- [x] 9.3 Make menus plan-aware from `/auth/me`'s `organization.plan` (the same feature table as the backend), show an upgrade message on `plan_required`, and show a trial banner with the days left. Verify: tests that a `starter` user sees no invoices, reservations, delivery, notifications, integrations or API-key entries; a `full` user sees all of them; and the banner shows only in trial.
- [x] 9.4 Show "Tu prueba terminó. Escríbenos para continuar" on `403 trial_expired` at sign-in. The contact `mailto:` link comes from `NEXT_PUBLIC_SUPPORT_EMAIL` and is omitted when unset. Verify: tests with the variable set (link present) and unset (message without a link, no error), and `pnpm build` succeeding with it unset.
- [x] 9.5 Add `/ajustes/instalar-caja`: issue a code (copy, countdown, new code), "Generar contraseña" for the box user (shown once, with the username), and the installer download link. Verify: component tests for the code countdown and expiry, and that the password is shown once and cleared on leaving.
- [x] 9.6 Add invitations to `/users`: the invite form (cloud roles only, admin only offered to admins), a copy-link dialog after create and after a new link, the pending list and revoke. Allow creating a manager without an email in the user form. Verify: component tests for each action and the role options per inviter.
- [x] 9.7 From `frontend/`, run `pnpm lint` and `pnpm test`, and fix findings. Verify: both pass.

## 10. Landing and rollout

- [x] 10.1 Add the "Prueba 14 días" call to action in `landing/`, pointing at the app's `/registro`, behind a build setting that hides it until signup opens. Verify: `pnpm check`, `pnpm test` and `pnpm build` pass from `landing/`, with the call to action absent by default and present when enabled.
- [x] 10.2 Write the operator runbook: the Google Cloud setup (design decision 12), the new settings (`GOOGLE_CLIENT_ID`, `SIGNUP_ENABLED`, `PUBLIC_APP_URL`, `NEXT_PUBLIC_GOOGLE_CLIENT_ID`, `NEXT_PUBLIC_SUPPORT_EMAIL`), the rollout order of design "Migration Plan" including raising `MIN_EDGE_VERSION`, and how to activate, change the plan, extend a trial and suspend with `PATCH /api/platform/organizations/:id`. Verify: the runbook is linked from the cloud deployment docs and every setting is listed with its default.
