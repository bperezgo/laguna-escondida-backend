## Context

See proposal.md for motivation. What the code does today:

- `config.LoadEdge()` calls `loadShared()`, which fails without `ADMIN_API_KEY`, `JWT_SECRET` and `STORAGE_BUCKET`, and validates `CDN_URL_TTL`.
- `bootstrap.NewInfra` builds an S3 client for both binaries. Only `bootstrap/cloud.go` passes `infra.Storage` to services (invoices, support documents, purchase entries, expenses).
- `registerAPIRoutes` (shared) builds `AdminAPIKeyMiddleware` and hands it to `registerStockRoutes`, which uses it only for `POST /api/stock/opening-balances`, registered only when `StockAuthoring` is true (cloud). Every other admin-key route is in `routes_cloud.go`. So no edge route accepts the admin key.
- `JWT_SECRET` feeds `service.NewJWTService` in `bootstrap/core.go`. On a box it signs tokens at sign-in and verifies them on protected routes. Nothing the box sends to the cloud uses it.
- Config is loaded before the database is opened (`bootstrap.Main`), so that a box running the wrong binary changes nothing.

## Goals / Non-Goals

**Goals:**

- A box's env file needs no value that the operator must supply, apart from what enrollment returns.
- The cloud binary's configuration and behavior do not change.
- Laguna's box keeps working, and keeps its sessions, with its env file untouched.

**Non-Goals:**

- No rotation endpoint or schedule for the box's signing secret.
- No change to how the node key or the database password reach the box.
- No cleanup of `edge/ai-plan/`, which is historical.

## Decisions

### 1. The three settings move from `Shared` to `Cloud`

`AdminAPIKey` and `Storage` leave `config.Shared` for `config.Cloud`. `JWTSecret` stays in `Shared` (both binaries sign tokens) but its "is not set" check moves to `LoadCloud`; `LoadEdge` accepts it empty. `loadStorage()` is called only by `LoadCloud`, which is what stops an invalid `CDN_URL_TTL` from failing a box.

Alternative: keep them in `Shared` and skip the checks by mode. Rejected: the config split exists so that each binary's struct holds only what it uses.

### 2. The storage client is built by the cloud assembly

`NewInfra` stops building the S3 client. `Infra.Storage` stays as a field so tests can inject a stub, and the cloud boot fills it before assembling. The edge leaves it nil, like `ElectronicInvoice` already is.

### 3. The admin-key middleware is created only in the cloud

`registerAPIRoutes` and `registerStockRoutes` lose the `adminAPIKey` / `adminAuth` parameters. `POST /api/stock/opening-balances` moves to the cloud route registration, next to the other admin-key routes. With no middleware instance on the edge, the spec's "admin key is ignored on a box" holds by construction.

### 4. The box's signing secret lives in its own database

When `cfg.JWTSecret` is empty, the edge assembly resolves the secret before building the JWT service: read the single row of a new edge-only table; if there is none, generate 32 random bytes (base64url), insert, and read back. The insert uses `ON CONFLICT DO NOTHING` on a fixed primary key so two starts cannot store two secrets.

