# Multi-tenancy: one database, one container, organization from the token

**Opened:** 2026-10-21 · **For:** this backend, the MCP server, `laguna-escondida-frontend`
· **Status:** plan only, nothing changed yet · **Related:** `marketing-app/docs/multi-restaurant-plan.md`
(the WhatsApp agent's side)

## 0. Goal

The POS becomes a platform for businesses the operator does not own: restaurants first (3 to start,
one location each, all using the whole POS — edge box, DIAN invoicing, stock), other services later.

- **One database, one container, one URL** for every organization.
- The **token** decides whose data a request sees.
- **Isolation between tenants is a hard requirement**: a leak is a security incident between
  separate customers. The database enforces it, not only the application code.

## Decisions log

| Date | Decision |
|---|---|
| 2026-10-21 | Shared tenancy (organization_id + RLS). A deployment per tenant and a schema per tenant are rejected. |
| 2026-10-21 | The sign-in page keeps the hardcoded Laguna Escondida branding for now. Single sign-on comes later. |
| 2026-10-21 | The edge box keeps receiving users **with password hashes** (it needs them for offline sign-in), but only its own org's users. |
| 2026-10-21 | One container for now. `ProductLockManager` stays in memory; scaling out requires B.5's database lock first. |
| 2026-10-21 | The WhatsApp agent links to an organization only through that org's API key. There is no stored mapping. |
| 2026-10-21 | E-invoicing credentials move to the database, encrypted in the app with AES-256-GCM (key in Secrets Manager, org id as additional data). KMS envelope and per-org Secrets Manager secrets are rejected. |
| 2026-10-21 | The migration hardcodes Laguna's org id `bb0cad3e-be13-4bad-9a37-bba9525bfece` for the backfill. Its e-invoicing row is imported from the legacy envs at boot, not by SQL. |
| 2026-10-21 | Each request pins **one pooled connection** with a session-level `app.org_id` (reset when the request ends), not one transaction per request. A request-wide transaction would abort on any handled DB error, roll back writes that commit today, and fire "after commit" events before the commit. Nested `UnitOfWork`/`InTx` calls become savepoints on that connection. |
| 2026-10-21 | `pos_app` is a NOLOGIN group role created by migration; ops creates the login user (`CREATE ROLE … LOGIN IN ROLE pos_app`), so no password is in SQL. Migrations use `DB_MIGRATION_USER`/`DB_MIGRATION_PASSWORD`, which default to the app's credentials. |
| 2026-10-21 | The invoice and support-document prefixes live in `organization_einvoicing` (migration 67, Laguna seeded `LAG`/`DS`); `ELECTRONIC_INVOICE_PREFIX` and `SUPPORT_DOCUMENT_PREFIX` are no longer read. `invoice_sequences` is keyed `(organization_id, prefix)`, and the two prefixes of an org must differ so they never share a counter. A consecutive is claimed and assigned in one transaction. |
| 2026-10-21 | Order idempotency uses an `idempotency_keys` table, per node and not synced, in front of `OrderService.CreateOrder`. The key gets `organization_id` with the other tables in step 2. |

## 1. Starting point

There is no tenant column anywhere. The org comes from one `ORGANIZATION_ID` env var, and
`organizations` (m33) has one row (`bb0cad3e-…`, Laguna) that nothing references. The JWT has no org
claim. `ADMIN_API_KEY`, `NODE_SYNC_KEY`, `JWT_SECRET`, `MCP_AUTH_TOKEN` and the DIAN credentials are
each one global value.

Problems that exist **today** and become leaks once tenants share the service:
- **The sync pull returns every user with their password hash**, with no filter
  (`repository/sync_reference_repository.go:28-196`). `NODE_SYNC_KEY` is shared and identifies neither the node nor the org;
  `SyncPushRequest.NodeID` is ignored.
- **`OpenBillProductHub` is keyed by area only** (`sse/open_bill_product_hub.go:29`). Every org's "kitchen" would receive every org's lines.
- **Sync appliers upsert `bill_owners` by the customer's document number** (`bill_sync_applier.go:74,148`). One org would overwrite another's customer.
- **The app connects as `postgres`**, a superuser that bypasses RLS, and runs migrations as that same user.
- **About 100 repository call sites use `r.db.WithContext` directly**, ignoring the transaction in the context:
  - entirely direct: expense_category, expense, delivery_zone, supplier, supplier_catalog, role, product_ingredient
  - partly direct: ~15 sites in bill_repository, several in open_bill_repository, which also opens its own `Transaction`s at :281, :640 and :761
- Existing bugs to fix along the way:
  - ~~The support-document counter updates `prefix='DS'` (a constant), while the configured `SUPPORT_DOCUMENT_PREFIX` is what gets sent to the provider.~~ Fixed: both come from `organization_einvoicing`.
  - ~~`GetNextConsecutive` runs outside the caller's transaction.~~ Fixed: claimed and assigned in one transaction.
  - ~~Order creation ignores `X-Idempotency-Key`.~~ Fixed.
  - `proxy.ts` in the frontend leaves 7 sections unguarded.

## 2. Tenant model

```
organizations (exists)   id, slug, name, status, settings…          global, no RLS
users                    id, username UNIQUE, password_hash, active,
                         platform_role NULL | 'platform_admin'       global identity
memberships              id, user_id, organization_id, active        UNIQUE(user_id, organization_id)
membership_roles         membership_id, role_id                      replaces user_roles
roles                    unchanged: global, ids 1–6 hardcoded in permissions/role_permissions.go
```

- **Identity is global and membership is per org.** One user can belong to several orgs, with different
  roles in each. At launch everyone has exactly one membership.
- **`platform_admin`** onboards orgs and replaces `ADMIN_API_KEY` for anything that crosses orgs.
- **Every tenant-owned table gets `organization_id uuid NOT NULL REFERENCES organizations`.**
  Child tables carry it too, so every policy is one predicate with no join. The tables are:
  - products
  - product_ingredients, product_preparation_responsibilities
  - bills, bill_products, bill_owners
  - open_bills, open_bills_products, open_bill_product_side_dishes
  - invoice_sequences, pending_invoices
  - stock, historic_stock
  - suppliers, supplier_catalog
  - purchase_entries, purchase_entry_items
  - expenses, expense_categories
  - support_documents, support_document_products
  - delivery_zones
  - nodes, sync_outbox, sync_inbox, sync_state
- **Uniques become per org:**
  - `bill_owners` PK → `(organization_id, id)`
  - `invoice_sequences` PK → `(organization_id, prefix)`
  - `expense_categories.code` → `(organization_id, code)`, seeded per org at onboarding instead of by m36
  - `sync_outbox` UNIQUE → `(organization_id, origin_node_id, seq)`
- **Composite foreign keys** `(organization_id, parent_id) → parent(organization_id, id)` on the main
  relations make it impossible to attach one org's row to another's:
  - open_bills_products → open_bills and products
  - bill_products → bills and products
  - bills → bill_owners
  - stock and historic_stock → products
  - supplier_catalog → suppliers and products
  - purchase_entry_items → purchase_entries and products

## 3. Enforcement: three layers

1. **Token → context.**
   - Sign-in resolves the membership: automatic with one; with several, the response lists them and the client retries with `organization_id`.
   - The JWT carries `org_id`, plus the roles and permissions **of that membership**.
   - `JWTAuthMiddleware` puts the org in the context through a small domain package (`domain/tenant`:
     `WithOrg(ctx, id)`, `OrgFrom(ctx) (string, bool)`). Services already take `ctx` first, so the domain stays free of platform imports.
   - A token without `org_id` is rejected.
2. **Repositories filter explicitly.**
   - A GORM scope `tenant(ctx)` adds `organization_id = ?` to every tenant query.
   - Creates set the column from the context, never from a DTO.
   - A missing org in the context is an error, not "all rows".
3. **Postgres RLS is the backstop.** On every tenant table:
   ```sql
   ALTER TABLE x ENABLE ROW LEVEL SECURITY; ALTER TABLE x FORCE ROW LEVEL SECURITY;
   CREATE POLICY tenant ON x
     USING (organization_id = current_setting('app.org_id', true)::uuid)
     WITH CHECK (organization_id = current_setting('app.org_id', true)::uuid);
   ```
   - With no setting, a query sees no rows; it fails closed.
   - The app connects as a new non-owner role, `pos_app`. Migrations run as the owner through a separate DSN.
   - **Per-request connection:** `DBSessionMiddleware` pins one pooled connection the first time the request
     touches the database, runs `SELECT set_config('app.org_id', $1, false)` on it, and runs `RESET app.org_id`
     when the request ends (or discards the connection if the reset fails). `GetTxOrDB` picks the pinned connection up,
     statements still autocommit as before, and `UnitOfWork` and the repositories' own `InTx` calls begin their
     transaction on it, as a savepoint when one is already open. SSE streams skip it. Events are published
     through `DetachedEventBus`, so subscribers on other goroutines never share the request's connection.
   - SSE streams, crons and event handlers open a short transaction per operation, each with the setting. They never hold one open for a stream.
   - Cross-org work (the cron loop, onboarding) reads `organizations` (no RLS), then works org by org. Nothing in the app uses `BYPASSRLS`.

**Leak suite.** A testcontainers acceptance suite (`test/acceptance/tenancy`) seeds two orgs with mirrored data. For every endpoint,
MCP tool, sync push and sync pull, it acts as org A and asserts that nothing of org B can be read or written. Repository
integration tests run with RLS on and the app scope removed, to prove the backstop works on its own. The suite gates every release.

## 4. Per-organization configuration (out of env)

| Today | Becomes |
|---|---|
| Hardcoded `America/Bogota` (`electronic_invoice.go:237,412`, `sse_handler.go:20`, `handler/dateutil.go:10`, daily close) | `organizations.timezone` |
| `PHONE_HOME_REGION` | `organizations.country`, plus `.locale` and `.currency` for the frontend |
| `BUSINESS_NAME/NIT/ADDRESS`, `TICKET_FOOTER`, `TICKET_LEGAL_NOTICE` (edge, `config.go:391-395`) | organization business info; the edge receives it in the pull |
| Frontend branding constants | `organizations.branding` (display name, logo URL, primary colors) |
| `ELECTRONIC_INVOICE_URL/USER/PASSWORD`, `ELECTRONIC_INVOICE_PREFIX`, `SUPPORT_DOCUMENT_PREFIX` (`config.go:112-141`) | `organization_einvoicing`: provider URL, user, **encrypted** password (see below), invoice and support-document prefixes, resolution range |
| `NEXT_PUBLIC_ORDER_ACTION_PIN` (frontend, build-time) | per-org setting, stored hashed, checked by the API |
| `ORGANIZATION_ID` | cloud: removed. Edge: the box's org |
| Consumidor-final `"222222222222"` and phone `"00000000"` (`electronic_invoice.go:253,277`) | defaults per country |

**E-invoicing credentials are secrets.** The provider takes the password as Basic auth on every call
(`httpclient/electronic_invoice.go:341`), so it can't be hashed; it has to be decryptable. In plaintext, a DB dump,
snapshot restore or read-only connection would expose every tenant's DIAN credentials, and with them anyone could issue
legal invoices under that tenant's NIT. Today the credentials live in Secrets Manager (`aws-infra` `services-stack.ts:219`);
moving them into a table must not make them less protected.

- **App-side AES-256-GCM, key in Secrets Manager.** `organization_einvoicing` stores `password_ciphertext`, `nonce`,
  `key_version`, never the plaintext. One master key, `EINVOICE_ENCRYPTION_KEY`, is added to the existing `laguna/app`
  secret. A DB dump without the key is useless.
- **The ciphertext is bound to its org:** `organization_id` is the GCM additional data, so a value copied into another org's row fails to decrypt.
- **Domain port** `SecretCipher` (`Encrypt`/`Decrypt`); the AES implementation is an adapter, and services are tested with its mock.
- `key_version` allows rotating the key later without re-encrypting everything at once.
- **Handling:**
  - Write-only in the API: `GET` returns `password_set: true`, never the value.
  - Never logged, never exposed by an MCP tool.
  - Never sent to the edge; invoicing is cloud-only.
- Rejected:
  - **Plaintext:** see above.
  - **KMS envelope:** about the same protection, with an extra KMS call or cache and more moving parts. Overkill for 3 tenants.
  - **A Secrets Manager secret per org:** the app would need IAM rights to create and overwrite secrets during onboarding, and tests would need a fake.

## 5. Subsystems

**Crons** (`cron/scheduler.go:69-112`). `submitPendingInvoices`, `updateMissingDocumentURLs`,
`updateMissingSupportDocumentURLs` and `reconcileStock` loop over the active orgs, and the e-invoicing jobs only over orgs that
have e-invoicing configured. Each org gets its own transaction and credentials, and a failure stays isolated to that org.
Consecutives are assigned per `(org, prefix)`, in order within the org, inside the cron's transaction.

**Edge sync.**
- `nodes` gets `organization_id` and a hashed per-node key, which replaces `NODE_SYNC_KEY`.
  The middleware resolves the key to `(node_id, org_id)` and sets the tenant context.
- Push appliers **stamp** the node's org and ignore any org in the payload. Inbox, outbox and state are per org.
- The pull is filtered by the node's org. Users are sent **with their password hashes**, as today, but only those with
  an active membership in that org, together with that membership's roles.
  - A user who belongs to orgs A and B has the same hash on both boxes. That is accepted, since it is the same person.
  - When a membership is deactivated, the next pull must remove or deactivate that user on the box. Today's pull only adds or updates.
- The edge database has the same schema, holding one org.
- Node ids: existing ones keep the current derivation, so `sync_state` stays valid. New nodes get a stored random id instead of
  `SHA1("laguna-escondida/node/"+org+…)`.

**Real time and events.**
- `OpenBillProductHub` is keyed by `(org_id, area)`, with the org taken from the token.
- `sse.Hub` has no broadcaster; remove it.
- Watermill payloads (`order.*`, `purchase_entry.created`) carry `organization_id`.
  The stock handler opens its own tenant transaction, rather than relying on `msg.Context()`.
- `ProductLockManager` stays in memory because there is one container. Before running more than one,
  replace it with `SELECT … FOR UPDATE` or an advisory lock on `(org, product)`.

**API keys and the MCP server.**
- `api_keys(id, organization_id, role_id, hash, name, last_used_at, revoked_at)`. A key authenticates as a principal
  with a fixed org and role, for example `agent` (role 6). `IsAgentOnly` applies unchanged within the org.
- The MCP server becomes one multi-tenant process. It stops signing in as `LAGUNA_USERNAME/PASSWORD`, checks nothing
  itself, and forwards the caller's bearer (the org's API key) to the API.
  - The WhatsApp agent keeps one MCP URL, and each product's token is its org's API key.
  - `X-Customer-Phone` and `X-Customer-Location` are unchanged.
