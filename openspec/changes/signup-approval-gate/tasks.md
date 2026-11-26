Backend commands run from `backend/`, frontend commands from `frontend/`. Read only the files in design "Implementation map". Archive `self-serve-signup-trial` before archiving this change.

## 1. Status and migration

- [x] 1.1 Add migration `000097_add_organization_pending_status` (design Migration Plan step 1). Verify: an acceptance test over a testcontainers database (prefix `DB_HOST=`) inserts a `pending` organization without `trial_ends_at`, fails to insert one with it, and after the down migration finds the `pending` row `suspended`.
- [x] 1.2 Add `OrganizationStatusPending` and `plans.StatusPending`, `ErrOrganizationPending` and `ErrOrganizationNotPending`. Verify: `go build ./...` passes.

## 2. Access refusals

- [x] 2.1 Extend `OrganizationAccessService.Check` and `Usable` (design decision 2). Verify with unit tests: pending → `ErrOrganizationPending` from `Check`, and `ErrOrganizationInactive` from `Usable`; suspended still wins over pending; a trial is unaffected.
- [x] 2.2 Map `ErrOrganizationPending` to `403 pending_approval` in `abortOrganizationAccess`. Verify: `jwt_auth_organization_access_test.go` gains cases for a JWT of a pending organization, and `api_key_auth_middleware_test.go` a case for an API key of a pending organization refused with the opaque `401`, as for a suspended one.
- [x] 2.3 Map `pending` to `suspended` in `node_service.go`. Verify: a unit test for a node of a pending organization gets the suspended refusal.

## 3. Sign-in

- [x] 3.1 Refuse a pending organization in `completeSignIn` after the role check, and pass it through in `SignInWithGoogle`. Add `respondPendingApproval` and use it in `UserHandler.SignInHandler` and `SignupHandler.respondGoogleError`. Verify with tests: a correct password → `403 pending_approval`, a wrong password → the opaque `401`, Google of a pending admin → `403 pending_approval`, and an unchanged result for active, trial and expired organizations.

## 4. Signup

- [x] 4.1 Make `SignupService.create` store `pending` with no trial end and issue no token, and make the handler answer `201 {"status", "organization_name", "plan"}` (design decision 3). Verify: the signup service and handler tests assert the stored status, the missing `trial_ends_at`, the body with no `token`, and that the admin and box user are still created. A repeated signup of the same email answers `403 pending_approval`.

## 5. Starting the trial, update rules, list filter

- [x] 5.1 Add `OrganizationService.StartTrial` with the conditional update (design decision 4), and `POST /api/platform/organizations/:id/start-trial` in `organization_handler.go` and `routes_cloud.go`. Verify with tests: pending → `200` with `trial` and an end 14 days after the injected clock; trial or active → `409 not_pending` and unchanged; unknown → `404`; without the admin key → refused like the other platform routes. Update `routes_cloud.golden`.
- [x] 5.2 Apply design decision 5 to create and update validation. Verify with `organization_service` tests for every case in the `organization-management` delta: pending → trial by PATCH is refused, `status: pending` is refused on create and update, `trial_ends_at` on a pending organization is refused, pending → active or suspended passes, and a plan change keeps it pending.
- [x] 5.3 Add the `status` filter to the list (design decision 6). Verify: a handler test for `?status=pending` returns only pending organizations, `?status=waiting` answers `400` with `field: status`, and no parameter returns everything.
- [x] 5.4 Run `make lint` and `make test`. Verify: both pass.

## 6. API docs

- [x] 6.1 Update `signup.md` (201 body, pending, repeated signup), `auth.md` (the `pending_approval` sign-in error for both routes, and the access refusal order), and `organizations.md` (the `pending` status, start-trial with its errors, the list filter, and how to approve a signup). Verify: every status code and body in the four delta specs appears in the docs.

## 7. Frontend

- [x] 7.1 In `lib/signup.ts` and `lib/signIn.ts`, return a "go to pending" outcome for a `201` with status `pending` and for `pending_approval`. Verify with tests in `signup.test.ts` and `signIn.test.ts`, including that `trial_expired` still produces its notice.
- [x] 7.2 Make `app/api/signup/google/route.ts` pass a `201` through without setting a session cookie. Verify: a route test (or a test of the helper it uses) asserts no `Set-Cookie` on a `201` and the cookie still set on a `200`.
- [x] 7.3 Add `app/registro/pendiente/page.tsx` (design decision 7), and send `SignupPageClient` and `SignInForm` there per 7.1. Verify: `pnpm lint`, `pnpm test` and `pnpm build` pass.

## 8. End to end

- [ ] 8.1 On the local rig with `SIGNUP_ENABLED=true`: sign up with Google, land on `/registro/pendiente` with no session; sign in with Google, land there again; call start-trial with the platform key; sign in, land on the home page as a trial user. Verify each step's screen and status code.
- [x] 8.2 Run `openspec validate signup-approval-gate --strict`. Verify: it passes.
- [ ] 8.3 When archiving, rewrite the `## Purpose` of `openspec/specs/self-serve-signup/spec.md` and `openspec/specs/organization-plans/spec.md`: signup creates a pending organization, and the trial starts on approval. Deltas can't carry a Purpose. Verify: neither Purpose still says signup starts a trial.
