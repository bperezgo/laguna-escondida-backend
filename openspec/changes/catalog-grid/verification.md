# Verification: catalog-grid

Commit `96292e4` · base `origin/main` · 2026-11-27

## Verdict
Needs work: the logic is well covered and every check is green, but a paste that changes a row's type drops plate links without the confirmation the spec requires, and the JSON loader ignores `taxes_format`, so a document in another format is silently read as percentages. Scenarios 27 proven, 4 weak, 0 missing. Tasks 13/14 checked, 0 false claims. Checks: all green.

## Read these yourself
1. `frontend/lib/catalogGrid/fromDocument.ts`: the trust boundary for files that may come from an AI. Line 128 (`str(o, "taxes_format", where)`) and line 97 accept any `taxes_format` string and throw the value away. The server requires `"percentage"`. A document with `vat: 0.19` in another format loads as 0.19 % and is re-exported as `"percentage"`. Design D7 says "same rule as the server". Two smaller silent drops: `prep` on an `INGREDIENT` entry is accepted and ignored (lines 135-146), and an `opening_stock` count that names an `existing` ref is ignored (line 264), where the server would report `stock_on_existing`.
2. `frontend/components/catalog-grid/CatalogGrid.tsx`: no test covers this component. It wires check and create (152-178), the confirmation dialogs (180-199), the draft save effect and the `saving` ref (110-114, 171, 296), paste (231-244) and file load (246-271). The modules it calls are tested; how it calls them is not.
3. `frontend/lib/catalogGrid/state.ts:248-252`: a pasted type cell that turns an Ingrediente into Producto or Plato goes through `changeType`, and `withoutRefs` (259) then removes the row from plate recipes and choices with no confirmation. The spec's requirement "Rows and product types" says this SHALL ask first.
4. `frontend/components/products/IngredientsEditor.tsx` and `frontend/components/products/OptionSlotsEditor.tsx`: the live product page was refactored onto `RecipeLinesFields` and `OptionSlotsFields`. The product page has no component tests, so the claim that it "behaves as before" (D4) rests on the unchanged `optionSlots.test.ts` and on reading the code. Check that `handleChange` / `recipeChange` (IngredientsEditor.tsx:82-109) still adds, updates and removes exactly one line per edit.
5. `frontend/lib/catalogGrid/route.ts` and `frontend/lib/catalogGrid/proxy.ts`: these forward the session token and the raw body, and delete the cookie on a 401. `risk-areas.sh` also flags `frontend/lib/permissions/constants.ts` and `edge.ts`, which add `catalog:import` and put it in `CLOUD_ONLY_PERMISSIONS`.
6. `frontend/lib/catalogGrid/draft.ts:120-129`: draft storage, keyed by organization and user. A stored value that is not valid JSON makes `loadDraft` return `available: false`, so the page says "the draft is not being saved" until the first save succeeds, about 1 s later. Small, but the message is wrong.

