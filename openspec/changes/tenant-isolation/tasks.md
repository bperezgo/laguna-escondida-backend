## 1. Release 1: schema

- [x] 1.1 Add the schema migration per design decision 4: `organization_id` on every tenant table in `specs/tenant-isolation` (add with Laguna's id as `DEFAULT` → `SET NOT NULL`; the default stays until task 4.5), indexes leading with `organization_id`, and a down migration. Verify `make migrate-up`, `make migrate-down`, `make migrate-up` run clean and no tenant row has a NULL org.
- [x] 1.2 In the same migration, swap the uniques to per-org: `users.username` → `(organization_id, username)`, `bill_owners` PK → `(organization_id, id)`, `expense_categories.code` → `(organization_id, code)`, `sync_outbox` → `(organization_id, origin_node_id, seq)`. Verify with a migration test that the same username and the same expense code can exist in two orgs, and not twice in one.
- [x] 1.3 Add composite unique `(organization_id, id)` on the parent tables and composite foreign keys on the relations in plan §2 (open bill products, bill products, bills → bill owners, stock and historic stock → products, supplier catalog, purchase entry items). Verify with a migration test that a child row pointing at another org's parent is rejected.
- [ ] 1.4 Rehearse the migration on a restored production snapshot and record its duration. If the foreign keys lock too long, switch them to `NOT VALID` + `VALIDATE`. Verify the rehearsal notes are in the PR.
- [x] 1.5 Existing integration-test seed helpers that insert tenant rows without `organization_id` fail once the column has no default (task 4.5). **Ask before editing existing test helpers**, then add the org. Verify `DB_HOST= go test ./internal/platform/postgres/...`.

## 2. Release 2: tenant context and boot checks

- [x] 2.1 Add `domain/tenant` with `WithOrg` and `OrgFrom`, and a domain error for a missing org. Verify `go build ./...`.
- [x] 2.2 Config: `ORGANIZATION_ID` required in edge mode, rejected in cloud mode; `NODE_ID` required in cloud mode; the edge keeps deriving its ids. Verify with `config_test.go` cases for all four `APP_MODE` × `ORGANIZATION_ID` combinations and a missing cloud `NODE_ID` (**ask before editing existing config tests**).
- [x] 2.3 Compute Laguna's current cloud node id and document it for the cloud task definition in `aws-infra`. Verify it equals the edge's default `CLOUD_NODE_ID` with a unit test.
- [x] 2.4 `DBSessionMiddleware(db)`: drop the `orgID` parameter, take the org from `tenant.OrgFrom`, open no session without one; register it inside the authenticated route groups after auth instead of globally. Verify a handler test that two requests with tokens of different orgs get sessions with different `app.org_id`, and an unauthenticated request gets none.

## 3. Release 2: sign-in and tokens

- [x] 3.1 Add `org_id` to the JWT claims and `UserDomain`; auth middleware rejects a token without it and calls `tenant.WithOrg`; on the edge it also rejects a token of another org. Verify middleware tests for each case in `specs/organization-sign-in` and `specs/tenant-isolation` ("An edge box serves only its own organization").
- [x] 3.2 Sign-in takes `organization`: resolve the slug through the organization repository, reject unknown or suspended with the opaque `401`, then look up the user in that org's session. Edge: optional, must match the box's slug. Verify service tests `TestAuthService_SignIn_*` for every scenario in `specs/organization-sign-in` (**ask before editing existing sign-in tests**).
- [x] 3.3 User creation: users created by a signed-in user land in the creator's org; `POST /api/users` with the admin key names the org by slug in the cloud (`400` without it, `404` unknown). Duplicate username in one org → `409`. Verify `user_service_test.go` cases (**ask before editing existing user tests**).
- [x] 3.4 `GET /api/auth/me` returns the token's organization. Verify a handler test with a user of a second org.
- [x] 3.5 Update `docs/api/auth.md` (request body, `org_id` claim, re-sign-in note), `docs/api/users.md` (bootstrap body) and `docs/examples/curl-examples.md`. Verify the curl example signs in against a local server.

## 4. Release 2: repositories and services

- [x] 4.1 Add the tenant helper in `platform/postgres` (design decision 5): scoped `*gorm.DB` from the context's org, error when missing. Verify unit tests for both cases.
- [x] 4.2 Move every tenant repository onto the helper and stamp `organization_id` from the context on every create, including raw SQL (`claimConsecutive`, sync appliers' upserts). Work repository by repository; verify each with its integration test and `go test ./...`.
- [x] 4.3 Add a CI check (lint rule or grep in `make lint`) that flags direct `GetTxOrDB` use on tenant models outside the helper. Verify it fails on a deliberately unscoped call and passes on the tree.
- [x] 4.4 Services drop their `organizationID` constructor argument and read `tenant.OrgFrom(ctx)`: `PurchaseEntryService`, `ExpenseService`, `SupportDocumentService`, `InvoiceSubmissionService`, `SupportDocumentRepository`, `UserHandler`. Run `make generate-mocks` if ports change. Verify service tests cover a missing org and S3 keys under the context's org (**ask before editing existing service tests**).
- [x] 4.5 Add a migration that drops the Laguna `DEFAULT` on every `organization_id` added by migration A, once every create stamps the org. Verify an insert without the column fails and `make migrate-up/down/up` runs clean.

## 5. Release 2: background work

- [x] 5.1 Cloud crons loop over active organizations with a tenant context and session each; e-invoicing jobs only for orgs with an `organization_einvoicing` row; one org's failure is logged and the loop continues. Verify scheduler tests: two orgs, one failing, one suspended.
- [x] 5.2 The admin-key maintenance triggers (`update-missing-document-urls` for invoices and support documents) run per org like the crons; `POST /api/stock/opening-balances` requires the org to be named. Verify handler tests for each scenario under "Admin-key operations name their organization".
- [x] 5.3 Event payloads (`order.*`, `purchase_entry.created`) carry `organization_id`; subscribers restore it on their detached context and open their own session; a payload without it is rejected and logged. Verify subscriber tests for both cases.
- [x] 5.4 Cloud sync push and pull resolve the org from the calling node's `nodes.organization_id`; an unregistered node is rejected with `401`. Verify sync handler tests, and an integration test that the pull returns only the node's org's users and products.
- [x] 5.5 Edge background work (push and pull loops, local crons) uses the box's `ORGANIZATION_ID`. Verify the edge boots and its loops run against a testcontainers database.

## 6. Release 2: leak suite and frontend

- [ ] 6.1 Add `test/acceptance/tenancy`: seed orgs A and B with mirrored data and, acting as A, assert no endpoint, MCP-exposed endpoint, sync push or pull, cron run or event handler reads or changes B's data. Verify `make test-acceptance` passes and fails when one repository's scope is removed on purpose. _Unblocked by `split-cloud-edge-binaries`: boot the real cloud router with `bootstrap.NewCloud(cfg, infra)`, injecting an `Infra` with the testcontainers `*gorm.DB`, a stub `ports.ElectronicInvoiceClient` and a fake `ports.StorageClient`, and drive `App.Router` with `httptest`. Building the app starts no cron or subscriber, so cron runs are exercised through `internal/platform/cron` directly._
- [ ] 6.2 Frontend (`laguna-escondida-frontend`): "Restaurante" field on `app/signin/page.tsx`, remembered in the browser, passed through `app/api/auth/signin`; hidden or optional on the edge. Verify signing in to Laguna end-to-end against a local backend.
- [ ] 6.3 Update `aws-infra` task definition: drop `ORGANIZATION_ID` from the cloud, set `NODE_ID` (task 2.3). Verify the cloud boots in staging with the new config.
- [ ] 6.4 Release 2 to staging with a production snapshot: Laguna signs in with the slug, sync with Laguna's box continues (no resend or gap), the crons run for Laguna. Verify on staging before production.

## 7. Release 3: row-level security

- [ ] 7.1 Cloud boot check: refuse to start when the connected role owns the tenant tables or has `BYPASSRLS`. Verify with a testcontainers test connecting as the owner and as a `pos_app` login user.
- [ ] 7.2 Add the RLS migration (design decision 4): `ENABLE` + `FORCE ROW LEVEL SECURITY` and the `app.org_id` policy on every tenant table, with a down migration that disables it. Verify `make migrate-up/down/up` run clean.
- [ ] 7.3 Run the repository integration tests and the leak suite as a `pos_app` login user with the app scope disabled, proving RLS alone isolates and that a query with no org set returns nothing and cannot insert. Verify both pass.
- [ ] 7.4 Ops: create the `pos_app` login user in staging and production, point `DB_USER` at it and `DB_MIGRATION_USER` at the owner. Verify the app boots past the check from 7.1.
- [ ] 7.5 Release 3 to staging, run the leak suite with a scope removed on purpose, watch the crons and Laguna's box sync, then production. Verify no tenant query returns empty for Laguna in the logs over one service day.

## 8. Wrap-up

- [ ] 8.1 Update `docs/multi-tenancy-plan.md`: decisions log (one org per user, usernames per org, sign-in by slug, `APP_MODE` boot rules, background work per org), mark steps 2–4 done, and note that memberships are dropped. Verify the plan no longer mentions memberships as planned work.
- [ ] 8.2 Run `make lint`, `go build ./...`, `go test ./...` and `make test-acceptance`. Verify all pass.
