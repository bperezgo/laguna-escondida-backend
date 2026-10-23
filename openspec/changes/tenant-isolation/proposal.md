## Why

The cloud must serve several organizations from **one deployment and one database** (`docs/multi-tenancy-plan.md`). Today no business table has an `organization_id`, and every request runs as the org in the `ORGANIZATION_ID` env var (`cmd/main.go:308`), so a second organization would read and write Laguna's data. This change makes the caller, not the server's config, decide whose data a request sees, and makes the database enforce it.

It covers tenancy steps 2–4 of the plan as one change, because the column, the org in the token and the repository filter only isolate data together; none of them is useful alone. Laguna stays the only organization until it ships.

## What Changes

- **Tenant column on every tenant table.** `organization_id uuid NOT NULL REFERENCES organizations` on: `users`, `user_roles`, `products`, `product_ingredients`, `product_preparation_responsibilities`, `bills`, `bill_products`, `bill_owners`, `open_bills`, `open_bills_products`, `open_bill_product_side_dishes`, `pending_invoices`, `stock`, `historic_stock`, `suppliers`, `supplier_catalog`, `purchase_entries`, `purchase_entry_items`, `expenses`, `expense_categories`, `support_documents`, `support_document_products`, `delivery_zones`, `idempotency_keys`, `nodes`, `sync_outbox`, `sync_inbox`, `sync_state`. Existing rows are backfilled with Laguna's id. `invoice_sequences` and `organization_einvoicing` already have it.
- **Uniques per organization**, among them `users.username` → `(organization_id, username)`, `bill_owners` → `(organization_id, id)`, `expense_categories.code` → `(organization_id, code)`. Composite foreign keys stop a row from pointing at another org's row.
- **One organization per user.** `users.organization_id` replaces the memberships model in the plan; there is no org picker and no org switching.
- **BREAKING: sign-in takes the organization.** `POST /api/auth/signin` requires `organization` (the immutable slug) in the cloud. On the edge it is optional and must match the box's org. An unknown or suspended org returns the same opaque `401` as bad credentials. The frontend adds a "Restaurante" field to its single sign-in page.
- **BREAKING: the JWT carries `org_id`.** A token without it is rejected, so everyone signs in once more after the release.
- **The org comes from the caller.** Authentication puts the org in the context (`domain/tenant`); `DBSessionMiddleware` runs after authentication and takes the org from the context instead of `cfg.OrganizationID`. A request with no org gets no tenant session.
- **Mode from `APP_MODE`, checked at boot.** Edge requires `ORGANIZATION_ID` and rejects a token from another org; cloud refuses to boot when `ORGANIZATION_ID` is set. The cloud sets `NODE_ID` explicitly to the id it derives for Laguna today, so the edge box's sync state stays valid.
- **Services take the org from the context**, not from a constructor argument (S3 keys, e-invoicing settings and consecutives, support documents).
- **Repository scopes.** Every tenant query is filtered by the context's org, and every create stamps it from the context, never from a DTO. A missing org is an error.
- **Row-level security** on every tenant table, keyed on `app.org_id`, failing closed. The app connects as `pos_app`.
- **Background work runs per org**, since under RLS a query without an org sees nothing. Cloud crons loop over active orgs; event payloads carry the org; the cloud's sync push and pull resolve the org from the calling node's `nodes` row. On the edge, background work uses the box's org.
- **Leak suite.** An acceptance suite seeds two orgs and asserts that, acting as one, nothing of the other can be read or written, with the app scopes on and, separately, with RLS alone.

Out of scope, each its own change before a second organization goes live:
- per-node sync keys (until then only Laguna's box may be registered, since the key is shared)
- the SSE hub keyed by `(org, area)`
- encrypted per-org e-invoicing credentials (until then only Laguna may have an `organization_einvoicing` row)
- org API keys for the MCP server and the WhatsApp agent
- per-org branded sign-in URLs and a platform-admin user (`/api/platform/*` keeps `ADMIN_API_KEY`)

## Capabilities

### New Capabilities
- `tenant-isolation`: how a request's organization is determined (from the caller in the cloud, checked against the box's org on the edge), and the guarantee that no request can read or write another organization's data, enforced by the application and by the database.
- `organization-sign-in`: signing in to an organization with its slug, username and password, with usernames unique per organization and opaque failures.

### Modified Capabilities
- (none: no existing spec covers authentication or data ownership.)

## Impact

- **Database:** a migration adding `organization_id`, per-org uniques and composite foreign keys with a Laguna backfill; a migration enabling RLS. Runs on edge and cloud. Ops creates the `pos_app` login user and points `DB_USER` at it before RLS is enabled.
- **Domain:** new `domain/tenant` package; `UserDomain` / claims gain the org; services drop `organizationID` constructor arguments; user and sign-in services change. Existing tests of changed services need sign-off before they are edited.
- **Platform:** auth middleware, `DBSessionMiddleware` placement and signature, `session.go`, every repository (scope + create stamping), config boot checks, `cmd/main.go` wiring.
- **API:** `POST /api/auth/signin` request body; JWT structure. `docs/api/auth.md` and `docs/examples/curl-examples.md` change.
- **Other repos:** `laguna-escondida-frontend` sign-in page and route handler (ship together); `aws-infra` task definition (drop `ORGANIZATION_ID` from cloud, set `NODE_ID`, `DB_USER`/`DB_MIGRATION_USER`).
- **Release:** backend and frontend ship together; all sessions are invalidated.
