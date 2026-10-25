## Context

`UserService.SignIn` resolves the organization first (by slug, or by `scope.Pinned()` on an edge box), scopes the context to it, then looks the user up by username. Users are unique per `(organization_id, username)` (migration 69), are written only in the cloud, and reach edge boxes through the reference pull (`UserSyncPayload`). Since migration 71, schema changes to tables both binaries keep need a twin in `migrations/cloud/` and `migrations/edge/` (`TestEdgeCloudSharedSchemaParity`). The frontend runs one build for both modes and can't tell which it is talking to.

## Goals / Non-Goals

**Goals:**
- Keep one `POST /api/auth/signin` route in both binaries, where the mode decides which credential is required.
- Keep the stopgap for integrations (MCP and agent on email + password) small enough to delete in a few lines when service tokens arrive.

**Non-Goals:**
- Service tokens or API keys for integrations (separate spec).
- Password reset by email, email verification or invitations. The column makes them possible later, but none is built here.
- Multi-organization people or memberships (ruled out: one person, one organization).
- Signing in by email on the edge.

## Decisions

**1. Mode comes from `scope.Pinned()`, not a new flag.** The service already uses it to tell an edge box from the cloud. `SignIn` branches on it: when pinned, use the username and the box's organization, and when not pinned, use the email and look up the user across all organizations. *Alternative:* separate routes (`/auth/cloud/signin`, `/auth/edge/signin`). Rejected because it gives clients two URLs to know about for no behavioral gain.

**2. `SignInRequest` carries `Email` and `Username`, both optional at the binding level.** The service enforces which one is required per mode and returns `400` through the existing validation error. `Organization` is removed. *Alternative:* a single `login` field. Rejected because it's ambiguous and makes the docs and the MCP config harder to read.

**3. Email lookup runs across all organizations.** It's a new port method, `UserRepository.FindByEmail`, that deliberately runs without an org scope (like the slug lookup it replaces). It returns the user with its `organization_id`, after which `SignIn` calls `tenant.WithOrg` and continues exactly as today (load roles, issue the token). A one-line comment on the port records that it is the intentional exception to tenant scoping.

**4. Email storage.** Add a nullable `users.email VARCHAR(255)` with a partial unique index `ON users (lower(email)) WHERE email IS NOT NULL AND deleted_at IS NULL`. The service lowercases and trims before writing and looking up, and the index backs that up. A unique violation maps to a new `ErrUserEmailTaken` (`409`). The migration ships as twins in `cloud/` and `edge/` (same version) to satisfy the parity test. On the edge the column exists but is never written. *Alternative:* cloud-only migration plus a parity exception. Rejected because a nullable, never-written column costs nothing and keeps the parity rule free of exceptions.

**5. Email stays out of sync.** `UserSyncPayload` is unchanged. Edge sign-in never needs the email, and leaving it out keeps personal data off the floor boxes.

**6. Cloud roles are one list in `permissions`.** `CloudRoles = {Admin, Manager, Accountant, Agent}` plus `HasCloudRole(roleIDs)`. Both the sign-in check and the create/update validation ("a cloud role requires an email") use it. `Agent` is in the list only for the integration stopgap and can be removed with the rest of it.

**7. The cloud check runs after the password check.** A floor-only user gets `ErrInvalidCredentials` wrapping a new `ErrCloudAccessDenied`, so the handler's existing opaque `401` path covers it and the log records the real reason. The opaque message is mode-specific ("Invalid email or password" / "Invalid username or password"). The handler picks it from the request field that was used.

**8. `GET /api/auth/context` is shared and backed by the user service** (or a tiny auth-context service). When pinned, it loads the box's organization and returns its name. When not pinned, it returns `{mode: "cloud"}`. It's public, with no rate limiting beyond what sign-in has, and it exposes only the name of the box it is served from.

**9. MCP stopgap.** `mcpserver.Config` gains `Email` from `LAGUNA_EMAIL`. Exactly one of `LAGUNA_EMAIL` (cloud) or `LAGUNA_USERNAME` (edge) is required. `signIn` sends whichever is set. This is the only change in `mcpserver/`, so the in-progress integration work isn't disturbed.

## Risks / Trade-offs

- [Cloud managers or service users without an email are locked out of the cloud at release] → Fill in every cloud-role user's email before deploying (Migration Plan step 1). Floor staff are unaffected because they never used the cloud.
- [An old frontend sends `organization` + `username` to the cloud] → It gets `400`. Ship the frontend in the same release window. The edge ignores unknown fields, so an old frontend on a box keeps working.
- [Email lookup ignores tenant scoping, and the RLS work still planned would hide the row] → When Release 3 (RLS) lands, this lookup needs the same bypass the slug lookup gets. That is noted in the carried-forward RLS work.
- [Email enumeration through timing (unknown email returns fast, wrong password runs bcrypt)] → This already applies to usernames today and isn't made worse. A dummy bcrypt compare on a miss can be added in the same function if wanted.

## Migration Plan

1. Before the cloud deploy: set an email (`PUT /api/admin/users/:id`, or SQL on the new column after the migration) for every user with admin, manager, accountant or agent, including the MCP service user. Set `LAGUNA_EMAIL` wherever the MCP server targets the cloud.
2. Deploy the cloud (migration runs, email sign-in is live) and the frontend together.
3. Update edge boxes on their normal schedule. Until then, an old edge still accepts the old body, and the new frontend's `{username, password}` works on it because the slug was already optional there. An old edge returns `404` for `/api/auth/context`, and the frontend treats that as edge mode with no name (the cloud is always deployed first, so it never returns `404`).
4. Rollback: redeploy the previous cloud. The down migration drops the index and the column. Emails are lost, which is acceptable because they are only re-entered metadata.
