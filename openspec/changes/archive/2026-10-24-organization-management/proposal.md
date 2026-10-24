## Why

The POS is becoming a platform for a few restaurants on **one database and one deployment** (`docs/multi-tenancy-plan.md`, decisions log 2026-10-21). Today an organization exists only as a row inserted by migration 33 and the `ORGANIZATION_ID` env var. There is no way to create a second organization or to store the settings that the plan moves out of env vars and frontend constants (timezone, country, locale, currency, business info, receipt text, branding).

This change is the first piece of tenancy step 9 (onboarding), pulled forward because it does not depend on the tenant columns. `organizations` is a global table, so creating a row exposes nothing. A new organization can't hold users or data until tenancy steps 2–4 land, and this change does not try to make it.

## What Changes

- **Organization settings columns.** Migration `000068` adds these columns to `organizations`:
  - `slug` (unique) and `status` (`active` | `suspended`)
  - `timezone`, `country`, `locale`, `currency`
  - business info: `legal_name`, `nit`, `address`, `phone`
  - receipt text: `ticket_footer`, `ticket_legal_notice`
  - branding: `display_name`, `logo_url`, `primary_color`, `secondary_color`

  Laguna's row is backfilled: slug `laguna-escondida`, `active`, `America/Bogota`, `CO`, `es-CO`, `COP`. The timestamps are left alone: migration 49 already made them `TIMESTAMPTZ`.
- **Platform organization endpoints** (cloud only, behind `ADMIN_API_KEY`):
  - `POST /api/platform/organizations`: create an organization, optionally with its e-invoicing prefixes, in one transaction
  - `GET /api/platform/organizations`: list organizations
  - `GET /api/platform/organizations/:id`: get one organization, including its e-invoicing prefixes
  - `PATCH /api/platform/organizations/:id`: partial update
  - `PUT /api/platform/organizations/:id/einvoicing`: create or replace the invoice and support-document prefixes
- **No delete.** An organization is retired with `status = suspended`, because its invoices are legal records.
- **Stored, not yet enforced.** `status`, timezone, locale, currency, business info and branding are stored and returned, but nothing reads them yet:
  - the running organization still comes from `ORGANIZATION_ID`
  - the edge still prints from `BUSINESS_*` and `TICKET_*`
  - the timezone is still hardcoded to `America/Bogota`
  - a suspended organization is not blocked yet

  Each consumer switches over in its own tenancy step.

Out of scope, deferred to the tenancy steps named in `docs/multi-tenancy-plan.md` §8:
- step 2: the first admin user and membership, and seeding per-org expense categories
- step 3: `platform_admin` replacing `ADMIN_API_KEY` on these routes; a tenant-scoped `GET/PATCH /api/organization`; enforcing `suspended`
- step 5: e-invoicing provider URL and encrypted credentials; reading timezone and business info
- step 6: registering edge nodes and sending org info in the sync pull
- step 7: org API keys
- step 8: frontend branding and formatting (this change only gives the frontend the data, on `/auth/me`)
- MCP tools for organization management

## Capabilities

### New Capabilities
- `organization-management`: The organization's settings model and its defaults and validation, and the platform endpoints that create, list, read and update organizations and their e-invoicing prefixes. Also the guarantee that organizations are never deleted.

### Modified Capabilities
- (none: the project has no durable specs yet.)

## Impact

- **Database:** migration `000068_extend_organizations` adds the settings columns and backfills Laguna. The migration runs on edge and cloud alike; the edge just never writes to these columns. `pos_app` already gets privileges on `organizations` (migration 65).
- **Domain:**
  - `dto/organization.go` gains the new fields plus create and update requests
  - new errors in `error/organization_error.go`
  - new port `OrganizationRepository`
  - `EInvoicingSettingsRepository` gains `Upsert`
  - new `OrganizationService` with service tests
- **Platform:**
  - new `organization_repository.go`
  - `einvoicing_settings_repository.go` implements `Upsert`
  - new `organization_handler.go`
  - routes in `registerCloudRoutes`, wiring in `cmd/main.go`
  - the route-guard test (`cmd/agent_routes_test.go`) covers the new routes
- **Docs:**
  - new `docs/api/organizations.md`
  - an example in `docs/examples/curl-examples.md`
  - the decisions log and step 9 of `docs/multi-tenancy-plan.md`
- **No behavior change** for existing endpoints, sync, crons or the edge, except that `GET /api/auth/me` gains an `organization` object (design 10), which fails with `500` if the configured organization is missing.
