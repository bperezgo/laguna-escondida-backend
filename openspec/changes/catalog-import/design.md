## Context

See proposal.md for the motivation. Constraints in the current code that shape the approach:

- The catalog is a graph whose links are product UUIDs: `product_ingredients` (composite → ingredient, `default_quantity`), plate option slots (`PUT /api/products/:id/option-slots`, each choice a bundle of ingredient items with **integer** quantities, exactly one default per slot), and preparation responsibilities (`product_id`, `area`, `priority`). Building it through today's endpoints needs ids read back between calls.
- Products have no database uniqueness on name or SKU. `ProductRepository.FindByName` exists. The `SKU` value object (`aggregate/product/sku.go`) accepts letters, digits and `-`. SKU generation comes from the `sku-suggestion` change (`product-sku` capability).
- Product type decides what can link to what: `INGREDIENT` and `BOTH` can be recipe or choice items, while `COMPOSITE` holds recipes and slots.
- Stock writes are registered only on the cloud (`registerStockRoutes`, `authoring`). Every write records a movement, and a count becomes `counted - on-hand` (`cloud-stock-ownership`). The MCP stock tool descriptions still say "edge mode only", which is the opposite of the truth.
- The MCP server picks a toolset at startup (`full` or `customer`) and already refuses the customer toolset over stdio. Playbooks are embedded markdown (`internal/playbooks/*.md`).
- `ports.UnitOfWork.Do(ctx, fn)` gives one transaction.

## Goals / Non-Goals

**Goals:**
- One request validates or applies a whole catalog, with every problem reported at once and a path pointing at the offending entry.
- Apply is all-or-nothing, and a repeated apply cannot double-create.
- An operator's AI client can drive it end to end through two MCP tools and a playbook.

**Non-Goals:**
- Updating, merging or deleting existing products through an import.
- Parsing menus, PDFs or spreadsheets on the server.
- Persisting drafts, a review UI or an in-app agent (Phase 2).
- Supplier catalog links and purchase entries.

## Decisions

### D1. Two endpoints, not a `dry_run` flag

`POST /api/catalog/import/validate` and `POST /api/catalog/import` take the same body. A forgotten query parameter must never turn a check into a write. Both are cloud-only routes.

*Alternative:* `?dry_run=true` on one path. Rejected because the default, when the parameter is missing, would be the dangerous action.

### D2. Document shape: local refs plus an explicit `existing` list

```
{
  "defaults":     { "vat", "ico", "taxes_format", "unit_of_measure", "prep_area", "prep_priority" },
  "existing":     [ { "ref", "sku" | "name" } ],
  "ingredients":  [ { "ref", "name", "category", "unit_of_measure", "product_type": INGREDIENT|BOTH,
                      "total_price_with_taxes"?, "sku"?, ... } ],
  "sellables":    [ { "ref", "name", "category", "total_price_with_taxes", "sku"?, "prep"?, ... } ],
  "plates":       [ { "ref", "name", "category", "total_price_with_taxes", "sku"?, "prep"?,
                      "recipe": [ { "ingredient": ref, "quantity" } ],
                      "slots":  [ { "name", "choices": [ { "name", "default"?,
                                    "items": [ { "ingredient": ref, "quantity": int } ] } ] } ] } ],
  "opening_stock": [ { "product": ref, "counted" } ]
}
```

Refs are unique within the document and can only be used inside it. A product that already exists is reachable **only** through an `existing` entry, which resolves by SKU (exact match) or name (trimmed, case-insensitive) within the caller's organization. Matching is never implicit: a new entry whose name or SKU equals an existing product, or another new entry, is a conflict rather than a silent reuse. This is the create-and-reference rule from the proposal, and it means a second apply of the same document fails on conflicts instead of duplicating.

Splitting the product lists by role (`ingredients`, `sellables`, `plates`) instead of one list with a type lets the schema forbid recipes on sellables and slots on ingredients structurally.