## Scenarios → tests
| Scenario | Rating | Test | Assertion on THEN |
|---|---|---|---|
| Who sees: Admin on the cloud | proven | `frontend/lib/catalogGrid/view.test.ts:7` Admin on the cloud | `assert.equal(gridAccess(admin), "ready"); assert.equal(showGridLink(admin), true)` |
| Who sees: Role without the permission | weak | `frontend/lib/catalogGrid/view.test.ts:13` Role without the permission | asserts `gridAccess(manager) === "forbidden"`, but nothing checks that no import endpoint is called. In the code, `CatalogGrid` is never mounted outside `"ready"` (`CatalogGridPage.tsx:35-42`) |
| Who sees: Edge box | weak | `frontend/lib/catalogGrid/view.test.ts:19` Edge box | asserts `gridAccess(edge) === "edge"`; same gap as above for "no import endpoint is called" |
| Rows: Ingredient row has no price | proven | `frontend/lib/catalogGrid/toDocument.test.ts:116` and `state.test.ts:37` | `assert.equal(key in arroz, false, key)` for price, vat, ico, taxes_format, prep (the grid row has `price: "999"`); `cellApplies("INGREDIENT", column) === false` |
| Rows: Deleting an ingredient used by plates | proven | `frontend/lib/catalogGrid/state.test.ts:66` | `platesUsing(s, ["arroz"])` deepEqual `["Bandeja paisa", "Arroz con pollo"]`; after delete, the recipe is `[rowRef("frijol")]` and the choice items are `[]` |
| Rows: Ingredient becomes a sellable | proven | `frontend/lib/catalogGrid/state.test.ts:88` | `typeChangeImpact(...)` deepEqual `{ plates: ["Arroz con pollo"], ... }`; `next.rows[1].slots[0].choices[0].items` deepEqual `[]` |
| Rows: Ingredient becomes Ambos | proven | `frontend/lib/catalogGrid/state.test.ts:98` | `typeChangeImpact(s, "club", "BOTH") === null`; `next.rows[1].recipe.length === 1` |
| Rows: Changing a plate's type | proven | `frontend/lib/catalogGrid/state.test.ts:52` | `dropsPlateDetail: true`; `next.rows[1].recipe` and `.slots` deepEqual `[]` |
| SKU: Two rows with the same name | proven | `frontend/lib/catalogGrid/state.test.ts:133` | `skus.a` deepEqual `{ value: "BAR-LIM-COC", suggested: true }`, `skus.b` deepEqual `{ value: "BAR-LIM-COC-2", ... }` |
| SKU: Typed SKU kept | proven | `frontend/lib/catalogGrid/state.test.ts:151` | after rename, `{ value: "LIMCOCO", suggested: false }`; after emptying the cell, the suggestion `BAR-LIM-NAT` comes back |
| Tax: New row defaults | proven | `frontend/lib/catalogGrid/state.test.ts:161` | `s.rows[0].ico === "8"`, `s.rows[0].vat === "0"` |
| Tax: Clear ICO and fill IVA | proven | `frontend/lib/catalogGrid/state.test.ts:170` | Producto rows deepEqual `["0", "19"]`, Ingrediente rows deepEqual `["", ""]` |
| Tax: Fill selected rows | proven | `frontend/lib/catalogGrid/state.test.ts:184` | `s.rows.map(r => r.vat)` deepEqual `["0", "19", "0", "19", "0"]` |
| Panel: Recipe from grid rows | proven | `frontend/lib/catalogGrid/check.test.ts:166` | `plateSummary(...) === "2 ingredientes"`; recipe deepEqual `[{ ingredient: "arroz", quantity: 200 }, { ingredient: "frijol", quantity: 150 }]` |
| Panel: Existing ingredient picked | proven | `frontend/lib/catalogGrid/check.test.ts:184` | `document.existing` deepEqual `[{ ref: picked.id, sku: "ING-LIM" }]`; `document.ingredients === undefined` |
| Panel: Plate offered as an ingredient | proven | `frontend/lib/catalogGrid/check.test.ts:195` | `names` deepEqual `["Yuca"]` (a second plate and an existing COMPOSITE are present and left out) |
| Panel: Slot without a default | proven | `frontend/lib/catalogGrid/check.test.ts:205` | `problem?.kind === "slot"`, `problem?.key === "s1"`. That the panel stays open depends on `PlatePanel.tsx:48-52`, which is untested |
| Paste: Paste names and prices | proven | `frontend/lib/catalogGrid/paste.test.ts:7` | 30 rows; first and last `[name, price]` checked; every row has `skus[r.id].value && skus[r.id].suggested` |
| Paste: Bad value in a paste | proven | `frontend/lib/catalogGrid/paste.test.ts:23` | `r.price === ""`, `r.marks.price === true`, and the other 7 cells deepEqual their pasted values |
| Check: Problem shown on its row | proven | `frontend/lib/catalogGrid/check.test.ts:58` | `placed.cells.r12` deepEqual `{ name: ['a product named "P12" already exists'] }` |
| Check: Edit after a passing check | proven | `frontend/lib/catalogGrid/check.test.ts:85` | `canCreate(...)` is `true` before the price edit and `false` after |
| Check: Successful create | proven | `frontend/lib/catalogGrid/check.test.ts:103` | `created` deepEqual `{ kind: "created", products: 12 }`; `cleared === 1`; `expected_digest === "abc"`; `sent` deepEqual the validate body |
| Check: Create refused by the server | weak | `frontend/lib/catalogGrid/check.test.ts:122` | asserts `kind === "refused"` and `cleared === 0` for 422, 409 and 500. Nothing checks that the returned problems come back to be shown "as after a check" (`CatalogGrid.tsx:174-175`) |
| Draft: Reload keeps work | proven | `frontend/lib/catalogGrid/draft.test.ts:27` | `loaded.state` deepEqual `before` (40 rows, including a plate with a recipe and a slot) |
| Draft: Another organization's draft not shown | proven | `frontend/lib/catalogGrid/draft.test.ts:36` | the three other org/user pairs get `{ state: null, available: true }`; storage still holds only the org-a/user-1 key |
| Draft: Storage unavailable | proven | `frontend/lib/catalogGrid/draft.test.ts:45` | `loadDraft` returns `{ state: null, available: false }`, `saveDraft` returns `false`, `clearDraft` does not throw. The page message (`CatalogGrid.tsx:317-319`) is untested |
| JSON: Agent document reviewed | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:115` | `rows.length === 120`; recipe linked to the ingredient row; 2 choices with the default set. The "no endpoint" half (`check.test.ts:151`) is tautological, because `parseDocumentText` takes no fetch |
| JSON: Oversized file | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:18` | `reason === "too_large"`; `read === false` (the file is not read). Rows stay unchanged because `CatalogGrid.tsx:255-258` returns before dispatching |
| JSON: Malformed document | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:33` | `reason === "invalid_shape"`; the message matches `/product_id/` |
| JSON: Markup in a name | weak | `frontend/lib/catalogGrid/check.test.ts:158`, `fromDocument.test.ts:108` | a source scan that `components/catalog-grid/` has no `dangerouslySetInnerHTML`/`innerHTML`, plus a check that the name survives parsing. Nothing renders the name |
| JSON: Round trip | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:141` | `comparable(loaded)` deepEqual `comparable(original)` over name, type, category, area, unit, price, vat, ico, stock, SKU, recipe and slots |

