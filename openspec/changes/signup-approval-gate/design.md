## Context

See `proposal.md` for the motivation. The current state that shapes the approach:

- **Status lives in three places.** There is the cloud constraint `organizations_status_check` (migration `000096`: `trial | active | suspended`, and `trial` needs `trial_ends_at`), the constants in `dto/organization.go`, and `plans.StatusTrial`.
- **One service decides cloud access.** `OrganizationAccessService.Check` refuses suspended, then expired, then plan, and the JWT and API-key middleware turn each error into a code in `abortOrganizationAccess`. `Usable` guards invitations and enrollment codes. Both read through the access cache, which is why changes take effect within 60 seconds.
- **Sign-in has two stops.** A suspended organization fails early and opaquely (`user_service.go`, `cloudSignInUser`). An expired trial fails late, after the password and role checks, in `completeSignIn`, so that only a person who proved who they are learns the reason. Signup reuses Google sign-in for an existing email.
- **Box auth** (`node_service.go`) maps an expired trial to `suspended` before the edge sees the status. The edge's own constraint (`000086`) has no `pending`.
- **Signup returns a session today.** `SignupService.create` builds the organization as `trial` and issues a token. The Next.js route `app/api/signup/google` passes the backend's answer to `backendSessionResponse`, which sets the cookie. `SignupPageClient` then goes to the app, and `signupFailure` and `signInFailure` already branch on `trial_expired`.
- **The platform list** (`GET /api/platform/organizations`) takes no filter.

## Goals / Non-Goals

**Goals:**
- `pending` reuses the existing access path: one new error, one new code, no second gate.
- Starting a trial is a single service method with no HTTP in it, so a payment webhook can call it later.
- No edge release and no change to the edge's data.

**Non-Goals:**
- A platform UI. The operator uses the API, as for every other platform action today.
- Expiring or cleaning up pending signups that are never approved.
- Telling the restaurant when it is approved. They find out by signing in.

## Decisions

### 1. `pending` is a status, not a flag

A fourth value of `status`, with the constraint extended to `status <> 'pending' OR trial_ends_at IS NULL`. Everything that already reads the status (access cache, node auth, `/auth/me`) carries it with no new column.

*Alternative considered:* an `approved_at` column next to `trial`. Rejected: every check would need two fields, and "trial without a clock" breaks the rule that `trial` has an end.

### 2. Refusal order and codes

`Check` becomes: suspended → pending (`ErrOrganizationPending`) → expired → plan. `abortOrganizationAccess` maps it to `403 {"error": "Approval pending", "code": "pending_approval"}`. An API key never gets that far: key validation already refuses a key of any organization that isn't `active` with the opaque `401`, so a pending organization's key is refused as a suspended one's is. `Usable` returns `ErrOrganizationInactive` for pending too, which covers invitations and enrollment codes with no handler change.

In sign-in, `completeSignIn` checks pending right before the expiry check, and `SignInWithGoogle` passes `ErrOrganizationPending` through like `ErrTrialExpired`. `UserHandler.SignInHandler` and `SignupHandler.respondGoogleError` gain the same case through a `respondPendingApproval` helper next to `respondTrialExpired`.

`node_service.go` maps `pending` to `suspended` along with an expired trial, so the edge never receives a status its constraint lacks. It can't happen today, since a pending organization has no enrollment path, but the cost is one condition.

### 3. Signup answers `201` without a token

`SignupService.create` stores `status: pending`, `trial_ends_at: nil`, and returns a result with `Created: true` and no `SignIn`. The handler writes `201 {"status": "pending", "organization_name", "plan"}`. Nothing else in the transaction changes: the admin and box user are still created, so approval needs no data from the person.

In the frontend, `app/api/signup/google/route.ts` stops using `backendSessionResponse` for a `201` and passes the body through without setting a cookie. Other statuses go through as before, so a `200` from an existing active user still signs them in. `SignupPageClient` goes to `/registro/pendiente` on a `201`, and on `pending_approval` from signup or sign-in. Both branches live in pure functions in `lib/signup.ts` and `lib/signIn.ts` (a `next` field, `"pending"` or the existing failure), so they are unit-tested.

*Alternative considered:* answering `403 pending_approval` to the first signup too. Rejected: a `201` says that something was created, and it is what the payment change will want to see.

