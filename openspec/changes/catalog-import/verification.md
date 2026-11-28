# Verification: catalog-import

Commit `4c7e960` · base `main` · 2026-11-27

## Verdict
Ready for human review: every scenario has a test that checks its outcome, and all checks pass. Two tasks are still open, and one untested risk (two applies at the same time) remains. Scenarios 32 proven, 0 weak, 0 missing. Tasks 27/29 checked, 0 false claims. Checks: all green.

## Read these yourself
1. `backend/internal/domain/service/catalog_import_service.go:107-154` (Apply): this is the only atomicity and approval guard. The digest is checked before the transaction, and validation runs again inside it against `FindAll`. Nothing locks the catalog, and products have no DB uniqueness on name or SKU. Two applies of the same document at the same moment (a double-click, or a retried MCP call) can both pass validation and duplicate the catalog. The "repeated apply" tests only cover applies one after the other. Design D4/Risks accepts this because onboarding has no other writers, so decide whether that is enough.
2. `backend/internal/domain/service/catalog_import_plan.go` (774 lines, all new): the validator. Look at `assignSKUs` (:413-451). SKU conflict checks ignore case through `takenSKUs`, but `existing` SKU matching is exact (`bySKU`, :112). Also look at `checkNames` (:453) and `checkSlots` (:549-613), which re-implement slot rules before also calling `validateSlotInputs`. Rule drift with the per-entity endpoints would start here.
3. `backend/internal/domain/service/catalog_import_service.go:249-316` (digest): canonicalization uses struct marshalling, not "sorted keys" as D5 says. Defaults are folded in, and the default prep area is folded into ingredients too, although ingredients never get one. That is harmless, but it is the approval binding, so check that every field that matters reaches the hash.
4. `backend/internal/platform/mcpserver/tools_catalog_import.go:85-121`: the `file_path` gate (`localFiles`) and the capped read. The only place that turns the gate on is `cmd/mcp-server/main.go:66-69` (`--stdio`).
5. `backend/internal/platform/postgres/repository/product_repository.go:120-166`: the existing `Create` was refactored into `newProductModel`, shared with the new `CreateMany`. It looks behavior-preserving, but it changes a hot existing path.
6. `backend/internal/bootstrap/routes_cloud.go:230-236` and `cloud.go:302-305`: the route guards and wiring, flagged by risk-areas (auth/deploy). Route tests cover them.

