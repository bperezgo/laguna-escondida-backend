## Context

The box's sign-in page (`frontend/app/signin/page.tsx` → `components/auth/SignInForm.tsx`) reads `GET /api/auth/context` (`UserHandler.SignInContextHandler` → `UserService.SignInContext`, `backend/internal/domain/service/user_service.go:559`) to choose the cloud or edge form. Users reach the box only through the cloud pull. `EdgeSyncScheduler.pull` (`backend/internal/platform/cron/edge_sync_scheduler.go:241`) runs `SyncPullService.PullChanges` every minute and records the outcome in `syncstatus.Tracker`, which has only two failure outcomes:
- `RecordFailure`, for any error
- `RecordBlocked`, for a `node_revoked` or `organization_suspended` refusal

So a pull that reached the cloud but could not be applied on the box looks the same as no internet.

`PullChanges` (`sync_pull_service.go:46`) has a clear seam. The fetch fails with `"pull from cloud: %w"`, while the upserts fail inside the apply transaction (`"upsert users: %w"`, …). On the owner's test box every pull failed in that apply step on `users_organization_id_fkey`, because the box has no row for its organization. That bug is fixed in a separate change. The same missing row also makes `SignInContext` return `500` today, since it loads the organization first.

`GET /api/edge/status` is also public. It already reports `online` and `sync_blocked`, but it is not on the sign-in page's path, and it carries outbox counts that the sign-in page does not need.

## Goals / Non-Goals

**Goals:**
- A fresh box never shows a sign-in form that cannot work. It shows that it is waiting, and why it may be stuck.
- The form appears by itself when the first user lands.
- Once a box has users, nothing about sync can keep the form from showing.

**Non-Goals:**
- Showing progress after the first user: later users and password changes keep arriving silently.
- Fixing the missing organization row (separate change).
- Telling the cloud's Instalar caja page that the box got its users. That is a possible follow-up.
- Changing `GET /api/edge/status` or the sign-in endpoint.

## Decisions

### D1. Extend `GET /api/auth/context`, not `/api/edge/status`

The sign-in page already fetches the context on the server and again on the client. Adding `users_ready` and `sync` there means no second request, and the page renders the waiting screen on first paint. `/api/edge/status` stays as it is.

*Alternative: have the sign-in page also call `/api/edge/status`.* Rejected: it is a second request, its `online` flag cannot tell `failed` from `offline`, and it exposes outbox numbers the page doesn't need.

### D2. `users_ready` comes from the box's database and stays true once true

`users_ready` is computed by `UserRepository.AnyActive(ctx)`: `SELECT EXISTS (… FROM users WHERE organization_id = <box org> AND active AND deleted_at IS NULL)`, through the tenant-scoped DB. On the box every user is a pulled staff user. The cloud-only `api_key_managed` column does not exist there.

`UserService` keeps the first `true` in memory (an `atomic.Bool`) and stops querying after it. A box that has had a user keeps answering `true` until the process restarts, even if every user is later deactivated. That is acceptable: it then shows the form, as it does today. While the result is `false`, each call runs one indexed `EXISTS`.

*Alternative: count users.* Rejected: a count is more than the page needs and more than an unauthenticated caller should learn.

### D3. The tracker learns a failure kind

- `PullChanges` wraps apply-step errors with a new domain sentinel, `domainError.ErrPullApplyFailed` (`fmt.Errorf("%w: upsert users: %w", ErrPullApplyFailed, err)`). The fetch error stays as it is.
- The scheduler calls a new `Tracker.RecordApplyFailure()` when `errors.Is(err, ErrPullApplyFailed)`, and otherwise `RecordFailure()` as today.
- A new `Tracker.SyncState()` returns:
  - `""` before the first attempt, or after a success (reported as `waiting`)
  - `offline` or `failed`
  - the `Blocked` value
- `Online()` and `Blocked()` are unchanged, so `/api/edge/status` is unaffected. `RecordApplyFailure` still counts as not online.

The service gets the tracker's state through a small port (`ports.SyncStateReader`). The domain does not import `platform/syncstatus`. A cloud has no reader and never pins, so it returns `{"mode":"cloud"}` exactly as now.

### D4. A missing organization no longer fails the context

