## 1. Data and ports

- [x] 1.1 Add twin migrations `000073_add_email_to_users` in `migrations/cloud/` and `migrations/edge/`: a nullable `users.email VARCHAR(255)` and a partial unique index on `lower(email)` where `email IS NOT NULL AND deleted_at IS NULL`, with down migrations. Verify `make migrate-up/down/up` run clean and `TestEdgeCloudSharedSchemaParity` passes.
- [x] 1.2 Add `Email` to `dto.User`, `CreateUserRequest` and `UpdateUserRequest` (`omitempty,email,max=255`). Replace `Organization`/`Username` in `SignInRequest` with optional `Email` and `Username`. Add `ErrUserEmailTaken`, `ErrEmailRequired` and `ErrCloudAccessDenied` in `domain/error`. Verify `go build ./...`.
- [x] 1.3 Add `CloudRoles` and `HasCloudRole(roleIDs []int) bool` in `domain/permissions`. Verify with a `permissions` unit test covering each role.
- [x] 1.4 Add `FindByEmail(ctx, email) (*dto.User + organization id)` to the user repository port, with a one-line comment that it deliberately ignores org scope. Run `make generate-mocks` and verify the mocks compile.

## 2. Service

- [x] 2.1 `UserService.SignIn`: when pinned, require `username` and use the box's organization (drop the slug path). When not pinned, require `email`, normalize it, `FindByEmail`, check the organization is active, `tenant.WithOrg`, compare the password, check active, then require `HasCloudRole`. Verify `TestUserService_SignIn_*` covers every scenario in `specs/organization-sign-in` (**ask before editing existing sign-in tests**).
- [x] 2.2 Create and update user: normalize the email, reject a cloud role without an email (`ErrEmailRequired`), and map a unique violation to `ErrUserEmailTaken`. On update, check the resulting role set together with the resulting email. Verify the service tests for create/update with and without an email, a floor-only user, a duplicate in another organization and mixed case.
- [x] 2.3 Add the sign-in context use case: pinned returns `{mode: "edge", organization: {name}}`, not pinned returns `{mode: "cloud"}`. Verify the service tests for both modes.

## 3. Adapters

- [x] 3.1 User repository: persist and read `email`, implement `FindByEmail` without the org scope, and map the `lower(email)` unique violation to `ErrUserEmailTaken`. Leave `UserSyncPayload` and the sync repository unchanged. Verify the repository integration tests for the lookup, the case-insensitive duplicate and that a pulled user carries no email.
- [x] 3.2 Handlers: `SignInHandler` returns `400` for a missing credential and a mode-specific opaque `401` body. Map `ErrEmailRequired` to `400` and `ErrUserEmailTaken` to `409` in the user handlers. Add a public `GET /api/auth/context` in `routes_shared.go`. Verify the handler/route tests and `curl` against local cloud and edge binaries.
- [x] 3.3 MCP stopgap: `LAGUNA_EMAIL` in `mcpserver.Config` (exactly one of email or username is required), and `signIn` sends whichever is set. Touch nothing else in `mcpserver/`. Verify `go test ./internal/platform/mcpserver/...` and a sign-in against the local cloud.

## 4. Docs

- [x] 4.1 Update `docs/api/auth.md` (sign-in bodies per mode, error bodies, the new `/api/auth/context`), `docs/api/users.md` (`email` field, `400`/`409`) and `docs/examples/curl-examples.md`. Verify each curl example works against a local server.

## 5. Frontend (`laguna-escondida-frontend`)

- [x] 5.1 On load, `SignInForm` calls `GET /api/auth/context` (through a Next route). `cloud` shows "Correo" + "Contraseña". `edge` shows the restaurant name, "Usuario" and "Contraseña". A `404` falls back to edge mode with no name. Remove the "Restaurante" field. Verify signing in to the local cloud with an email and to a local edge with a username.
- [x] 5.2 `app/api/auth/signin/route.ts` forwards `{email, password}` or `{username, password}`. Delete `lib/signinOrganization.ts`, its test and the remembered-organization storage key. Verify `npm test` and the build pass.
- [x] 5.3 The user admin screens (create/edit) gain an email field, required when a cloud role is selected. Verify that creating a manager without an email shows the `400` message and that a duplicate email shows the `409` message.

## 6. Release

- [ ] 6.1 Before the cloud deploy, set an email for every user with admin, manager, accountant or agent (including the MCP service user), and set `LAGUNA_EMAIL` wherever the MCP server targets the cloud. Verify with a query that no cloud-role user has a null email.
- [ ] 6.2 Deploy the cloud and frontend together, then update edge boxes. Verify on staging: a manager signs in to the cloud with an email, a server is refused in the cloud, and a server signs in on Laguna's box with a username.
- [x] 6.3 Run `make lint`, `go build ./...`, `go test ./...` and `make test-acceptance`. Verify all pass.