*Alternative:* match existing products by name automatically. Rejected, because a typo or a menu that reuses a name ("Limonada") would silently attach recipes to the wrong product.

### D3. Validation collects every problem, it does not stop at the first

The validator walks the whole document and returns `{ valid, problems: [ { path, code, message } ], summary }`, where `path` is a JSON pointer such as `/plates/3/slots/0/choices`. Codes are stable strings (`duplicate_ref`, `unknown_ref`, `name_conflict`, `sku_conflict`, `existing_not_found`, `invalid_link_type`, `slot_default_count`, `invalid_unit`, `invalid_quantity`, `stock_on_existing`, `limit_exceeded`, ...), so the AI client can fix problems programmatically. `summary` gives counts per kind to show the operator before applying.

The rules are the same as the per-entity endpoints (product field validation, the `SKU` value object, the slot rules of `plate-option-slots`), reused from the domain and not re-implemented. Additional rules:

- Recipe and choice items must resolve to `INGREDIENT` or `BOTH`, so a plate can never contain a plate and there are no cycles.
- Choice quantities are positive integers. The playbook tells the client to normalize to g, ml or unit.

### D4. Apply: one unit of work, fixed order, re-validate inside

`apply` runs the full validation again **inside** the transaction (against the same organization snapshot) and writes nothing if any problem is found. The order is: ingredients → sellables and plates → recipes → option slots → preparation responsibilities → opening-stock movements. The import service calls repositories and domain validators directly in one `UnitOfWork.Do`. It does not call the per-entity services, because those may open their own transactions. If a per-entity service does work on create that the import would otherwise skip (sync bookkeeping, events), the import must do the same. The implementation task verifies this for products, ingredients, slots and responsibilities before writing.

Findings (task 1.1):

- **Products** (`ProductService.CreateProduct`): builds the aggregate (`NewAggregateFromCreateProductRequest`, which validates name, category, SKU, type, unit and, for sellable types, price and taxes), then `ProductRepository.Create` and, optionally, `CreatePreparationResponsibility` in one unit of work. No events, no outbox, no other bookkeeping.
- **Recipe lines** (`ProductIngredientService.AddIngredient`): checks the plate is `COMPOSITE`, no self-reference, no duplicate ingredient, a positive decimal quantity and no cycle, then `ProductIngredientRepository.Create`. No side effects. The import enforces "items are `INGREDIENT` or `BOTH`", which rules out cycles without a graph walk.
- **Option slots** (`PlateOptionSlotService.Replace`): `validateSlotInputs` plus `buildSlots` (ids, positions, trimmed names), then `ReplaceForCompositeProduct`. No side effects. The import reuses `validateSlotInputs` and `buildSlots`.
- **Responsibilities**: `ProductRepository.CreatePreparationResponsibility` only.
- **Stock** (`StockService.BulkStockCreationOrUpdating`): writes the stock row (`BulkCreateOrUpdate`), then for each count calls `recordMovement`, which assigns an `op_id`, writes the `historic_stock` row with kind `count` and hands it to `StockMovementEmitter` (a no-op on the cloud). The import calls the same `recordMovement`.
- **Sync**: catalog rows reach edges through the reference pull, which reads rows whose `updated_at` is newer than the edge's cursor. Nothing is enqueued on create. The cursor is the latest `updated_at` the pull returned. If some other write commits during a long import with a later timestamp, an edge could move its cursor past the import's rows before they are visible. Every multi-row transaction already has this risk, and onboarding runs with no other writers, so it is accepted and not handled here.

### D5. Digest binds the operator's approval to what was validated

`validate` returns `digest`: the SHA-256 of the canonicalized document (sorted keys, defaults applied). `apply` requires `expected_digest` and refuses a mismatch with `digest_mismatch`, so the AI client can't apply a document that changed after the operator saw the summary. It is a guard against drift between turns, not an authentication mechanism.

