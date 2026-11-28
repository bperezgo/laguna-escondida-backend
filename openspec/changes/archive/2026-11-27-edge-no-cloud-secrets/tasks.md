## 1. Configuration (backend)

- [x] 1.1 Move `AdminAPIKey` and `Storage` from `config.Shared` to `config.Cloud`, call `loadStorage()` only from `LoadCloud`, and move the `JWT_SECRET` required check to `LoadCloud`; verify config tests show: the edge loads with none of `ADMIN_API_KEY`, `STORAGE_*`, `JWT_SECRET`; the edge loads with an invalid `CDN_URL_TTL`; the cloud fails naming each of the three when it is missing
- [x] 1.2 Update `boot_test.go` for the two "wrong binary on the other's environment" scenarios; verify both still refuse to boot before opening the database

## 2. Cloud-only clients and routes (backend)

- [x] 2.1 Stop building the S3 client in `NewInfra`; build it in the cloud boot and leave `Infra.Storage` nil on the edge; verify the cloud route tests that upload files still pass and the edge assembly builds with a nil storage client
- [x] 2.2 Remove the admin-key parameters from `registerAPIRoutes` and `registerStockRoutes`, and register `POST /api/stock/opening-balances` with the cloud routes; verify `stock_routes_test.go` still covers opening balances in the cloud, and add an edge route test that a request with `X-API-Key` and no token gets `401` on a protected route and creates no user through `POST /api/users`

## 3. Box signing secret (backend, edge)

- [x] 3.1 Add edge migration 000084 creating `box_secrets` (design decision 4) with the app role's grants and a down migration; verify the migration tests pass for the edge schema and that the cloud schema does not get the table
- [x] 3.2 Add the box secret port and repository with "get or create" using `ON CONFLICT DO NOTHING`; verify with repository tests that a first call creates a value of at least 32 random bytes, a second call returns the same value, and two concurrent first calls return one value
- [x] 3.3 Resolve the secret in the edge assembly when `JWTSecret` is empty and pass it to the core assembly as a value; verify with tests that the environment value wins when set, and that the secret never appears in log output
- [x] 3.4 Add an acceptance test that boots the edge assembly as the app role with none of the three variables: sign in, call a protected route, rebuild the assembly on the same database and call again with the same token (accepted), then present the token to an assembly on a second database (rejected); verify with `DB_HOST= make test-acceptance` from `backend/`

## 4. Updater state folder (backend, edge)

- [x] 4.1 Remove `LAGUNA_STATE_DIR` and the `LagunaStateDir` config field; resolve the folder from the executable's path (design decision 6) in a small function that takes the executable path as an argument, and enable the update service only when the folder exists; verify with unit tests for `C:\laguna-edge\artifacts\edge-node\edge-node.exe`, for another install root, and for a missing folder (boots, `enabled: false`), and that setting either old variable changes nothing
- [ ] 4.2 Update `backend/docs/api/edge-update.md` (the Configuration section no longer names a variable), `backend/.env.example`, the mention in `ARCHITECTURE.md` and the code comments that name the variable; verify `grep -rn "LAGUNA_STATE_DIR" backend` finds nothing
- [x] 4.3 Remove the `EDGE_STATE_DIR` warning from `edge/scripts/install.ps1`, update `edge/docs/UPDATE_PROTOCOL.md`, add a comment in `edge/services/edge-node.xml` that the binary's depth under the root locates `state\`, and make `verify-install.ps1` fail when the edge-node log says owner-triggered updates are off; verify the Windows install smoke test passes with that check

## 5. Appliance (edge)

- [x] 5.1 Remove `ADMIN_API_KEY`, `STORAGE_*`, `ELECTRONIC_INVOICE_*`, `JWT_SECRET`, `EDGE_STATE_DIR` and `APP_MODE` from `edge/env/edge-node.env.example`, with one comment line saying the box creates its own signing secret; verify no script under `edge/scripts/` reads any of them (`grep`)
- [x] 5.2 Remove the same values from the smoke-test env in `.github/workflows/edge-ci.yml`; verify the Windows install smoke test passes on the PR
- [x] 5.3 Update `edge/docs/DEPLOYMENT_RUNBOOK.md` and `edge/services/README.md` so they no longer list these as required; verify neither file mentions `ADMIN_API_KEY` or `STORAGE_`

## 6. Checks

- [x] 6.1 Run `make lint` and `make test` from `backend/`; verify both pass
- [x] 6.2 Run `openspec validate edge-no-cloud-secrets --strict`; verify it reports the change as valid

## 7. Rollout (owner, after merge)

- [ ] 7.1 Cut the next `edge/v*` and update Laguna's box; verify it serves and syncs with its env file unchanged, and that the edge-node log no longer says owner-triggered updates are disabled
- [ ] 7.2 Delete `ADMIN_API_KEY`, `STORAGE_*` and `ELECTRONIC_INVOICE_*` from the box's `env\edge-node.env` (keep a copy until verified) and re-run `install.ps1`; verify `/api/edge/status` reports `online: true` and a ticket prints
- [ ] 7.3 Rotate the cloud's `ADMIN_API_KEY` and the AWS access key that was on the box; verify the cloud boots, the MCP server and aws-infra use the new admin key, and an invoice document still uploads
