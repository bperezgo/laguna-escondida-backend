## Context

See proposal.md for why and the specs for every requirement. Today's code that shapes the approach:

- **One door for authenticated cloud requests.** `JWTAuthMiddleware` (`platform/handler/middleware.go`) is the single entry for user JWTs and organization API keys (`lgk_…`). It is attached per route, but every protected route uses it. It puts the organization in the request context through `tenant.WithOrg`.
- **Sign-in.** `UserService.SignIn` → `cloudSignInUser` finds the user by email and already refuses a suspended organization with the opaque `401`.
- **Box auth.** `NodeService.Authenticate` refuses `organization_suspended` from `NodeAuthRecord.OrganizationStatus`. `expired-box-backoff` caches that refusal for 5 minutes.
- **Passwords.** `users.password` is `NOT NULL`, in a shared migration, so it exists on the edge too. Users are pulled to the box.
- **Migrations.** These live in `migrations/shared`, `/cloud` and `/edge`. Organization columns added since `000075` are cloud-only.
- **Patterns to reuse.**
  - `KeyValueCache` with a decorator (`platform/cache`, 30s TTL for organization settings).
  - `RateLimitMiddleware(limit, window)`, per client IP.
  - API keys: a SHA-256 token hash, with the plaintext shown once.
- **Invoices.** `PendingInvoiceSyncApplier` creates the `pending_invoice` row when a box pushes one, and the cloud cron submits rows with `status = pending`.
- **Box enrollment.** The `edge-installer` change already ships `edge/scripts/enroll.ps1`, which signs in with email and password, then calls `POST /api/edge/enroll`.
- **Frontend.** The frontend talks to the backend through its own Next.js route handlers (`frontend/app/api/**`). `app/api/auth/signin/route.ts` receives the token and sets the session.
- **Google.** No Google or OIDC library is in `go.mod`. `golang-jwt/jwt/v5` is.

## Goals / Non-Goals

**Goals:**
- Exactly one place decides "may this organization make this request".
- Signup can't half-create an account.
- No new infrastructure other than a Google OAuth client.
- Signup can be switched off without a deploy.

**Non-Goals:**
- A Redis or shared cache: per-replica 30s caching meets the spec's 60s.
- Account linking UI.
- Changing the edge's sign-in. Its schema changes only by the `organizations` twin columns that schema parity requires; the box never writes or reads them.
- Billing state beyond `status`, `plan` and `trial_ends_at`.

## Decisions

### 1. Organization access is checked inside `JWTAuthMiddleware`, against one route table

Every authenticated cloud route already passes through that middleware. After it has a principal, it calls an `OrganizationAccessService` (domain) with the org id, the principal's kind (user or API key), its roles, and the matched route (`c.FullPath()`).

**Checks, in order:**
1. The organization is suspended: `403 organization_suspended`.
2. The trial has expired (status `trial` and now ≥ `trial_ends_at`): `403 trial_expired`.
3. The route's feature isn't in the plan, or the request is from an API key or the agent role on a plan without agents: `403 plan_required` with `required_plan`.

**The route table.** `domain/plans` holds the spec's table: route prefix → feature, and plan → features. That keeps one source that the spec table and the tests both point at. Routes not in the table are core.

**Mode.** On the edge the service is absent and the middleware skips the check (plans are cloud-only).

*Alternatives:*
- A `RequirePlan` middleware on each route group: easy to forget on a new route, and the agent/API-key rule cuts across groups.
- Plan in the JWT: stale for up to 7 days, and API keys have no token.

### 2. Access data cached 30s per replica

`OrganizationAccessReader` (port) returns `{status, plan, trial_ends_at}`. A `platform/cache` decorator on the in-memory `KeyValueCache` caches it, with a 30s TTL. This is the same pattern and TTL as the organization settings. It reads the organization by id, without a tenant in the context, just as `FindForAuth` does. Each authenticated request costs a map lookup. A cache error falls through to the database.

### 3. Expiry is computed, never written

"Expired" is a pure function `plans.Expired(status, trialEndsAt, now)` in the domain, used in four places:
- the access service
- sign-in
- `NodeService.Authenticate`: `NodeAuthRecord` gains `TrialEndsAt`, and an expired trial maps to `ErrNodeOrganizationSuspended`, so the box gets exactly the suspended refusal and its backoff
- enrollment-code issue and use

