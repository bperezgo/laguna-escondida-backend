Backend commands run from `backend/`. Read only the files in design "Implementation map". Acceptance tests need the `DB_HOST=` prefix to use testcontainers.

## 1. Groundwork

- [x] 1.1 Read the create paths of `product_service.go`, `product_ingredient_service.go`, `plate_option_slot_service.go` and the responsibility and stock services, and write down in design.md (D4) anything they do on create beyond the insert (sync bookkeeping, events, movements). Verify: design.md D4 lists the findings, or states there are none.
- [x] 1.2 Add `catalog:import` to `permissions.go` and grant it only to `RoleAdmin` in `role_permissions.go`. Verify: permission tests assert admin holds it and manager, cooker, accountant, server, agent and courier do not.
- [x] 1.3 Add the import DTOs in `domain/dto/catalog_import.go` (document, defaults, existing, entries, problem, summary, result) with concrete types only. Verify: `go build ./...` passes.
- [x] 1.4 Confirm `sku-suggestion` is merged, then use its generator (`aggregate/product/sku_generate.go`) for D6 with the taken set described there. Verify: service tests for "Generated SKU avoids collisions" and for a generated SKU that would have taken a SKU given later in the document.
- [x] 1.5 Add any missing repository port methods: batched product lookup by names and SKUs scoped to the organization, and bulk inserts for products, ingredients, slots and responsibilities if absent. Run `make generate-mocks`. Verify: `go build ./...` passes and the mocks are regenerated, not hand-edited.

## 2. Validation

- [x] 2.1 Implement document validation in `catalog_import_service.go`: refs (`duplicate_ref`, `unknown_ref`), `existing` resolution (`existing_not_found`), conflicts with the organization and inside the document (`name_conflict`, `sku_conflict`, trimmed and case-insensitive), collecting every problem with its JSON-pointer path. Collect every ref before resolving any link, so order in the document never matters. Verify: service tests for each scenario of "Catalog import document" (including "Plate defined before its ingredient") and "Create and reference, never edit", including several problems reported at once.
- [x] 2.2 Add link and slot rules: items resolve to `INGREDIENT` or `BOTH` (`invalid_link_type`), positive recipe quantities, positive integer choice quantities (`invalid_quantity`), slot name, at least one choice and exactly one default (`slot_default_count`), empty choices allowed; reuse the domain validators for product fields and slots. Verify: service tests for each scenario of "Product links follow product types".
- [x] 2.3 Apply `defaults` and report fields missing from both entry and defaults; generate missing SKUs; validate provided SKUs with the SKU value object. Verify: service tests for each scenario of "Defaults and generated SKUs".
- [x] 2.4 Validate `opening_stock` (refs created by the document only, `stock_on_existing`, non-negative amounts) and the count limits of D11 (`limit_exceeded`). Verify: service tests for "Count on an existing product refused" and "Too many products", plus one test per other cap.
- [x] 2.5 Build the summary (counts per kind, generated SKUs) and the digest over the canonicalized document (D5). Verify: tests that the same document with keys in another order gets the same digest, and a changed price gets a different one.

## 3. Apply

- [x] 3.1 Implement apply in one `UnitOfWork.Do`: check `expected_digest` (`digest_mismatch`), re-validate inside the transaction, then write in the D4 order and record opening counts as count movements. Return ref → id and SKU. Do whatever 1.1 found the per-entity services do on create. Verify: service tests for "Successful apply", "Apply with a problem writes nothing", "Digest mismatch", and a write failing halfway that leaves nothing behind.

## 4. HTTP