### D6. SKU generation

A missing SKU is generated with the rule of the `product-sku` capability (from `sku-suggestion`): area prefix plus up to three name words, for example `COC-BAN-PAI`, using the backend generator that change adds. The entry's prep area (after `defaults`) gives the prefix, and the taken SKUs are the organization's products plus every SKU already given or generated earlier in the document, in document order (ingredients, then sellables, then plates). Generated SKUs are returned in the result and in the dry-run summary.

### D7. Opening stock only for products this import creates

`opening_stock` entries must point at refs created by the document. A ref from `existing` gets `stock_on_existing`. Each entry is recorded as a count movement (`counted - on-hand`, where on-hand is 0 for a new product), through the same path as other cloud counts. Counting stock for existing products stays with the stock screen, which has the sync-freshness warning that a new product does not need.

### D8. Defaults

`defaults` fills `vat`, `ico`, `taxes_format`, `unit_of_measure` and the prep `area`/`priority` for any entry that leaves them out. There is no server-side guess, so a field missing from both the entry and the defaults is a validation problem. The playbook proposes the usual Colombian restaurant values (ICO 8 %, VAT 0) and asks the operator to confirm them.

### D9. Permission

New permission `catalog:import`, granted to `RoleAdmin` only. Both endpoints require it, because the dry run reads the organization's catalog to report conflicts. The `agent` role does not get it.

### D10. MCP tools and playbook

- `validate_catalog_import` and `apply_catalog_import` are registered in the **full** toolset only.
- Input is either `document` (inline object, typed as the import DTO) or `file_path` (a JSON file on the MCP host, for large menus). `file_path` is accepted **only on the stdio transport**, which runs on the operator's own machine. Over HTTP it is refused before any read. Files are read with a size cap equal to the endpoint's body cap.
- `apply_catalog_import` requires `expected_digest`.
- New embedded playbook `onboard-catalog.md` covers: read the menu or spreadsheet → propose defaults → ask about plates' options and recipes → build the document (or write it to a file) → validate → fix problems → show the summary and wait for the operator's explicit approval → apply with the digest → optionally count opening stock.
- Stock tool descriptions are corrected to say "cloud only".

### D11. Limits