There's no cron and no status flip, so the operator extending `trial_ends_at` takes effect within the cache windows: 30s for users, 5 min for boxes.

### 4. Google ID tokens verified with `coreos/go-oidc/v3`

- **Port.** `GoogleIdentityVerifier.Verify(ctx, idToken) (GoogleIdentity{Email, Name, Subject}, error)`.
- **Adapter.** `platform/googleauth` uses go-oidc's provider for `https://accounts.google.com`. It checks the signature against Google's JWKS (fetched and rotated by the library), the issuer, `aud == GOOGLE_CLIENT_ID` and expiry, and requires `email_verified`.
- **Email.** Normalized with the existing `user.NormalizeEmail`. Accounts are matched by email; `sub` is not stored.

The library is small and standard, and handles key rotation.

*Alternatives:*
- `google.golang.org/api/idtoken`: a much larger dependency tree.
- Hand-rolled JWKS over `golang-jwt`: key-rotation code we'd own.

The browser gets the ID token from Google Identity Services, so the backend never handles an OAuth code or client secret.

### 5. Google-only users get an unusable random password hash

`users.password` stays `NOT NULL`. Google signups and Google-accepted invitations store the hash of 32 random bytes that are thrown away, so password sign-in can never succeed for them. An admin can still set one later with the existing reset route.

This avoids a nullable password in a shared migration and in every edge sign-in path.

*Alternative:* a nullable column. That touches the edge's schema and sign-in for no user-visible gain.

### 6. Signup is one transaction, made safe against races by unique constraints

`SignupService.SignUpWithGoogle`:
1. Verify the token.
2. If a user has the email, return `UserService` Google sign-in.
3. Otherwise, in one unit of work, create the organization, then, with `tenant.WithOrg(newOrgID)`, the admin, the box user and their roles.

**Slug.** A small `SlugFromName` helper next to the signup service (lowercase, strip accents with `golang.org/x/text/unicode/norm`, collapse to `-`, cut to 40). A unique violation retries with `-2`…`-20` inside the same call, then fails `500`.

**Race on the email.** If a concurrent signup takes the email (unique index violation on email), the transaction rolls back and the call is answered as a sign-in.

**Box user.** Username from the email's local part, per the spec. It can't clash, because the organization is new and the admin's username is the full email.

### 7. Invitations and enrollment codes: hashed tokens, consumed under row locks

| | Invitation | Enrollment code |
|---|---|---|
| Secret | 32 random bytes, base64url, in `link` = `PUBLIC_APP_URL` + `/invitacion/<token>` | 8 characters from a 31-character alphabet with no look-alikes (~40 bits) |
| Stored | SHA-256 of the token | SHA-256 of the normalized code |
| Table (cloud migration) | `invitations(id, organization_id, email, name, role_ids int[], token_hash unique, invited_by, expires_at, accepted_at, revoked_at, created_at)` with a partial unique index on `(organization_id, lower(email))` where neither accepted nor revoked | `edge_enrollment_codes(id, organization_id, code_hash unique, created_by, expires_at, used_at, created_at)` |
| Consume | `SELECT … FOR UPDATE` by hash, check validity, create the user, set `accepted_at`, all in one transaction | Same, and run the existing `NodeService.Enroll` for that organization in the same transaction. `used_at` is set only when it returns success, so a `409 box_already_enrolled` rolls back and leaves the code usable |
| Renew | `POST …/link` replaces `token_hash` and `expires_at` | Issuing a new code sets `used_at` on the organization's unused ones |

~40 bits is weak against offline guessing if the database leaks, but a code lives 30 minutes and the route is rate-limited. The invitation token gets full strength because it lives 7 days.

### 8. Bills of plans without e-invoicing are marked `skipped` on receipt

`PendingInvoiceSyncApplier` asks the access reader for the organization's plan (decision 2) and inserts the row with status `skipped` instead of `pending` when `plans.Includes(plan, EInvoicing)` is false. The submit cron already selects only `pending`, so it needs no change. `skipped` is a new `PendingInvoiceStatus`. The column has a check constraint, so the migration widens `pending_invoices_status_check` in both sets (the table is on both sides, for schema parity).

