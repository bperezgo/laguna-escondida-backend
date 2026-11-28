## 1. Backend: sync state

- [ ] 1.1 Add `domainError.ErrPullApplyFailed` and wrap every apply-step error in `SyncPullService.PullChanges` with it, leaving the fetch error unwrapped. Test both paths in `sync_pull_service_test.go`
- [ ] 1.2 Add `Tracker.RecordApplyFailure()` and `Tracker.SyncState()` (`""`, `offline`, `failed`, `replaced`, `suspended`), keeping `Online()` and `Blocked()` unchanged. Extend `tracker_test.go`
- [ ] 1.3 In `EdgeSyncScheduler.pull`, call `RecordApplyFailure` for `ErrPullApplyFailed`, otherwise keep the current calls. Extend the scheduler test

## 2. Backend: sign-in context

- [ ] 2.1 Add `UserRepository.AnyActive(ctx)` (tenant-scoped `EXISTS` over active, undeleted users), run `make generate-mocks`, and add a repository integration test (other organization's users don't count)
- [ ] 2.2 Add the `ports.SyncStateReader` port, adapt the tracker to it, and wire it in `bootstrap/edge.go`, not in the cloud
- [ ] 2.3 Extend `dto.SignInContextResponse` with `users_ready` and `sync` (omitted in the cloud). In `UserService.SignInContext`, compute them before the org lookup, memoize the first `true`, and return `200` without `organization` when the org lookup fails. Service tests: cloud unchanged, ready, waiting, offline, failed, blocked, org missing, memoized
- [ ] 2.4 Handler test (negative): with several users the body has only `mode`, `organization`, `users_ready`, `sync`, and no counts or usernames; with an apply failure whose error mentions `users_organization_id_fkey`, the body does not contain that text
- [ ] 2.5 Update `backend/docs/api/auth.md` (Sign-In Context: fields, the waiting example, the missing-org case) and `docs/examples/curl-examples.md`. Run `make lint`, `go build ./...`, `go test ./...`

## 3. Frontend

- [x] 3.1 Extend `SignInContext` and `parseSignInContext` in `lib/signIn.ts` with `users_ready` and `sync` (strict parsing, missing means ready). Add tests in `lib/signIn.test.ts`
- [x] 3.2 Add `BoxWaitingForUsers` (texts per state, reusing `lib/edge/syncBlocked.ts`) and render it from `SignInForm` when `mode === "edge" && users_ready === false`, with 5-second polling of `/api/auth/context`, stop on unmount, and "Iniciar sesión de todas formas". Put the pure state→message logic in `lib/` with tests
- [ ] 3.3 Run `pnpm exec tsc --noEmit`, `pnpm test` and `pnpm lint`. On a local edge run (`make run-edge` with an empty users table), see the waiting screen switch to the form when a user is inserted

## 4. Security review

- [x] 4.1 Run the security-audit skill in guidance mode over this change's diff, against design.md's Security section: what the public context reveals, error-text leakage, unauthenticated polling cost, and the fail-open paths. Fix confirmed findings and record anything that needs validation in design.md