- [x] 4.1 Add `catalog_import_handler.go` with `POST /api/catalog/import/validate` and `POST /api/catalog/import`: a 2 MiB `http.MaxBytesReader` (413), unknown fields rejected (400), domain errors mapped. Verify: handler tests for "Unknown field", "Oversized body", a valid validate response and an apply response.
- [x] 4.2 Wire the service and handler in `bootstrap/cloud.go` and register both routes in `routes_cloud.go` behind `JWTAuthMiddleware` and `RequirePermission(catalog:import)`; update `testdata/routes_cloud.golden`. Verify: the golden route test passes, and the edge route list does not gain the routes.
- [x] 4.3 Write `docs/api/catalog-import.md` in the `products.md` format (endpoint table, request and response with realistic JSON, every problem code, errors) and add a curl example to `docs/examples/curl-examples.md`. Verify: every problem code in the service appears in the doc.

## 5. Acceptance

- [x] 5.1 Add `test/acceptance/catalogimport/` covering: a full document (ingredients, sellables, a plate with recipe and slot, opening stock) validated then applied; the same document applied twice; a document referencing an existing product. Verify: `DB_HOST= make test-acceptance` passes; the test also records how long a 1,000-product apply takes.
- [x] 5.2 Add an acceptance test that an imported catalog reaches an edge through sync exactly like a hand-built one (products, recipes, slots, responsibilities, stock). Verify: it passes with the acceptance target.

## 6. MCP server and playbook

- [x] 6.1 Add `tools_catalog_import.go` with `validate_catalog_import` and `apply_catalog_import` (inline `document` or `file_path`, `expected_digest` required on apply), registered in the full toolset only. Verify: tool tests that both forward the body and return the response unchanged, and that apply without `expected_digest` fails.
- [x] 6.2 Accept `file_path` only on the stdio transport and cap file reads at 2 MiB. Verify: tool tests that on HTTP a `file_path` fails before any read and without a backend call, that a 3 MiB file fails, and that a valid file on stdio is validated.
- [x] 6.3 Correct the stock tool descriptions in `tools_stock.go` to say cloud only. Verify: a test asserts no tool description contains "edge mode only".
- [x] 6.4 Write `internal/playbooks/onboard-catalog.md` following the "Onboarding playbook" requirement (defaults confirmed, recipes and slots plate by plate, units normalized, validate and fix, explicit approval, apply with the digest, optional opening stock). Verify: `internal/playbooks` tests pass and `get_playbook onboard-catalog` returns it.
- [ ] 6.5 Run a real onboarding from Claude Desktop with a sample menu (photo or spreadsheet) through the playbook against a local cloud. Verify: the catalog appears in the operations UI and an order with a slot choice decrements the right ingredients.

## 7. Negative security tests

- [x] 7.1 Handler or acceptance test "Unauthenticated caller": both endpoints answer 401 without a token and write nothing.
- [x] 7.2 Handler or acceptance test "Role without the permission": manager, cooker and agent users get 403 on both endpoints.
- [x] 7.3 Acceptance test "Another organization's product cannot be referenced": an `existing` SKU of organization B, used by an admin of organization A, gives `existing_not_found`, writes nothing, and the response holds nothing of B's product.
- [x] 7.4 Route test "Edge node": both endpoints answer 404 in edge mode.
- [x] 7.5 Tool test "Customer toolset": neither import tool is registered with the customer toolset.
- [ ] 7.6 Confirm 4.1 (oversized body, unknown field), 2.4 (`limit_exceeded`), 3.1 (`digest_mismatch`) and 6.2 (`file_path` over HTTP) cover their negative scenarios. Verify: each scenario name maps to a test name in a short list added to the PR description.

## 8. Finish

- [x] 8.1 Run `make lint`, `go build ./...`, `go test ./...` and `DB_HOST= make test-acceptance` from `backend/`. Verify: all pass, or remaining lint findings are only in files this change did not touch.

## 9. Security review

- [x] 9.1 Run the security-audit skill in guidance mode (focused review, not the full audit) over this change's diff against design.md "Security". Fix confirmed findings, and record the needs-validation items (proxy body limit, the deployed MCP server's user and role) in design.md with how they were checked or who will check them. Verify: design.md Security is updated and no confirmed finding is left open.