`SignInContext` computes `users_ready` and `sync` first. If loading the organization fails, it logs the error and returns them with no `organization`. The page then still shows the waiting screen with `failed`, instead of falling back to a form that cannot work. A box without its organization row cannot be pulling successfully, so `sync` will be `failed`.

### D5. The page waits, polls, and fails open

- `parseSignInContext` keeps `users_ready` only when it is a boolean, and `sync` only when it is one of the five values (anything else is treated as `waiting`). A missing `users_ready` means ready, so a cloud or an older box keeps the form.
- `SignInForm` renders `BoxWaitingForUsers` when `mode === "edge" && users_ready === false`. It polls `/api/auth/context` every 5 seconds while that screen is mounted and stops on unmount.
- A failed poll keeps the current screen. The waiting screen has "Iniciar sesión de todas formas", which sets a local `forceForm` flag.
- Texts (es-CO):
  - `waiting`: "Los usuarios llegan en uno o dos minutos. Si tarda más, revisa en la nube, en Usuarios, que exista el usuario de la caja."
  - `offline`: "La caja no tiene conexión a internet. Revisa la red; los usuarios llegarán cuando vuelva."
  - `failed`: "La caja no pudo recibir los datos de la nube. Contacta a soporte."
  - `replaced` and `suspended`: the existing `lib/edge/syncBlocked.ts` messages

## Security

**Boundary: an unauthenticated caller on the box's LAN → `GET /api/auth/context` on the box.**
- *Lower-trust principal:* anyone who can reach the box over the LAN (Caddy serves tablets) or from the box PC, with no token.
- *Input they control:* only that they call the endpoint, and how often. It takes no parameters.
- *What it now reveals:* `users_ready` (whether the box has any active user) and, only while it has none, a five-value `sync` enum. The `sync_blocked` part is already public on `/api/edge/status`. No counts, usernames, names, roles, error text, constraint or table names, cloud URL or timestamps (enforced in `UserService.SignInContext` and the response DTO `dto.SignInContextResponse`).
- *Protected resource:* the box's user directory, and the confidentiality of internal error details.
- *If the control fails:* raw error text from a failed pull (such as a constraint or table name) would leak schema details to the LAN. A count or a list of usernames would help an attacker guess passwords. Specs cover both with negative scenarios.
- *Attack classes considered* (`WEB-PROTOCOL-AND-AUTH.md`, `RESOURCE-EXHAUSTION-AND-AVAILABILITY.md`, `CLIENT-SIDE.md`):
  - account and user enumeration on a public endpoint: only a boolean, which on a fresh box says nobody can sign in yet
  - information disclosure through error messages: only the enum leaves the box
  - resource exhaustion from unauthenticated polling: one indexed `EXISTS` per call while not ready, none after (D2). The page polls at 5 seconds, and the cost per call is lower than the existing org lookup.
  - an availability lockout, where a wrong or forged `users_ready: false` keeps staff out: the value comes from the box's own DB, the page fails open on errors, and "Iniciar sesión de todas formas" always shows the form
  - client-side trust: the waiting screen is cosmetic. Sign-in authorization is unchanged and stays server-side (`POST /api/auth/signin`).

**Not touched:** auth decisions, tokens, tenancy (the query uses the tenant-scoped DB for the box's pinned org), and the edge↔cloud protocol. The pull's apply errors are only reclassified, not changed.

**Needs validation:** none outside the repo.

## Risks / Trade-offs

- [A box whose users were all deactivated in the cloud while it was running still reports `users_ready: true`] → it shows the form, as today. A restart re-evaluates.
- [Reclassifying apply errors could hide a real network failure as `failed`] → only errors returned from inside the apply transaction are wrapped. The fetch error path is unchanged, and a unit test covers each path.
- [Polling from a tablet left open on the waiting screen] → 5-second interval, negligible cost, and it stops as soon as users arrive.

## Migration Plan

No data migration. The cloud response is unchanged. An older box keeps answering without `users_ready`, and the page shows the form. Deploy as part of the next edge release; rollback is the previous release.

## Open Questions

- Should `failed` point to the logs folder (`C:\kuentapos\logs`) as well as support? It's left as "contact support" for now, since the owner is not expected to read logs.