## Scenarios → tests
| Scenario | Rating | Test | Assertion on THEN |
|---|---|---|---|
| Catalog import document: Plate references an ingredient by ref | proven | `catalog_import_service_test.go:128` TestCatalogImportService_Validate_PlateReferencesIngredientByRef | `assert.True(t, result.Valid, ...)` |
| Catalog import document: Plate defined before its ingredient | proven | `catalog_import_service_test.go:141` …PlateDefinedBeforeItsIngredient; `:745` …Apply_PlateDefinedBeforeItsIngredientLinksTheCreatedIngredient | `assert.True(t, result.Valid)`; `assert.Equal(t, result.Products["frijol"].ID, recipe[0].IngredientProductID)` |
| Catalog import document: Duplicate ref | proven | `catalog_import_service_test.go:154` …DuplicateRef | `requireProblem(t, result, "/sellables/0", ProblemDuplicateRef)` + `Len(Problems, 1)` |
| Catalog import document: Unknown ref | proven | `catalog_import_service_test.go:168` …UnknownRef | `requireProblem(t, result, "/plates/0/recipe/0", ProblemUnknownRef)` |
| Catalog import document: Unknown field | proven | `catalog_import_handler_test.go:91` TestCatalogImportHandler_UnknownField | `assert.Equal(t, http.StatusBadRequest, rec.Code)` on both paths; strict repo mocks fail on any read or write |
| Dry-run: Several problems reported at once | proven | `catalog_import_service_test.go:180` …SeveralProblemsReportedAtOnce | both `requireProblem` at `/plates/0/recipe/0` and `/plates/1/slots/0`, `Len(Problems, 2)`; strict mocks, no write expectations |
| Dry-run: Valid document | proven | `catalog_import_service_test.go:106` …ValidDocument; `test/acceptance/catalogimport/catalog_import_test.go:216` | `assert.Len(t, result.Digest, 64)`, summary counts; `assert.Zero(t, r.count("products", r.orgA, ""), "validation writes nothing")` |
| Atomic apply: Successful apply | proven | `catalog_import_service_test.go:648` …Apply_SuccessfulApply; acceptance `:216` | `require.Len(t, result.Products, 6)`, recipe, slot and choice ids checked; acceptance `r.count("products"...) == 6`, slots read back from DB |
| Atomic apply: Apply with a problem writes nothing | proven | `catalog_import_service_test.go:799` …Apply_WithAProblemWritesNothing | `requireProblem(... "/plates/1/slots/0", ProblemSlotDefaultCount)`, `m.products.AssertNotCalled(t, "CreateMany", ...)` |
| Atomic apply: Digest mismatch | proven | `catalog_import_service_test.go:822` …Apply_DigestMismatch; `catalog_import_handler_test.go:150` | `assert.Equal(t, []string{ProblemDigestMismatch}, problemCodes(result))`, `AssertNotCalled(... "CreateMany")`; handler 409 |
| Atomic apply: Repeated apply does not duplicate | proven | acceptance `catalog_import_test.go:269` …SameDocumentAppliedTwice | `assert.Equal(t, 6, conflicts)`, `r.count("products") == 6`, `r.count("historic_stock") == 1` |
| Create and reference: Plate reuses an existing ingredient | proven | `catalog_import_service_test.go:768`; acceptance `:291` | `assert.Equal(t, limon.ID, recipe[0].IngredientProductID)`; acceptance `assert.Equal(t, before.UpdatedAt, after.UpdatedAt)` |
| Create and reference: New entry collides with an existing product | proven | `catalog_import_service_test.go:256` …NewEntryCollidesWithAnExistingProduct | `requireProblem(t, result, "/sellables/0", ProblemNameConflict)` |
| Create and reference: Another organization's product cannot be referenced | proven | acceptance `catalog_import_test.go:321` | `Equal(ProblemExistingNotFound, ...)`, `Zero(r.count("products", r.orgA))`, `NotContains(body, secretID)`, `NotContains(body, "Salsa secreta")` |
| Links: Plate inside a plate | proven | `catalog_import_service_test.go:310` …PlateInsideAPlate | `requireProblem(t, result, "/plates/1/recipe/0", ProblemInvalidLinkType)` |
| Links: Fractional choice quantity | proven | `catalog_import_service_test.go:359` …FractionalChoiceQuantity | `requireProblem(t, result, "/plates/0/slots/0/choices/0/items/0", ProblemInvalidQuantity)` |
| Links: Slot without a default | proven | `catalog_import_service_test.go:373` …SlotDefaults ("no default") | `requireProblem(t, result, "/plates/0/slots/0", ProblemSlotDefaultCount)` |
| Defaults: Default tax applied | proven | `catalog_import_service_test.go:711`; acceptance `:258` | `byRef["coca"].ICO.Equal(decimal.NewFromFloat(0.08))`; `assert.Equal(t, "0.08", product.ICO.String())` read from DB |
| Defaults: Required value missing everywhere | proven | `catalog_import_service_test.go:424` …RequiredValueMissingEverywhere | `requireProblem(... "/ingredients/0", ProblemMissingField)`, `Contains(problem.Message, "unit_of_measure")` |
| Defaults: Generated SKU avoids collisions | proven | `catalog_import_service_test.go:488` …GeneratedSKUAvoidsCollisions | `Equal([]{{"a","ING-ARR-INT-2"},{"b","ING-ARR-COC"}}, result.Summary.GeneratedSKUs)` |
| Opening stock: Opening count recorded | proven | acceptance `catalog_import_test.go:249-252`; unit `:735-741` | `assert.Equal(t, 25000, stock.Amount)`; `count("historic_stock", ... kind = count AND change = 25000) == 1` |
| Opening stock: Count on an existing product refused | proven | `catalog_import_service_test.go:516` …OpeningStock | `requireProblem(t, result, "/opening_stock/0", ProblemStockOnExisting)`; strict stock mock, no writes |
| Who may import: Unauthenticated caller | proven | `bootstrap/catalog_import_routes_test.go:31` | `assert.Equal(t, http.StatusUnauthorized, rec.Code)`; handler has a nil service, so passing the guard would panic |
| Who may import: Role without the permission | proven | `bootstrap/catalog_import_routes_test.go:42` | `assert.Equal(t, http.StatusForbidden, rec.Code)` for manager, cooker and agent on both paths; nil service |
| Who may import: Edge node | proven | `bootstrap/catalog_import_routes_test.go:60` | `assert.Equal(t, http.StatusNotFound, rec.Code)` on the real edge assembly |
| Size limits: Oversized body | proven | `catalog_import_handler_test.go:113` | `assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)` on both paths; strict mocks |
| Size limits: Too many products | proven | `catalog_import_service_test.go:549` …TooManyProducts | `assert.Equal(t, []string{ProblemLimitExceeded}, problemCodes(result))` |
| MCP: Customer toolset | proven | `tools_catalog_import_test.go:216` | `NotContains(names, "validate_catalog_import")`, same for apply |
| MCP: File path over HTTP | proven | `tools_catalog_import_test.go:137` | `Contains(text, "only accepted when this MCP server runs on stdio")`, `NotContains(text, "no such file")`, `assert.Empty(t, backend.requests)` |
| MCP: File path over stdio | proven | `tools_catalog_import_test.go:169` | `assert.Equal(t, response, text)`, `Len(sent.Ingredients, len(entries))` for a file of at least 500 KiB |
| Playbook: Playbook listed | proven | `tools_playbooks_test.go:113` | `Contains(list, "\"onboard-catalog\"")`, `NotEmpty(pb.Description)`, content holds the steps |
| Playbook: Stock tool descriptions | proven | `tools_catalog_import_test.go:223` | `NotContains(tool.Description, "edge mode only")`, `Contains(..., "cloud only")` for the 4 stock writes |