Body cap 2 MiB on both endpoints (`http.MaxBytesReader`, as `lead_handler.go` does). Count caps: 1,000 products in total, 50 recipe items per plate, 20 slots per plate, 30 choices per slot, 50 items per choice, 1,000 opening-stock entries (opening stock only applies to products the document creates, so it can't usefully exceed the product cap). Exceeding a cap is a `limit_exceeded` problem (or HTTP 413 for the body).

The caps are sized to fit inside the body cap. Estimated sizes: a sellable or ingredient takes about 250 B, and a plate with 5 recipe lines and 2 slots of 3 choices about 1.2 KB. A typical restaurant (about 300 products, 80 of them plates) comes to about 150 KB, a large menu (about 800 products) to about 400 KB, and the worst case allowed (1,000 heavy plates) to about 1.2 MB. Name lookups for `existing` and conflicts are batched, never one query per entry.

## Security

| Boundary | Lower-trust principal | Input/action it controls | Control and where enforced | Protected resource | If the control fails |
|---|---|---|---|---|---|
| Import endpoints | Authenticated user of an organization | Whole import body, both endpoints | `JWTAuthMiddleware` + `RequirePermission(catalog:import)` in `routes_cloud.go`; `catalog:import` only on `RoleAdmin` in `role_permissions.go` | The organization's catalog and stock | A manager, cook or the `agent` role could create products or counts |
| Tenant scoping of refs | Authenticated admin of org A | `existing[].sku` / `name`, and conflict lookups | All lookups go through organization-scoped repositories using the tenant from the JWT context (`postgres/tenant.go`); the import service never takes an organization id from the body | Org B's products and stock | Org A links recipes to, or learns of the existence of, org B's products (cross-tenant read/write) |
| LLM-produced body | Operator's AI client (model output) | Size, cardinality, nesting, names, quantities | D11 limits in the import handler and validator; typed DTO with unknown fields rejected; all values through domain validators | Cloud API availability, DB | Memory or CPU exhaustion from a huge document; query fan-out from per-entry lookups |
| Approval binding | Model in a later turn | Document applied after the operator approved a summary | D5 digest checked in the import service before the transaction | Operator's intent | A changed document (other prices, extra products) is applied under the operator's approval |
| MCP `file_path` | Caller of the MCP HTTP endpoint (token holder or prompt-injected model) | A path on the MCP host | Refused unless the server runs on stdio (`mcpserver` tool handler, same pattern as `Config.ValidateTransport`); size-capped read | Files on the cloud MCP host | Arbitrary file read on the server, with contents sent to the API or echoed in errors |
| MCP toolset | Customer-facing WhatsApp agent | Calling the import tools | Tools registered only in the full toolset; the endpoint refuses the `agent` role anyway | Catalog | A customer conversation could inject products |

Attack classes considered (AI-AND-LLM, DATA-ISOLATION-AND-LIFECYCLE, RESOURCE-EXHAUSTION-AND-AVAILABILITY, WEB-PROTOCOL-AND-AUTH): tool-argument injection into a downstream sink, excessive agency and confused deputy, action-confirmation binding, tool-schema and dispatcher disagreement, MCP metadata as policy (tool descriptions are guidance, the permission check is the control), cross-tenant object reference, database query amplification, and oversized input. Menu content is untrusted text that becomes product names. It is stored and later rendered by the frontend and printed on tickets, so it gets the same treatment as names entered by hand (existing output encoding), and no new rendering path is added.

Needs validation:
- **Proxy body limits in front of the cloud API** (outside the repo, aws-infra). A proxy limit under 2 MiB would reject large imports before the handler. Next.js App Router route handlers (the web app's proxy routes) have no built-in body cap, so any limit would come from the infrastructure. Check: send a 1.5 MiB document through the cloud web app and directly to the API host, and record which layer refuses it, if any (an AWS load balancer has no body cap; an nginx has a 1 MB default).
  - *Checked in the repo (task 9.1):* the frontend has no proxy route for `/api/catalog/import` (no frontend in this phase), and the edge's Caddy is not in the cloud path. The MCP server and curl call the API host directly, so only aws-infra can cap the body. Measured size: a 1,000-product document is about 190 KB, well under 1 MB, so even an nginx default would not block realistic imports. **Owner:** the operator checks aws-infra's listener and any reverse proxy before the first large onboarding.
- **Token held by the HTTP MCP server deployment.** If a cloud-hosted MCP server holds an admin token, every holder of the MCP endpoint token gets `catalog:import`. Check which user the deployed MCP server authenticates as and its role.
  - *Not decidable from the repo (task 9.1):* the service account comes from `LAGUNA_EMAIL`/`LAGUNA_USERNAME` set at deploy time. **Owner:** the operator confirms the deployed HTTP MCP server's user is not an admin, or accepts that `MCP_AUTH_TOKEN` holders can import. On the HTTP transport `file_path` is refused either way.

Security review (task 9.1, focused review of this change's diff):

- **Fixed — decimal magnitude exhaustion.** shopspring `decimal` accepts JSON numbers such as `1e2000000000`. Encoding one for the digest or comparing it allocates in proportion to its exponent. Measured: `1e10000000` alone took 3.9 s and 158 MB, so a 2 MiB body of such values could exhaust the cloud API. `refuseOversizedDecimals` now runs before the digest and every rule, refusing an exponent outside ±12 or a coefficient over 96 bits. Covered by `TestCatalogImportService_HugeExponentsAreRefusedBeforeAnyWork`.
- **Holds — authorization.** Both routes sit behind `JWTAuthMiddleware` + `RequirePermission(catalog:import)`. Organization API keys carry only the agent role, so they get 403. Route tests cover 401, 403 for manager, cooker and agent, and 404 on the edge.
- **Holds — tenant scoping.** The catalog is read with `FindAll` through `TenantDB`. Every insert stamps `organization_id` from the context (`TenantOwned`), and the document carries no organization or product id. Acceptance test: organization B's SKU gives `existing_not_found`, and nothing of B appears in the response.
- **Holds — approval binding.** The digest is checked before the transaction, and validation runs again inside it.
- **Holds — MCP file reads.** `file_path` needs `WithLocalFiles`, which `cmd/mcp-server` passes only with `--stdio`. Reads are capped at 2 MiB and decoded with unknown fields refused.
- **Outside this change, noted for owners:** the per-entity product endpoints parse prices with `decimal.NewFromString`, likely with the same exponent cost. The pre-existing `extract_archive` and `upload_*_document` MCP tools read host paths on the HTTP transport too. Neither was changed here.

## Risks / Trade-offs

- [The import bypasses per-entity services, so it could skip bookkeeping they do] → D4 makes the implementation verify each service's create path first. Acceptance tests check that an imported catalog syncs to an edge exactly like one built by hand.
- [Validation rules drift from the per-entity endpoints] → Reuse the domain validators. Add a test that a document rejected by the import is also rejected entry by entry through the existing endpoints.
- [A 1,000-product document in one transaction holds locks for a while] → Imports run during onboarding, when nobody is selling. Accept it and measure the time in the acceptance test.
- [A proxy in front of the API caps bodies below 2 MiB] → Accept a gzip-compressed body (`Content-Encoding: gzip`), with separate limits on the compressed and the unpacked size, which keeps one request, one transaction and one digest. Chunked upload to a server-side draft only if Phase 2 brings server drafts for their own sake.
- [Name matching for conflicts is case- and space-insensitive, while the existing endpoints allow such near-duplicates] → Conflict detection is deliberately stricter than the per-entity endpoints. The operator can rename the entry or point it at the existing product.
- [Generated SKUs collide with ones the operator typed later in the document] → Given SKUs are reserved before any SKU is generated, so a generated one never takes a SKU written further down.

## Migration Plan

No schema change. Deploy order: backend (new routes and permission; admin roles get `catalog:import` from the role map), then the MCP server binary with the new tools and playbook. Rollback means removing the routes and tools. Imported products are ordinary products and stay.

## Implementation map

Backend (`backend/`):

- New `internal/domain/dto/catalog_import.go` (document, problem, summary, result)
- New `internal/domain/service/catalog_import_service.go` + `_test.go` (validate, digest, SKU generation, apply)
- `internal/domain/ports/product_repository.go` (batched name/SKU lookup, if missing), `plate_option_slot` and product-ingredient and responsibility repository ports (bulk insert, if missing); `make generate-mocks`
- `internal/domain/service/product_service.go`, `product_ingredient_service.go`, `plate_option_slot_service.go`, `stock_service.go` (read only, to reuse validators and confirm the create paths per D4)
- `internal/domain/aggregate/product/sku_generate.go` (generator from `sku-suggestion`, used here, not re-written)
- `internal/domain/error/` (new `catalog_import_error.go`)
- `internal/domain/permissions/permissions.go`, `role_permissions.go` (+ tests)
- New `internal/platform/handler/catalog_import_handler.go` + test
- `internal/bootstrap/cloud.go`, `routes_cloud.go`, `testdata/routes_cloud.golden`
- `internal/platform/mcpserver/` new `tools_catalog_import.go` + test; `tools_stock.go` descriptions; `server.go` registration (full toolset only)
- New `internal/playbooks/onboard-catalog.md`
- New `test/acceptance/catalogimport/`
- New `docs/api/catalog-import.md`; `docs/examples/curl-examples.md`

Frontend: none.