## Task claims
| Task | Code | Verify | Result |
|---|---|---|---|
| 1.1 | `frontend/lib/permissions/constants.ts:21`, `frontend/app/api/catalog/import/route.ts:3`, `.../validate/route.ts:3`, `frontend/lib/catalogGrid/route.ts:7`, `frontend/lib/catalogGrid/proxy.ts:11` | route tests: body and token forwarded, 413/403 unchanged | ran, passed (`proxy.test.ts`: 4 tests; they test `forwardCatalogImport`, not the Next handler) |
| 1.2 | `frontend/types/catalogImport.ts:75` | `pnpm lint` passes (typecheck) | ran, passed. `pnpm lint` is `eslint .` and does no type checking, so I also ran `tsc --noEmit`, which passed |
| 1.3 | `frontend/components/products/RecipeLinesFields.tsx:29`, `OptionSlotsFields.tsx:66`, `IngredientsEditor.tsx:169`, `OptionSlotsEditor.tsx:159` | existing product-page tests pass unchanged; new tests for the plain parts | ran, passed. The existing `optionSlots.test.ts` tests are unchanged (only additions). There are no product-page component tests, and the new tests cover the `lib/products` helpers, not the `*Fields` components |
| 2.1 | `frontend/lib/catalogGrid/state.ts:262` | the 11 listed unit tests | ran, passed (`state.test.ts`) |
| 2.2 | `frontend/lib/catalogGrid/toDocument.ts:63`, `:185` | expected document, percentages, path mapping, prefix fallback | ran, passed (`toDocument.test.ts`) |
| 2.3 | `frontend/lib/catalogGrid/fromDocument.ts:180`, `:287` | Oversized, Malformed, Round trip, unmatched existing, `__proto__` | ran, passed (`fromDocument.test.ts`) |
| 2.4 | `frontend/lib/catalogGrid/paste.ts:5`, `:78`; `state.ts:223` | names and prices, bad value, Colombian prices, 1,000 cap | ran, passed (`paste.test.ts`) |
| 2.5 | `frontend/lib/catalogGrid/draft.ts:120`, `:132`, `:149` | Reload, other org, storage unavailable, removal | ran, passed (`draft.test.ts`) |
| 3.1 | `frontend/app/(main)/products/importar/page.tsx:3`, `frontend/components/catalog-grid/CatalogGridPage.tsx:31`, `CatalogGrid.tsx:83`, `frontend/lib/catalogGrid/view.ts:12-34`, `ProductsPageClient.tsx:25` | component tests for Admin / Role / Edge, with no proxy call | ran, passed, but as pure-logic tests of `access.ts` (`view.test.ts`), as the tasks.md header allows. Nothing tests "no proxy call" |
| 3.2 | `frontend/components/catalog-grid/PlatePanel.tsx:37`, `state.ts:344`, `frontend/lib/catalogGrid/panel.ts:10` | component tests for the 4 panel scenarios | ran, passed as logic tests (`check.test.ts:166-224`). The panel component is untested |
| 3.3 | `frontend/components/catalog-grid/CatalogGrid.tsx:152-178`, `:246-271`; `frontend/lib/catalogGrid/check.ts:73-103` | component tests for 6 scenarios | ran, passed as logic tests (`check.test.ts`). The component wiring is untested |
| 4.1 (unchecked) | none | end to end against a local backend | not done, and it is correctly left unchecked |
| 5.1 | n/a | `pnpm lint`, `pnpm test` pass | ran, passed. Lint: 0 errors, 74 warnings. In changed files, only `IngredientsEditor.tsx:70` and `OptionSlotsEditor.tsx:66` (`set-state-in-effect`), the same load-in-effect pattern as on `origin/main` |
| 6.1 | `openspec/changes/catalog-grid/design.md` "Security" | Security section updated, no confirmed finding open | unverified: the section is updated and records the open needs-validation item (proxy body limit in aws-infra). I cannot confirm that the audit itself was run |