Deciding at receipt, not at submission, is what makes "never submitted after an upgrade" hold without extra state.

### 9. Signup is behind a switch, and the Google routes need the client id

- `SIGNUP_ENABLED` (default `false`). While false, `POST /api/signup/google` is not registered, so it answers `404`.
- `GOOGLE_CLIENT_ID`. When it's empty, the signup and `POST /api/auth/google` routes are not registered, and the invitation-accept route refuses Google tokens with `400`.
- `PUBLIC_APP_URL` (required in the cloud) builds invitation links.

The cloud can therefore ship before the landing links to signup, and Laguna is unaffected.

### 10. The installer swaps the password for the code

In `edge/scripts/enroll.ps1`:
- the input JSON becomes `{"code","replace","name","cloud_url"}`;
- it calls `POST /api/edge/enroll-with-code` directly, dropping the sign-in step;
- its stdout contract and exit codes stay as they are, except that "bad credentials" now means "invalid or expired code".

The Inno Setup wizard page (`edge/installer/kuenta.iss`) asks for the code and the box name, with the hint "Encuéntralo en la nube: Instalar caja".

### 11. Frontend

**Sign-in with Google.** Google Identity Services (`https://accounts.google.com/gsi/client`) renders the button and returns an ID token. The page posts it to new Next route handlers (`app/api/auth/google`, `app/api/signup/google`, `app/api/invitations/[token]/accept`). These proxy to the backend and set the session exactly like `app/api/auth/signin/route.ts`. `NEXT_PUBLIC_GOOGLE_CLIENT_ID` configures the button.