```sql
CREATE TABLE box_secrets (
    name       VARCHAR(64) PRIMARY KEY,
    value      TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

One row, `name = 'jwt_signing'`. The table is not a tenant table (no `organization_id`, no row-level policy) and is never part of sync. It goes under `migrations/edge/`, with the grant the app role needs to read and insert. Because its repository is deliberately not organization-scoped, `box_secret_repository.go` must be listed in `globalTableFiles` in `repository/tenant_scope_check_test.go`, or `TestTenantScopeCheck_RepositoriesAreScoped` fails.

Why the database:

- It survives an appliance update. `artifacts\edge-node\` is swapped on update, so a file next to the binary would log everyone out on each release.
- `EDGE_STATE_DIR` is optional and already shared with the updater script.
- The database is what a backup or restore of the box carries, so the sessions' key travels with the data it protects.

Alternative: the installer generates the value and writes it to the env file. Rejected: every way of installing a box (installer, scripts, CI) would have to remember to do it, and an env file with one less secret is the goal.

Alternative: derive the secret from the node key. Rejected: replacing or re-enrolling the box would change it, and it ties two unrelated credentials together.

The environment value wins when set, so Laguna's box is unaffected until its `JWT_SECRET` line is removed; removing it later logs its users out once.

### 5. Resolution happens in the edge assembly, not in config loading

`LoadEdge` cannot read the database (decision recorded in `bootstrap.Main`: config first, database second). The edge assembly already receives `infra.DB`, so it resolves the secret there and passes it on. The core assembly takes the secret as a value, not from `cfg`, so the cloud path stays "from the environment".

### 6. The state folder is derived from the executable's path

Today `config.LoadEdge` reads `LAGUNA_STATE_DIR`, while `edge/env/edge-node.env.example` and `install.ps1` name `EDGE_STATE_DIR`; nothing maps one to the other. The scripts never read either: `install.ps1` and `updater.ps1` both use `Join-Path $Root 'state'`. So the folder is already a fixed place relative to the install root, and only the backend needs telling.

The edge resolves it as `filepath.Join(filepath.Dir(os.Executable()), "..", "..", "state")`: `edge-node.xml` runs the binary from `<root>\artifacts\edge-node\`. The edge assembly enables the update service when that path is an existing directory, and logs that updates are off otherwise. `install.ps1` creates the folder before it starts the services, so an installed box always has it. The variable and the `LagunaStateDir` config field are removed; the state store keeps taking its directory as a constructor argument, so tests are unchanged.

Alternative: default to the literal `C:\laguna-edge\state`. Rejected: it breaks on any other install root and is meaningless on a developer's machine.

Alternative: keep an environment override. Rejected: the scripts cannot follow an override, so a box could be configured into the same split this decision removes.

## Risks / Trade-offs

- [The binary is moved to a different depth under the install root and silently loses the update banner] → The folder layout is set by `services/edge-node.xml`; a comment there names the dependency, and the Windows install smoke test fails when the edge-node log says updates are off (the endpoint itself needs a signed-in user).

- [Anyone who can read the box's database can read the signing secret] → They can already read and change users and password hashes there; the secret adds nothing to what database access gives.
- [Restoring a database backup onto another box carries the secret with it] → Intended for a restore of the same restaurant; a box enrolled fresh creates its own.
- [The app role cannot read the new table because of a missing grant, and the box fails to boot] → The acceptance test boots the edge assembly as the app role with `JWT_SECRET` unset.
- [Laguna's box still holds the platform admin key and AWS credentials in its env file after this ships] → Rollout step: delete the lines, then rotate both in the cloud.

## Migration Plan

1. Ships in an `edge/v*` release; no cloud release is needed. Order relative to `edge-box-enrollment` does not matter.
2. Laguna's box updates as usual and keeps running on its existing env file.
3. Afterwards, on the box: delete `ADMIN_API_KEY`, `STORAGE_*` and `ELECTRONIC_INVOICE_*` from `env\edge-node.env`, re-run `install.ps1`, confirm the box serves and syncs. Leave `JWT_SECRET` in place, or remove it at a quiet hour (one forced sign-in).
4. Rotate the cloud's `ADMIN_API_KEY` and the AWS access key that was on the box.

Rollback: install the previous edge release. It needs the three values again, so keep a copy of the old env file until step 3 is confirmed.

## Implementation map

- `backend/internal/platform/config/shared.go`, `cloud.go`, `edge.go` (+ `shared_test.go`, `cloud_test.go`, `edge_test.go`)
- `backend/internal/bootstrap/infra.go`, `main.go`, `boot.go`, `cloud.go`, `edge.go`, `core.go`
- `backend/internal/bootstrap/routes_shared.go`, `routes_stock.go`, `routes_cloud.go` (+ `stock_routes_test.go`, `edge_routes_test.go`, `boot_test.go`)
- `backend/internal/platform/postgres/migrations/edge/` : new migration `box_secrets`
- `backend/internal/domain/ports/` and `backend/internal/platform/postgres/repository/` : box secret store
- `backend/test/acceptance/` : edge boot without the three variables
- `backend/internal/platform/edgeupdate/state_store.go`, `backend/internal/platform/handler/edge_update_handler.go` (comments naming the variable), `backend/docs/api/edge-update.md`, `backend/docs/playbooks/ARCHITECTURE.md` (the one mention), `backend/.env.example`
- `edge/env/edge-node.env.example`, `.github/workflows/edge-ci.yml`, `edge/docs/DEPLOYMENT_RUNBOOK.md`, `edge/services/README.md`
- `edge/scripts/install.ps1` (the `EDGE_STATE_DIR` warning), `edge/scripts/verify-install.ps1`, `edge/services/edge-node.xml` (comment), `edge/docs/UPDATE_PROTOCOL.md`