## Test integrity
- `frontend/lib/products/optionSlots.test.ts`: only additions (2 new tests, new imports). No existing assertion was changed or removed. Justified by D4.
- No deleted tests, no `.skip` or `.only`, no golden files or mocks changed.
- `frontend/lib/catalogGrid/check.test.ts:151`: a new test that cannot fail. `calls.length === 0` holds because the fake server is never passed to `parseDocumentText`. Not harmful, but it proves nothing.
- `frontend/lib/catalogGrid/check.test.ts:158`: the "Markup in a name" scenario is covered by a source grep, not a render. D10 justifies this (no component renderer).

## Checks run
- `pnpm -C frontend lint` (from the repo root): passed, exit 0, 0 errors, 74 warnings
- `pnpm -C frontend test` (from the repo root): passed, 529 tests, 0 failed, 0 skipped
- `npx tsc --noEmit -p .` (from `frontend/`): passed, exit 0
- `scripts/risk-areas.sh origin/main` (from the repo root): flags `auth` on `frontend/lib/permissions/constants.ts` and `frontend/lib/permissions/edge.ts`
- Backend: not changed, so no backend checks were run. Confirmed that `origin/main` already has the dependency: `backend/internal/bootstrap/routes_cloud.go:233` (`RequirePermission(permissions.CatalogImport)`) and `frontend/lib/products/sku.ts`
- Working tree clean at `96292e4` apart from this file

## Flow from the code
```
User            CatalogGrid.tsx           lib/catalogGrid            Next proxy                    Backend
 | edit cell -->  :201 dispatch edit  -->  state.ts:280 setCell
 |                :110 effect (1 s)   -->  draft.ts:132 saveDraft -> localStorage[catalog-grid:v1:org:user]
 |                :118 useMemo        -->  toDocument.ts:63 (document + pointers)
 | Revisar ---->  :152 handleCheck    -->  check.ts:82 checkCatalog -> :49 postImport
 |                                          fetch /api/catalog/import/validate --> validate/route.ts:3
 |                                                                                 route.ts:10 forwardCatalogImport
 |                                                                                 proxy.ts:19 POST + Bearer --> routes_cloud.go:233
 |                                          check.ts:62 result <------------------------------------------- 200 {valid,problems,digest}
 |                :157 setCheck(passedCheck)  check.ts:73 {digest, documentHash}
 |                :142 placeProblems  -->  check.ts:118 -> toDocument.ts:185 locateProblem
 |                :119 createEnabled  -->  check.ts:78 canCreate (hash of current doc == checked hash)
 | Crear ------>  :162 handleCreate   -->  check.ts:91 createCatalog
 |                                          :97 postImport(APPLY_URL, {...doc, expected_digest}) --> import/route.ts:3 --> routes_cloud.go:233
 |                                          :98 applied? -> onCreated -> draft.ts:149 clearDraft
 |                :168 setCreated(n); :171 saving=false; :172 replace(EMPTY_STATE)
 |                else :175 showOutcome(problems)  (draft kept)
 | Cargar JSON -> :246 handleFile     -->  fromDocument.ts:287 size check -> :269 parse -> :180 documentToState
 |                :259 replace(state)      (no endpoint call)
```

Differs from design.md:
- D7: the shape check is looser than "same rule as the server". `taxes_format` accepts any string and is discarded (`fromDocument.ts:97`, `:128`). `prep` on `INGREDIENT` entries is accepted and dropped. `opening_stock` on an `existing` ref is dropped without a message.
- D2 / spec "Rows and product types": the confirmation before a type change that removes links exists only in the UI path (`CatalogGrid.tsx:180-189`). The paste path changes type and removes links with no confirmation (`state.ts:248-252`, `:259`).
- D3: an extension, not a conflict. `existing` entries of a loaded file that matched nothing are sent as `unmatched:<key>` refs (`toDocument.ts:138-141`), so the server reports them.
- D9: an extension. An emptied grid removes the stored draft instead of saving an empty one (`draft.ts:141`).
- D10 / tasks 3.1-3.3: the "component tests" became tests of pure modules, by the decision stated in the tasks.md header. The component layer has no tests.
- D1, D4, D5, D6, D8: followed as written.

## Out of plan
- No changed file falls outside a task. `lib/catalogGrid/values.ts` and `view.ts` support 2.1/2.4/3.1, and `access.ts` and `panel.ts` support 3.1/3.2.
- Task with no code: 4.1 (end to end), unchecked.
- Open question in design.md, still open: any request body limit in front of `laguna-web` (aws-infra, outside this repo). The owner is to check it during 4.1 or on staging with a catalog of about 1.5 MiB.