- The admin tools that use `LAGUNA_ADMIN_API_KEY` move behind platform-admin keys, or are dropped from the customer toolset.

**Storage.** S3 keys are already `{orgID}/…`. Take the org from the context instead of from config.

## 6. Frontend (`laguna-escondida-frontend`)

The browser only calls Next route handlers, which forward the cookie's token as a Bearer, so **the API calls need no change**.
What changes:
- **Sign-in page:** keeps the Laguna branding. It handles the "pick an organization" response, which no one gets at launch.
- **`/auth/me`:** returns the org (id, name, branding, timezone, locale, currency, business info) and the user's memberships.
  An `OrganizationProvider` exposes them.
- **Branding:** after sign-in, the header name, logo and title come from the org, and the colors are injected as CSS variables.
- **Formatting:** one `lib/format.ts` replaces the ~20 hardcoded `es-CO`/`COP` uses and the `America/Bogota` in the kitchen and daily close.
  The map center comes from the org.
- **Receipt:** prints the org's name, NIT and address.
- **`proxy.ts`:** every non-public route is protected.
- **Later, with single sign-on:** `POST /auth/switch-organization` reissues the token. The switcher closes any open `EventSource`.

## 7. Migrating Laguna (production data)

1. Restore a production snapshot and run the migration against it first. Migrations run at boot.
2. Take a snapshot, then pause the edge box's sync.
3. Run the migration:
   - Laguna's org id is hardcoded: `bb0cad3e-be13-4bad-9a37-bba9525bfece`. m33 already inserts that row, so the
     foreign key holds in production and in fresh (testcontainers) databases alike.
   - Per table: add `organization_id` with `DEFAULT 'bb0cad3e-be13-4bad-9a37-bba9525bfece'`, backfill, set `NOT NULL`, drop the default.
   - `invoice_sequences` already keeps its counters keyed `(bb0cad3e-…, prefix)` (migration 67).
   - Swap the uniques and keys.
   - Turn `user_roles` into `memberships` + `membership_roles`.
   - `pos_app` and its grants already exist (migration 65). Create the login user (`CREATE ROLE laguna_app LOGIN PASSWORD … IN ROLE pos_app`), point `DB_USER` at it and `DB_MIGRATION_USER` at the owner.
   - Laguna's `organization_einvoicing` row already exists with its prefixes (migration 67). Its credentials are **not**
     written by a migration, because the password must never appear in a SQL file. Instead there is a one-time import at boot:
     if Laguna's row has no credentials and the legacy `ELECTRONIC_INVOICE_URL/USER/PASSWORD` envs are set, the app
     encrypts them and fills the row. It is idempotent.
     The envs are deleted from `laguna/app` in the release after the import is confirmed in production.
     `ELECTRONIC_INVOICE_PREFIX` and `SUPPORT_DOCUMENT_PREFIX` can go as soon as migration 67 ships: drop them from the
     task definition first, then from the secret, or the task fails to start.
