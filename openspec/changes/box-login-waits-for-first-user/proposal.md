## Why

A freshly installed box shows the sign-in form before any user has reached it from the cloud. Every username and password fails with "Usuario o contraseña incorrectos", and nothing tells the owner whether the box is still waiting, has no internet, or is failing to receive the users. On the owner's own test install, every pull was failing (the box has no row for its organization, so each user insert violates `users_organization_id_fkey`), and the sign-in page looked exactly like a wrong password.

## What Changes

- On an edge box, `GET /api/auth/context` also says whether the box has any user who can sign in (`users_ready`) and, while it has none, the coarse state of its sync with the cloud (`sync`: `waiting`, `offline`, `failed`, `replaced`, `suspended`).
- While `users_ready` is `false`, the sign-in page shows a waiting screen instead of the form: "Esperando a que lleguen los usuarios desde la nube…", with what to check for each sync state. It checks again every few seconds and switches to the form by itself when the first user arrives.
- A pull that reaches the cloud but cannot be applied on the box is reported as `failed`, not as `offline`, so the waiting screen can say "the box could not receive the data" instead of "no internet".
- Once the box has a user, the context reports `users_ready: true` and the form always shows, online or offline. If the context cannot be read, the form shows. The waiting screen also has a link to show the form anyway, so it never locks anyone out.
- A box whose organization row is missing answers the context with `200` and no `organization` instead of `500`.
- Not changing: the cloud's response, the sign-in endpoint, and `GET /api/edge/status`.

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `organization-sign-in`: the public sign-in context gains the box's first-user readiness and coarse sync state, and the sign-in page waits for the first user.

## Impact

- `backend/`: the sign-in context service and handler, the edge sync scheduler and status tracker (failure kind), the pull service (marks apply failures), a user repository `EXISTS` query. Docs: `docs/api/auth.md` (Sign-In Context).
- `frontend/`: `lib/signIn.ts` (parsing), `components/auth/SignInForm.tsx` (waiting screen and polling).
- Depends on PR #88, which ensures the box's organizations row at edge-node startup (in `bootstrap.Main`, after migrations). With #88, a new box gets its users and `/api/auth/context` no longer returns `500`. Without it the box still never gets users; with this change the page says the pull is failing instead of looking like a wrong password.
- Trust boundary: this touches a **public, unauthenticated endpoint** on the box (LAN-reachable through Caddy). It adds a boolean and a five-value enum, and no counts, usernames, error text or cloud details. It touches no auth decision: sign-in itself is unchanged.