### 4. `StartTrial` on `OrganizationService`

```go
func (s *OrganizationService) StartTrial(ctx context.Context, id string) (*dto.Organization, error)
```

It reads the organization, returns `ErrOrganizationNotPending` unless the status is `pending`, and writes `status: trial, trial_ends_at: now + plans.TrialLength` with a conditional update (`WHERE status = 'pending'`), so two concurrent calls can't both start it. It uses the service's injected clock. The handler maps not-pending to `409 not_pending`. The route is `POST /api/platform/organizations/:id/start-trial` under the existing platform group and its admin API key.

The future payment change calls `StartTrial` from its webhook handler. Nothing in this method knows who called it.

*Alternative considered:* `PATCH {"status": "trial"}` computing the end itself. Rejected: it hides a clock rule inside a generic update, and a webhook would have to build a PATCH body.

### 5. Update validation for `pending`

`validateOrganizationStatus` stops accepting `pending` from requests (create and update). `Update` reads the current status (it already does, for the trial-end rule). When the current status is `pending`, it refuses `status: trial` and any `trial_ends_at` with a validation error on that field, and it allows `active`, `suspended` and changes to other fields, the plan included.

### 6. List filter

`GET /api/platform/organizations?status=` is validated against the four statuses and passed to `orgRepo.List` as an optional filter. Without the parameter, the result is the same as today.

### 7. The pending page

`app/registro/pendiente/page.tsx` is a static page inside `PublicShell`, like `/registro`. It shows the "Solicitud recibida" text, the support contact from `supportEmail()` when one is set, and a link to `/signin`. It doesn't read the query string or a session, so there is nothing to leak or forge.

## Risks / Trade-offs

- **The signup response changes shape** → The backend and frontend ship in one PR and one `cloud/v*` release. The web image and the API deploy together.
- **Pending signups pile up and hold slugs** → Acceptable at this volume. The operator suspends the ones that won't pay, and the slug stays taken as it would for any organization.
- **Nobody is told about a new signup** → The operator checks `?status=pending`. The existing `organization signed up` log line can drive a Grafana alert without code if that becomes a problem.
- **`self-serve-signup-trial` is not archived** → These deltas modify requirements that exist only in that change. Archive it first, or `openspec archive` of this change fails on missing requirements.

## Migration Plan

1. Cloud migration `000097`: widen `organizations_status_check` to include `pending` and add the pending-has-no-trial-end check. The down migration moves any `pending` rows to `suspended`, then restores the old constraints.
2. Ship as a `cloud/v*` release, before or with the `landing/v*` release of `landing-pricing-and-product-proof`.
3. Rollback: redeploy the previous cloud release and run the down migration. Pending signups become suspended, and the operator can reactivate them by hand.

## Implementation map

- **Migration**: `backend/internal/platform/postgres/migrations/cloud/000097_add_organization_pending_status.{up,down}.sql`
- **Domain**: `internal/domain/dto/organization.go`, `internal/domain/plans/plans.go`, `internal/domain/error/access_error.go`, `internal/domain/service/organization_access_service.go`, `organization_service.go` (+ `StartTrial`, validation, list filter), `signup_service.go`, `user_service.go`, `node_service.go`, `internal/domain/ports` (repository list filter, conditional update)
- **Adapters**: `internal/platform/postgres/repository/organization_repository.go`, `internal/platform/handler/middleware.go`, `user_handler.go`, `signup_handler.go`, `organization_handler.go`, `internal/bootstrap/routes_cloud.go` + `testdata/routes_cloud.golden`
- **Tests**: next to each file above. Acceptance coverage goes where `organization_handler_plans_test.go` and `jwt_auth_organization_access_test.go` already exercise trials.
- **API docs**: `backend/docs/api/signup.md`, `auth.md` (sign-in errors, access refusals), `organizations.md` (status, start-trial, list filter)
- **Frontend**: `frontend/app/api/signup/google/route.ts`, `frontend/lib/signup.ts` + test, `frontend/lib/signIn.ts` + test, `frontend/components/signup/SignupPageClient.tsx`, `frontend/components/auth/SignInForm.tsx`, new `frontend/app/registro/pendiente/page.tsx`
## Open Questions

- The exact Spanish wording of the pending page. It can be adjusted in review without changing the spec.
