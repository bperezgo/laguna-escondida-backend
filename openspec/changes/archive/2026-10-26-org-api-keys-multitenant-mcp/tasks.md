## 1. Domain

- [x] 1.1 Add the API-key DTOs (issue request, issued key with plaintext, listed key without it, key principal) in `domain/dto/`, and the domain errors (key not found, user not eligible, credential refused) in `domain/error/`. Verify with `go build ./...`.
- [x] 1.2 Add the `APIKeyRepository` port: create, list by org, revoke, find the principal by hash (joining the user and the org), and touch last-used. Run `make generate-mocks`. Verify the mocks are generated and `go build ./...` passes.
- [x] 1.3 Implement `APIKeyService.Issue`. It refuses unknown or suspended orgs, users of another org, and users that aren't active or don't hold only the `agent` role. It generates `lgk_` plus 32 random bytes, then stores the hash and the prefix. Verify with `api_key_service_test.go` covering each refusal and that the plaintext isn't stored.
- [x] 1.4 Implement `APIKeyService.List` and `Revoke`, where revoking another org's key or an unknown key gives not found. Verify with service tests.
- [x] 1.5 Implement `APIKeyService.Authenticate(plaintext)`. It returns the principal (user id, org id, current roles) only for a key that exists, isn't revoked, belongs to an active user and to an active org, and otherwise returns one "credential refused" error that carries the reason for logging. Verify with table-driven service tests for each refusal reason, and keep the service at 90%+ coverage.

## 2. Persistence

- [x] 2.1 Add the cloud-only migration that creates `api_keys` (D3): a unique index on `hash`, an index on `organization_id`, and foreign keys to `organizations` and `users`. Verify that `make migrate-up` and `make migrate-down` both apply cleanly, and that the edge migration set doesn't contain it.
- [x] 2.2 Implement the GORM repository. The lookup by hash runs without the org scope (D4) and returns the port type. Verify with an acceptance test that issues a key, finds its principal from a context with no org, and gets nothing back for a revoked key.

## 3. HTTP

- [x] 3.1 Extend `JWTAuthMiddleware` (D5): an `lgk_` bearer goes to `APIKeyService.Authenticate` and sets the same context keys and `tenant.WithOrg`. A refusal logs the reason without the key and returns the generic 401 body. Where no API-key service is wired (edge), an `lgk_` bearer gets the same 401. Verify with middleware tests for a valid key, each refusal, an unchanged JWT path, and the edge refusal.
- [x] 3.2 Throttle `last_used_at` updates to at most one per key per minute, off the request path. Verify with a test that several requests produce one update.
- [x] 3.3 Add the `APIKeyHandler` and the cloud routes `POST|GET /api/platform/organizations/:id/api-keys` and `DELETE …/:keyId` behind `AdminAPIKeyMiddleware`, wired in `cloud.go`. Verify with handler tests for status mapping (201, 400, 404), with `routes_cloud.golden` updated, and with `routes_edge.golden` unchanged.

## 4. MCP server (customer mode)

- [x] 4.1 In `config.go`, stop requiring `MCP_AUTH_TOKEN`, `LAGUNA_EMAIL`, `LAGUNA_USERNAME` and `LAGUNA_PASSWORD` when `MCP_TOOLSET=customer`, and leave the full toolset's checks unchanged. Verify with `config_test.go` cases for both toolsets.
- [x] 4.2 In `cmd/mcp-server/main.go`, wrap `/mcp` in customer mode with a guard that requires `Bearer lgk_…`, returns 401 otherwise, and puts the bearer on the request context. Verify with tests: no header gives 401, a JWT-shaped bearer gives 401, and a key passes through.
- [x] 4.3 In `client.go`, send the context's bearer when there is one, never sign in or retry for it, and turn a backend 401 into a "credential refused" tool error. Verify with `client_test.go`: two contexts with different keys reach the backend with their own bearer, concurrently, and a 401 makes no second request.
- [x] 4.4 Run the customer tools end to end against a fake backend with two keys. Verify with `tools_customer_test.go` that `list_menu` and `create_delivery_order` each carry the caller's key, and that the phone and location header behavior is unchanged.

## 5. Isolation and acceptance

- [x] 5.1 Write an acceptance test with two orgs, each with an agent key, that covers: org A's key can't read org B's products or orders, can't change org B's order by id, and creates orders only in org A with `created_by` set to the key's user. Verify that it passes under `DB_HOST= make test-acceptance`.
- [x] 5.2 Write an acceptance test showing that deactivating the user, suspending the org and revoking the key each lead to 401 on the next request, with an identical body. Verify that it passes under `make test-acceptance`.
- [x] 5.3 Write a test that fails if the key lookup is ever run with the org scope applied, as the guard for RLS (D4). Verify that it fails when a scope is added deliberately.

## 6. Docs

- [x] 6.1 Add `docs/api/api-keys.md` in the format of `docs/api/products.md` (endpoint table, then request, response and errors with realistic JSON), and add the Bearer `lgk_` note to `docs/api/auth.md`. Verify that every route from 3.3 is documented.
- [x] 6.2 Add curl examples for issuing, listing, revoking and using a key to `docs/examples/curl-examples.md`. Verify them by running them against a local cloud.
- [x] 6.3 Rewrite the customer section of `docs/mcp-server.md`: one deployment, a key per org, the new configuration table, and the rollout steps from design.md. Verify that it no longer mentions `MCP_AUTH_TOKEN` or the service account for customer mode.

## 7. Wrap-up

- [x] 7.1 Run `go build ./...`, `go test ./...` and `make lint` until all three are clean.
- [x] 7.2 Run `openspec validate org-api-keys-multitenant-mcp --strict` and verify that it passes.
