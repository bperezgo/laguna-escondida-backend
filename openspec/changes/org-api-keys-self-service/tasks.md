## 1. Backend: data and permission

- [x] 1.1 Add `api_keys:manage` (admin only) to `permissions.go`/`role_permissions.go` and verify `role_permissions_test.go` asserts admin has it and no other role does
- [x] 1.2 Add the `add_users_api_key_managed` migration (`BOOLEAN NOT NULL DEFAULT false`) to both `migrations/cloud/` and `migrations/edge/` at the next free numbers, and verify `TestEdgeCloudSharedSchemaParity` passes
- [x] 1.3 Add `APIKeyManaged` to `dto.User` and the user GORM model/mapping, and verify `go build ./...`

## 2. Backend: hide users created for keys

- [x] 2.1 Filter marked users in `UserRepository.FindAll` and `SyncReferenceRepository.FindChangedUsers`, and verify with repository integration tests (marked user absent from both)
- [x] 2.2 Make `UserService` get/update/delete/reset-password return `ErrUserNotFound` for a marked user and refuse it at sign-in (cloud and edge paths), and verify with `TestUserService_*_APIKeyManaged*` tests

## 3. Backend: service

- [x] 3.1 Extract the plaintext/hash/prefix creation from `APIKeyService.Issue` into a helper and verify the existing `api_key_service_test.go` still passes unchanged
- [x] 3.2 Inject `ports.UnitOfWork` into `APIKeyService` (update `cloud.go` wiring), run `make generate-mocks` if any port changed, and verify `go build ./...`
- [x] 3.3 Implement `IssueForOrganization(orgID, name)` (user + agent role + key in one unit of work; name validation; suspended org refused), and verify tests for success, a distinct user per key, blank or too-long name, suspended org, and rollback when key creation fails
- [x] 3.4 Implement `RevokeForOrganization` and make operator `Revoke` deactivate a marked user in the same unit of work, and verify tests for a marked user deactivated, an unmarked user left active, and another org's key returning not found

## 4. Backend: HTTP

- [x] 4.1 Add `OwnIssueHandler`, `OwnListHandler` and `OwnRevokeHandler` to `APIKeyHandler` (org from token, refuse an API-key principal, error mapping as in the operator handlers), and verify `api_key_handler_test.go` cases for 201/200/204, 400, 403, 404 and the API-key bearer refusal
- [x] 4.2 Register `/api/organization/api-keys` in `routes_cloud.go` with `JWTAuthMiddleware` + `RequirePermission(APIKeysManage)`, and verify a route test that a manager gets 403, an admin gets 201, and the edge router returns 404
- [x] 4.3 Run `make lint` and `make test` from `backend/` and verify both are clean

## 5. Docs

- [x] 5.1 Add an "Organization routes" section to `docs/api/api-keys.md` (request, responses, errors, the dedicated-user and revoke-deactivates rules) and a curl example to `docs/examples/curl-examples.md`, and verify the examples match handler responses
- [x] 5.2 Update `docs/mcp-server.md` "Adding a restaurant" to say keys can be issued from Ajustes, and verify the operator flow is still documented

## 6. Frontend

- [x] 6.1 Add `types/organizationApiKeys.ts` and `lib/api/organizationApiKeys.ts` (list, issue, revoke) with `organizationApiKeys.test.ts`, and verify `pnpm test`
- [x] 6.2 Add Next proxy routes `app/api/organization/api-keys/route.ts` (GET, POST) and `[keyId]/route.ts` (DELETE) following the integrations proxy, and verify the errors pass through
- [ ] 6.3 Read `MCP_PUBLIC_URL` in `app/(main)/ajustes/integraciones/page.tsx` and pass it down, and verify the page renders with and without it
- [ ] 6.4 Build `components/settings/ApiKeysCard.tsx` (list, issue dialog showing the plaintext once with copy and URL, in-app revoke confirmation) and render it from `IntegrationsPageClient.tsx` only for `api_keys:manage`, and verify with component tests for issue, revoke, no URL, and hidden for a manager
- [x] 6.5 Run `pnpm lint` and `pnpm test` from `frontend/` and verify both are clean

## 7. End to end

- [ ] 7.1 Against the local sync stack, issue a key from Ajustes, call `initialize` on `http://localhost:8090/mcp` with it, revoke it, and verify the second call gets 401 and the key's user is absent from Usuarios