## Task claims
| Task | Code | Verify | Result |
|---|---|---|---|
| 1.1 | `design.md` D4 "Findings (task 1.1)" | D4 lists the findings | ran (read), passed |
| 1.2 | `permissions.go:27-28,201`; admin gets `AllPermissions()` (`role_permissions.go:60`) | permission test | ran, passed (`TestGetPermissionsForRoles_CatalogImportOnlyForAdmin`) |
| 1.3 | `domain/dto/catalog_import.go` | `go build ./...` | ran, passed |
| 1.4 | `catalog_import_plan.go:444` (`product.GenerateSKU`, from #89 on main) | SKU collision tests | ran, passed (`:488`, `:501`) |
| 1.5 | `ports/product_repository.go:12,22`, `product_ingredient_repository.go:11`; repo impls | build + mocks regenerated | ran, passed: `make generate-mocks` left the tracked mocks byte-identical. Note: no batched lookup by names and SKUs was added. The catalog is read with one `FindAll` (one query, not per entry) |
| 2.1 | `catalog_import_plan.go:228-323,453` | service tests | ran, passed |
| 2.2 | `catalog_import_plan.go:505-635` | service tests | ran, passed |
| 2.3 | `catalog_import_plan.go:324-451` | service tests | ran, passed |
| 2.4 | `catalog_import_plan.go:188,636` | service tests incl. one per cap | ran, passed (`TestCatalogImportService_Validate_NestedLimits`) |
| 2.5 | `catalog_import_plan.go:169`; `catalog_import_service.go:251` | digest tests | ran, passed |
| 3.1 | `catalog_import_service.go:107-247` | service apply tests | ran, passed |
| 4.1 | `catalog_import_handler.go:29-84` | handler tests | ran, passed |
| 4.2 | `cloud.go:302-305`, `routes_cloud.go:230-236`, golden +2 lines | golden + edge route test | ran, passed |
| 4.3 | `docs/api/catalog-import.md`, `docs/examples/curl-examples.md` | every code in doc | ran, passed (`TestCatalogImportService_APIDocNamesEveryProblemCode`) |
| 5.1 | `test/acceptance/catalogimport/` | `DB_HOST= make test-acceptance` + time | ran, passed; 1,000-product apply took 2.75 s (187 KB document) |
| 5.2 | `test/acceptance/sync/catalog_import_test.go` | acceptance | ran, passed |
| 6.1 | `mcpserver/tools_catalog_import.go:54-83`, `server.go:56` | tool tests | ran, passed |
| 6.2 | `tools_catalog_import.go:85-121`, `cmd/mcp-server/main.go:66-69` | tool tests | ran, passed |
| 6.3 | `tools_stock.go:42,53,65,76` | description test | ran, passed |
| 6.4 | `internal/playbooks/onboard-catalog.md` | playbooks tests + get_playbook | ran, passed |
| 6.5 | (unchecked) | manual Claude Desktop onboarding | not claimed; not done |
| 7.1 | `bootstrap/catalog_import_routes_test.go:31` | test | ran, passed |
| 7.2 | `bootstrap/catalog_import_routes_test.go:42` | test | ran, passed |
| 7.3 | acceptance `catalog_import_test.go:321` | test | ran, passed |
| 7.4 | `bootstrap/catalog_import_routes_test.go:60` | test | ran, passed |
| 7.5 | `tools_catalog_import_test.go:216` | test | ran, passed |
| 7.6 | (unchecked) | scenario-to-test list in PR description | not claimed; the "Scenarios → tests" table above can serve as that list |
| 8.1 | n/a | lint, build, test, acceptance | ran, passed (see Checks run) |
| 9.1 | `design.md` Security (review notes, `refuseOversizedDecimals` `catalog_import_plan.go:722`) | design updated, no open finding | ran (read), passed; fix covered by `TestCatalogImportService_HugeExponentsAreRefusedBeforeAnyWork` |

## Test integrity
- `backend/internal/platform/mcpserver/tools_product_ingredients_test.go:49`: expected tool count changed from 100 to 102. Justified, because D10 adds two tools, and `docs/mcp-server.md` was updated to match.
- `backend/internal/bootstrap/testdata/routes_cloud.golden`: 2 lines added (`POST /api/catalog/import`, `POST /api/catalog/import/validate`), none removed. Justified by D1 and task 4.2.
- `backend/internal/domain/ports/mocks/mock_ProductRepository.go`, `mock_ProductIngredientRepository.go`: methods added only. Regenerating them matched the committed files exactly. Justified by task 1.5.
- No deleted tests, no loosened assertions, no new `t.Skip`/`.only`. `mock.Anything` is used only in new tests; their `RunAndReturn` callbacks capture the arguments and assert on them.

## Checks run
- `go build ./...` (from `backend/`): passed
- `make lint` (from `backend/`): passed (0 issues; tenant-scope check ok)
- `go test -count=1 ./internal/bootstrap/... ./internal/domain/... ./internal/platform/handler/... ./internal/platform/mcpserver/... ./internal/platform/postgres/... ./internal/playbooks/...` (from `backend/`): passed
- `DB_HOST= make test-acceptance` (from `backend/`): passed (all suites, including `catalogimport` and `sync`)
- `make generate-mocks` (from `backend/`, reverted afterwards): tracked mocks unchanged. It also produced an untracked `mock_BoxOrganizationRepository.go`, a gap that is already on main and not part of this change. Removed.

## Flow from the code
```
MCP client ──validate_catalog_import──▶ tools_catalog_import.go:60
   │  catalogImportDocument (inline | file_path only if localFiles)   :85
   └─POST /api/catalog/import/validate──▶ routes_cloud.go:231 JWTAuth + RequirePermission(catalog:import)
        └▶ catalog_import_handler.go:29 decodeCatalogImport (2 MiB, unknown fields → 400/413) :64
            └▶ catalog_import_service.go:89 Validate
                 refuseOversizedDecimals            catalog_import_plan.go:722
                 productRepo.FindAll (tenant)       service.go:93
                 planCatalogImport                  plan.go:99 (limits → refs → fields → SKUs → names → aggregates → recipe/slots → stock)
                 catalogImportDigest                service.go:251
            ◀── 200 {valid, problems, summary, digest}

MCP client ──apply_catalog_import(expected_digest)──▶ tools_catalog_import.go:73 (empty digest → tool error)
   └─POST /api/catalog/import──▶ routes_cloud.go:232 (same guards) ─▶ handler.go:43
        └▶ service.go:107 Apply
             refuseOversizedDecimals → digest == expected?  (no → 409 digest_mismatch)   :109-126
             unitOfWork.Do                                                               :129
               FindAll + planCatalogImport again (problems → 422, rollback)              :130-138
               write: products.CreateMany                                                :159
                      ingredients.CreateMany (recipe)                                    :174
                      slots.ReplaceForCompositeProduct per plate (buildSlots)           :178
                      products.CreatePreparationResponsibilities                         :199
                      stock.BulkCreateOrUpdate + recordMovement(kind=count)              :216-246
        ◀── 201 {applied, products: ref→{id, sku}}
```

Differs from design.md:
- D5 says "sorted keys". The digest uses re-encoding into the typed struct (`service.go:249`) to fix field order. The effect is the same, but it is not literal key sorting.
- D11 / task 1.5 say "batched name/SKU lookup". The code loads the whole organization catalog with one `FindAll` (`service.go:93,130`) and matches in memory. That is one query, so there is no fan-out, but the lookup is not batched by the document's names.
- D4 says "re-validate inside the transaction (against the same organization snapshot)". The re-read happens at default isolation with no lock, so it is not a snapshot that guards against a concurrent apply (see "Read these yourself" 1).
- Added beyond design: problem codes `existing_ambiguous`, `missing_field`, `invalid_field`, `invalid_slot`, `duplicate_item`, all documented in `docs/api/catalog-import.md`. Also added: the "trailing data refused" check in the handler, and the `document`+`file_path` "not both" rule in the MCP tools.
- Design Risks mitigations not implemented (not tasks): gzip-compressed body support, and the "document rejected by the import is also rejected entry by entry through the existing endpoints" test.

## Out of plan
- `backend/internal/platform/postgres/repository/product_repository.go`: `Create` refactored to share `newProductModel` with `CreateMany`. It is needed for 1.5, but it touches an existing path.
- `backend/go.mod`: `github.com/google/jsonschema-go` moved from indirect to direct (used by the MCP tool schema). No task names it.
- Tasks with no code: 6.5 (manual Claude Desktop onboarding) and 7.6 (scenario list in the PR description), both unchecked.
- Open questions in design.md, both owned by the operator: the aws-infra proxy body limit (in-repo check done, aws-infra unchecked), and the role of the deployed HTTP MCP server's user (not decidable from the repo).