**Pages:**
- `/registro`: restaurant name, plan cards, Google button.
- `/invitacion/[token]`: shows the organization and role, then a password or Google.
- `/ajustes/instalar-caja`: issue a code (copy button, countdown), "Generar contraseña" for the box user shown once, and the installer download link (the landing's download endpoint).
- Invitations in `/users`: invite form, copy link, new link, revoke, pending list.

**Plan-aware navigation.** `/auth/me`'s `organization.plan` hides menu entries the plan excludes, and a `plan_required` response shows an upgrade message.

**Trial banner.** It shows the days left while in trial.

**Expired trial.** A `403 trial_expired` at sign-in shows "Tu prueba terminó. Escríbenos para continuar". The contact email comes from `NEXT_PUBLIC_SUPPORT_EMAIL`, used both here and in the `plan_required` upgrade message:
- When it's set, the message adds a `mailto:` link.
- When it's unset or empty, the message shows without a link. The app never fails to build or render because of it.

### 12. Google Cloud setup (operator, once)

1. **Create a project.** In Google Cloud Console, create the project (e.g. `kuenta-auth`).
2. **Set up the consent screen** (APIs & Services → OAuth consent screen):
   - User type **External**, with app name, support email, and the app and landing domains.
   - Scopes `openid`, `email` and `profile` only. These are non-sensitive, so no Google verification review is needed.
   - **Publish** the app (move it from Testing to In production), or only listed test users can sign in.
3. **Create the client ID** (Credentials → Create credentials → OAuth client ID → **Web application**):
   - Authorized JavaScript origins: the cloud app's public origin and `http://localhost:3000` for development.
   - No redirect URIs are needed with the ID-token button flow.
4. **Configure it.** Put the client ID in the cloud's `GOOGLE_CLIENT_ID` and the frontend's `NEXT_PUBLIC_GOOGLE_CLIENT_ID`. There's no client secret to store.

## Risks / Trade-offs

- **The edge schema gets the plan columns only as a twin.** `TestEdgeCloudSharedSchemaParity` requires `organizations` to match, so an edge migration adds `plan`, `trial_ends_at` and the widened status checks, as was done for `settings`. The box never writes them, and the access reader and the `plans` code are cloud-only. Mitigation: the access service is wired only in `cloud.go`, and an edge boot acceptance test (`test/acceptance/edgeboot`) runs against the edge schema.
- **A missed route stays ungated.** Mitigation: a bootstrap test walks `router.Routes()` like `agent_routes_test.go`. It asserts that each table prefix matches at least one route, and that a `starter` token is refused on every non-core route and allowed on every core one.
- **The 30s cache means a just-suspended user keeps access for up to 30s per replica.** That's within the spec's 60s and accepted.
- **Google is the only signup path.** People without a Google account can't sign up. Accepted for now. Password signup comes with SES later.
- **The `edge-installer` change is in flight and ships `enroll.ps1` with the password flow.** Mitigation: if it merges first, this change edits its script. If not, update its tasks to target the code flow. Either way, only one enrollment flow ships to customers.
- **An admin who mistypes an invitee's email**, with the invitee accepting by password, leaves a wrong email on the account. The admin can fix it through the existing user update.

## Migration Plan

1. **Cloud migration.**
   - `organizations` gains `plan TEXT NOT NULL DEFAULT 'full'` with a check, and `trial_ends_at TIMESTAMPTZ NULL`.
   - The status check widens to `trial | active | suspended`, with a check that `trial` needs `trial_ends_at`.
   - The `invitations` and `edge_enrollment_codes` tables are created.
   - `pending_invoices_status_check` accepts `skipped` (cloud and edge).
   - Laguna ends up `active` + `full`.
   - The edge set gets a twin for the `organizations` columns and checks only (schema parity).
   - The down migration drops the tables and columns. It refuses while any organization is in `trial` or any pending invoice is `skipped`.
2. **Deploy the cloud** with `SIGNUP_ENABLED=false`. Laguna is unaffected.
3. **Deploy the frontend.**
4. **Ship the edge release** with the code-based installer. This comes after `expired-box-backoff` is released and `MIN_EDGE_VERSION` raised.
5. **Open signup.** Create the Google client (decision 12) and set `SIGNUP_ENABLED=true`, then point the landing's "Prueba 14 días" at `/registro`.

Rollback: set `SIGNUP_ENABLED=false`. Organizations created meanwhile stay, and the operator suspends them if needed.

## Implementation map

- **Domain**:
  - New package `domain/plans` (features, route table, `Includes`, `Expired`) + tests
  - `domain/dto/organization.go`, `domain/dto/user.go`, new `domain/dto/invitation.go` and `domain/dto/enrollment_code.go`, `domain/dto/pending_invoice.go` (`skipped`)
  - Ports: `organization_access_reader.go`, `google_identity_verifier.go`, `invitation_repository.go`, `enrollment_code_repository.go`; additions to the organization, user and node repository ports
  - Services, each with a `_test.go`: new `organization_access_service.go`, `signup_service.go`, `invitation_service.go`, `enrollment_code_service.go`; `user_service.go` (Google sign-in, `trial_expired`, `GeneratePassword`, the email rule for managers); `node_service.go` (expiry); `organization_service.go` (plan and trial fields)
  - `domain/error/` (`trial_expired`, `plan_required`, invitation and code errors)
- **Platform**:
  - New `platform/googleauth/`
  - `platform/cache/organization_access_repository.go`
  - Repositories: `organization`, `user`, `node` (`TrialEndsAt`), new `invitation` and `enrollment_code`, `pending_invoice_sync_applier.go`
  - Handlers: `middleware.go` (access check), new `signup_handler.go`, `invitation_handler.go`, `enrollment_code_handler.go`; `user_handler.go`, `user_signin_handler` (Google, `403`), `organization_handler.go`
  - `platform/config/cloud.go` (`SIGNUP_ENABLED`, `GOOGLE_CLIENT_ID`, `PUBLIC_APP_URL`)
  - The cloud migration
- **Bootstrap**: `cloud.go`, `routes_cloud.go` (new routes and rate limits), a new plan-gating route test
- **Edge appliance**: `edge/scripts/enroll.ps1`, `edge/installer/kuenta.iss`
- **Frontend**:
  - `app/registro/`, `app/invitacion/[token]/`, `app/(main)/ajustes/instalar-caja/`, `app/(main)/users/` (invitations)
  - The `app/api/` route handlers listed in decision 11
  - The Google button component, plan-aware nav, the trial banner, and the expired message on `app/signin/`
- **Landing**: the "Prueba 14 días" call to action
- **Docs**:
  - `docs/api/auth.md`, `organizations.md`, `edge-enrollment.md`, new `signup.md` and `invitations.md`
  - `docs/examples/curl-examples.md`
  - Edge runbook (the code-based installer)
