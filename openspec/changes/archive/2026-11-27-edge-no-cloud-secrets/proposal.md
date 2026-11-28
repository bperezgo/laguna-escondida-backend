## Why

The edge binary refuses to boot without `ADMIN_API_KEY`, `STORAGE_BUCKET` and `JWT_SECRET`, although nothing on a box uses the first two and the third never has to match the cloud. A box that customers install alone must not need secrets that only the operator can supply, and must not carry the cloud's admin key or AWS credentials.

## What Changes

- The edge binary stops reading `ADMIN_API_KEY` and every `STORAGE_*` variable, and no longer builds an S3 client. No edge route accepts the admin API key today, so no behavior on the box changes.
- `JWT_SECRET` becomes optional on the edge. When it is unset, the box creates a random signing secret on first boot, keeps it in its own database and reuses it on every later boot, so sessions survive restarts and appliance updates. A value in the environment still wins, so Laguna's box keeps its current sessions.
- The edge finds the updater's state folder by itself, at `state\` under the install root, worked out from where the binary runs. The `LAGUNA_STATE_DIR` variable is removed. This fixes a mismatch: the appliance sets `EDGE_STATE_DIR`, which the backend never reads, so a box installed from the current template never offers updates.
- The cloud binary is unchanged: it still requires all three.
- The appliance stops asking for them: `edge/env/edge-node.env.example` drops `ADMIN_API_KEY`, `STORAGE_*`, `ELECTRONIC_INVOICE_*`, `JWT_SECRET`, `EDGE_STATE_DIR` and the unread `APP_MODE`; `install.ps1` stops warning about the state variable; the install smoke test in `edge-ci.yml` runs without them; `DEPLOYMENT_RUNBOOK.md` and `services/README.md` stop listing them as required.
- The user-creation requirement loses its clause about admin-key creation on the edge, which describes a route the edge binary no longer serves.

Not in this change: enrollment (`edge-box-enrollment`), the installer, and the database password.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `cloud-edge-binaries`: the edge boots without the admin API key, storage settings or a configured signing secret; it holds no cloud credentials; it creates and keeps its own signing secret; it finds the updater's state folder without configuration.
- `organization-sign-in`: user creation with the admin API key is described for the cloud only.

## Impact

- **Backend**: `config/shared.go`, `config/cloud.go`, `config/edge.go` (the three settings move from shared to cloud; `JWT_SECRET` optional on the edge); `bootstrap/infra.go`, `cloud.go`, `edge.go`, `core.go`, `routes_shared.go`, `routes_stock.go` (storage client and admin-key middleware built only in the cloud); one edge-only migration and a small repository for the box's signing secret.
- **Backend (edge update)**: `config/edge.go`, `bootstrap/edge.go`, `edgeupdate/state_store.go`, `docs/api/edge-update.md`, `backend/.env.example`.
- **Edge appliance**: env template, CI smoke test, `UPDATE_PROTOCOL.md` and two other docs, one warning removed from `install.ps1`.
- **Operations**: none required. Laguna's box keeps working with its current env file; the unused values can be deleted from it at any time.
- **Security**: a stolen or inspected box no longer yields the platform admin key or AWS credentials. Laguna's box holds both today, so they should be rotated in the cloud once the box's env file is cleaned.