4. Ship the backend, edge and frontend together, and give the edge box its per-node key.
   Old tokens have no `org_id` and are rejected, so everyone signs in once more.
5. Enable RLS only after the leak suite passes with the app scopes in place. In staging, also run it with a scope deliberately removed.

## 8. Order of work

1. **Groundwork, no behavior change:**
   - [x] every repository uses `GetTxOrDB`
   - [x] `UnitOfWork` reuses an outer transaction (as a savepoint)
   - [x] per-request connection middleware (`app.org_id` from `ORGANIZATION_ID` until step 3)
   - [x] the `pos_app` role with a separate migration DSN (migration 65)
   - [x] prefixes per organization in `organization_einvoicing`, counters per `(organization, prefix)` (migration 67); fixes the support-document prefix bug
   - [x] claim and assign the invoice consecutive in one transaction
   - [x] order idempotency (`X-Idempotency-Key`, migration 66)
2. **Schema:** `organization_id`, the new uniques and composite foreign keys, memberships, and the org settings columns.
3. **Auth:** the `org_id` claim, membership-scoped roles, `domain/tenant`, the middleware, `platform_admin`.
4. **Repository scopes and the leak suite**, then RLS.
5. **Per-org configuration:** timezone and formatting, business info, encrypted e-invoicing; crons loop per org.
6. **Sync:** per-node keys, an org-filtered pull, user removal on the box. Also the hub keyed by `(org, area)` and events carrying the org.
7. **Org API keys and the multi-tenant MCP server.** Switch the WhatsApp agent to the global MCP URL.
8. **Frontend:** org context, branding, formatting, receipt, route guard.
9. **Onboarding:** a platform-admin flow that creates the org, its settings and e-invoicing, its first admin and its agent API key.
   Then onboard restaurants 2 and 3.

Per this repo's rules:
- Each step ships with service tests.
- `docs/api/*.md` is updated for every changed endpoint: sign-in, `/auth/me`, API keys, platform admin, sync.
- Existing tests that change because of memberships, such as the user service and the sync appliers, need sign-off before they are edited.

## 9. Open questions

1. How is the frontend's cloud build deployed today? The repo has no Netlify config. Does it move to ECS?
2. Which DIAN provider fields vary per org besides the URL, credentials and prefixes (resolution number, technical key, test-set id)?
