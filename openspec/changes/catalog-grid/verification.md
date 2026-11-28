# Verification: catalog-grid

Commit `825b373` · base `origin/main` · 2026-11-27

## Verdict
Ready for human review: commit 825b373 closes the last finding from the previous run. A paste that changes a Plato row with a recipe or option slots to another type now asks before it removes them. I reproduced the old bug and confirmed the fix with a scratch script. No new defect was found. Scenarios 28 proven, 3 weak, 0 missing. Tasks 13/14 checked, 0 false claims. Checks: all green.

Re-check of the previous finding:
- Paste that turns a plate into another type: **fixed**. `pasteImpact` (`state.ts:168-182`) now returns `{ plates, platesLosingDetail }` or `null`. It adds a COMPOSITE row to `platesLosingDetail` when its type changes and it has recipe lines or slots, which is the same rule `typeChangeImpact` uses for a type changed by hand (`state.ts:194`). `CatalogGrid.tsx:242-250` asks with `window.confirm` whenever the result is not `null`, and returns without dispatching if the user cancels. The new test is `paste.test.ts:122`. The scratch probe also covered cases the test does not: a plate that has only slots and is pasted to Ambos (`platesLosingDetail: ["Solo opciones"]`), a plate pasted to Ingrediente (reported), "Plato" pasted onto a plate (`null`), and an unknown pasted type (`null`).
- Cosmetic: if one paste changes both an ingredient and a plate that uses it, the plate is named twice in the dialog (`{"plates":["Bandeja"],"platesLosingDetail":["Bandeja"]}`). This does not change what is removed.

## Read these yourself
1. `frontend/lib/catalogGrid/state.ts:160-182` and `frontend/components/catalog-grid/CatalogGrid.tsx:232-255`: this is the fix in this commit. Check that the Spanish confirmation text reads well, and decide whether a plate named twice (see above) is acceptable. The test covers a plate with a recipe. A plate with only slots is covered only by the `||` in line 176 and my probe.
2. `frontend/components/catalog-grid/CatalogGrid.tsx`: the component still has no tests. These parts are untested: the check and create wiring (153-177), the five `window.confirm` dialogs (187, 195, 249, 262, 285), the 1 s draft save effect (111-114), and the file load (257-273).
3. `frontend/lib/catalogGrid/fromDocument.ts`: this file is the trust boundary for files that may come from an AI. It has not changed since the last run. The refusals at 77-82, 136-142 and 278-283 are tested. Check that their messages, which are in Spanish and name the JSON path, make sense to a user.
4. `frontend/components/products/IngredientsEditor.tsx` and `OptionSlotsEditor.tsx`: the live product page was refactored onto `RecipeLinesFields` and `OptionSlotsFields` (D4). The product page still has no component tests.
5. `frontend/lib/catalogGrid/route.ts` and `proxy.ts`, plus `frontend/lib/permissions/constants.ts` and `edge.ts` (flagged `auth` by `risk-areas.sh`): the proxy forwards the session token and the raw body, and `catalog:import` is in `CLOUD_ONLY_PERMISSIONS`. Not changed since the last run.

## Scenarios → tests
| Scenario | Rating | Test | Assertion on THEN |
|---|---|---|---|
| Who sees: Admin on the cloud | proven | `frontend/lib/catalogGrid/view.test.ts:7` Admin on the cloud | `assert.equal(gridAccess(admin), "ready"); assert.equal(showGridLink(admin), true)` |
| Who sees: Role without the permission | weak | `frontend/lib/catalogGrid/view.test.ts:13` | asserts `gridAccess(manager) === "forbidden"`. No test checks that no import endpoint is called. In the code, `CatalogGrid` mounts only when access is `"ready"` (`CatalogGridPage.tsx:35-42`) |
| Who sees: Edge box | weak | `frontend/lib/catalogGrid/view.test.ts:19` | asserts `gridAccess(edge) === "edge"`. Same gap: no test checks that no import endpoint is called |
| Rows: Ingredient row has no price | proven | `frontend/lib/catalogGrid/toDocument.test.ts:116`, `state.test.ts:37` | `assert.equal(key in arroz, false, key)` for price, vat, ico, taxes_format and prep; `cellApplies("INGREDIENT", column) === false` |
| Rows: Deleting an ingredient used by plates | proven | `frontend/lib/catalogGrid/state.test.ts:66` | `platesUsing(s, ["arroz"])` deepEqual `["Bandeja paisa", "Arroz con pollo"]`; after the delete, the recipe is `[rowRef("frijol")]` and the choice items are `[]` |
| Rows: Ingredient becomes a sellable | proven | `frontend/lib/catalogGrid/state.test.ts:88`, `paste.test.ts:104` | `typeChangeImpact(...)` deepEqual `{ plates: ["Arroz con pollo"], ... }`; paste path: `pasteImpact(start, toSellable)` deepEqual `{ plates: ["Bandeja"], platesLosingDetail: [] }` and the state is unchanged |
| Rows: Ingredient becomes Ambos | proven | `frontend/lib/catalogGrid/state.test.ts:98`, `paste.test.ts:104` | `typeChangeImpact(s, "club", "BOTH") === null`; `recipe.length === 1`; `assert.equal(pasteImpact(start, toBoth), null)` |
| Rows: Changing a plate's type | proven | `frontend/lib/catalogGrid/state.test.ts:52`, `paste.test.ts:122` | typed path: `dropsPlateDetail: true`, and `recipe` and `slots` deepEqual `[]` after the change. Paste path: `assert.deepEqual(pasteImpact(start, action), { plates: [], platesLosingDetail: ["Bandeja"] })`, where the empty plate "Plato vacío" in the same paste is not named, and `start.rows[1].recipe.length === 1` (the check changes nothing). That the dialog is shown is component wiring (`CatalogGrid.tsx:243-250`) and untested |
| SKU: Two rows with the same name | proven | `frontend/lib/catalogGrid/state.test.ts:133` | `{ value: "BAR-LIM-COC", suggested: true }` and `{ value: "BAR-LIM-COC-2", ... }` |
| SKU: Typed SKU kept | proven | `frontend/lib/catalogGrid/state.test.ts:151` | `{ value: "LIMCOCO", suggested: false }` after the rename |
| Tax: New row defaults | proven | `frontend/lib/catalogGrid/state.test.ts:161` | `ico === "8"`, `vat === "0"` |
| Tax: Clear ICO and fill IVA | proven | `frontend/lib/catalogGrid/state.test.ts:170` | Producto rows deepEqual `["0", "19"]`, Ingrediente rows deepEqual `["", ""]` |
| Tax: Fill selected rows | proven | `frontend/lib/catalogGrid/state.test.ts:184` | `vat` per row deepEqual `["0", "19", "0", "19", "0"]` |
| Panel: Recipe from grid rows | proven | `frontend/lib/catalogGrid/check.test.ts:174` | `plateSummary(...) === "2 ingredientes"`; recipe deepEqual `[{ ingredient: "arroz", quantity: 200 }, { ingredient: "frijol", quantity: 150 }]` |
| Panel: Existing ingredient picked | proven | `frontend/lib/catalogGrid/check.test.ts:192` | `document.existing` deepEqual `[{ ref: picked.id, sku: "ING-LIM" }]`; `document.ingredients === undefined` |
| Panel: Plate offered as an ingredient | proven | `frontend/lib/catalogGrid/check.test.ts:203` | `names` deepEqual `["Yuca"]` |
| Panel: Slot without a default | proven | `frontend/lib/catalogGrid/check.test.ts:213` | `problem?.kind === "slot"`, `problem?.key === "s1"`. That the panel stays open depends on `PlatePanel.tsx`, which is untested |
| Paste: Paste names and prices | proven | `frontend/lib/catalogGrid/paste.test.ts:7` | 30 rows; the first and last `[name, price]` are checked; every row has a suggested SKU |
| Paste: Bad value in a paste | proven | `frontend/lib/catalogGrid/paste.test.ts:23` | `price === ""`, `marks.price === true`, and the other cells deepEqual their pasted values |
| Check: Problem shown on its row | proven | `frontend/lib/catalogGrid/check.test.ts:58` | `placed.cells.r12` deepEqual `{ name: ['a product named "P12" already exists'] }` |
| Check: Edit after a passing check | proven | `frontend/lib/catalogGrid/check.test.ts:85` | `canCreate` is `true` before the price edit and `false` after it |
| Check: Successful create | proven | `frontend/lib/catalogGrid/check.test.ts:103` | `created` deepEqual `{ kind: "created", products: 12 }`; `cleared === 1`; `expected_digest === "abc"` |
| Check: Create refused by the server | proven | `frontend/lib/catalogGrid/check.test.ts:122` | `cleared === 0`; for 500, `outcome` deepEqual `{ kind: "error", status: 500, message: "boom" }`; for 422 and 409, `returned` deepEqual the server's `problems` |
| Draft: Reload keeps work | proven | `frontend/lib/catalogGrid/draft.test.ts:27` | `loaded.state` deepEqual `before` (40 rows) |
| Draft: Another organization's draft not shown | proven | `frontend/lib/catalogGrid/draft.test.ts:36` | the other org/user pairs get `{ state: null, available: true }`; storage holds only the org-a/user-1 key |
| Draft: Storage unavailable | proven | `frontend/lib/catalogGrid/draft.test.ts:45` | `{ state: null, available: false }`, `saveDraft` returns `false`, `clearDraft` does not throw. The message on the page is untested |
| JSON: Agent document reviewed | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:128`, `check.test.ts:159` | `rows.length === 120` with recipes and slots linked; `fromDocument.ts` contains no `fetch`, `XMLHttpRequest` or import of `./check` (a scan of that one file) |
| JSON: Oversized file | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:18` | `reason === "too_large"`; `read === false` |
| JSON: Malformed document | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:33` | `reason === "invalid_shape"`; the message matches `/product_id/` |
| JSON: Markup in a name | weak | `frontend/lib/catalogGrid/check.test.ts:166`, `fromDocument.test.ts:121` | a source scan finds no raw-HTML APIs in `components/catalog-grid/`, and the name survives parsing. Nothing renders the name (no renderer, D10) |
| JSON: Round trip | proven | `frontend/lib/catalogGrid/fromDocument.test.ts:154` | `comparable(loaded)` deepEqual `comparable(original)` |

## Task claims
| Task | Code | Verify | Result |
|---|---|---|---|
| 1.1 | `frontend/lib/permissions/constants.ts:21`, `frontend/app/api/catalog/import/route.ts:3`, `.../validate/route.ts:3`, `frontend/lib/catalogGrid/route.ts:7`, `proxy.ts:11` | route tests: body and token forwarded, 413 and 403 unchanged | ran, passed (`proxy.test.ts`, which tests `forwardCatalogImport`, not the Next handler) |
| 1.2 | `frontend/types/catalogImport.ts` | `pnpm lint` passes (typecheck) | ran, passed. `pnpm lint` does no type checking, so I also ran `tsc --noEmit`, which passed |
| 1.3 | `frontend/components/products/RecipeLinesFields.tsx:29`, `OptionSlotsFields.tsx:66`, `IngredientsEditor.tsx`, `OptionSlotsEditor.tsx` | existing product-page tests pass unchanged; new tests for the plain parts | ran, passed. `optionSlots.test.ts` has only additions. The new tests cover the `lib/products` helpers, not the `*Fields` components |
| 2.1 | `frontend/lib/catalogGrid/state.ts:287` (reducer), `:168` (`pasteImpact`), `:190` (`typeChangeImpact`) | the 11 listed unit tests | ran, passed (`state.test.ts`, plus `paste.test.ts:104` and `:122` for the paste path) |
| 2.2 | `frontend/lib/catalogGrid/toDocument.ts:63`, `:185` | expected document, percentages, path mapping, prefix fallback | ran, passed (`toDocument.test.ts`) |
| 2.3 | `frontend/lib/catalogGrid/fromDocument.ts:194`, `:289` | Oversized, Malformed, Round trip, unmatched existing, `__proto__` | ran, passed (`fromDocument.test.ts`) |
| 2.4 | `frontend/lib/catalogGrid/paste.ts`, `state.ts:248` | names and prices, bad value, Colombian prices, 1,000 cap | ran, passed (`paste.test.ts`) |
| 2.5 | `frontend/lib/catalogGrid/draft.ts:120`, `:137`, `:154` | Reload, other org, storage unavailable, removal | ran, passed (`draft.test.ts`) |
| 3.1 | `frontend/app/(main)/products/importar/page.tsx`, `components/catalog-grid/CatalogGridPage.tsx`, `CatalogGrid.tsx`, `lib/catalogGrid/access.ts`, `view.ts` | component tests for Admin, Role and Edge, with no proxy call in the last two | ran, passed as pure-logic tests (`view.test.ts`), as the tasks.md header allows. No test checks that the proxy is not called |
| 3.2 | `frontend/components/catalog-grid/PlatePanel.tsx`, `lib/catalogGrid/panel.ts` | component tests for the 4 panel scenarios | ran, passed as logic tests (`check.test.ts:174-229`). The panel component is untested |
| 3.3 | `frontend/components/catalog-grid/CatalogGrid.tsx:153-177`, `:257-273`; `lib/catalogGrid/check.ts:73-118` | component tests for 6 scenarios | ran, passed as logic tests. The component wiring is untested |
| 4.1 (unchecked) | none | end to end against a local backend | not done, and correctly left unchecked |
| 5.1 | n/a | `pnpm lint` and `pnpm test` pass | ran, passed. Lint: 0 errors, 74 warnings. The only warnings in changed files are in `IngredientsEditor.tsx` and `OptionSlotsEditor.tsx` (`set-state-in-effect`), the same pattern as on `origin/main` |
| 6.1 | `openspec/changes/catalog-grid/design.md` "Security" | Security section updated, no confirmed finding open | unverified: the section is updated and records the open needs-validation item. I cannot confirm that the audit itself was run |

## Test integrity
- `frontend/lib/catalogGrid/paste.test.ts:113`, `:117`, `:119` (changed in 825b373): the expected values changed from `["Bandeja"]` to `{ plates: ["Bandeja"], platesLosingDetail: [] }` and from `[]` to `null`. This follows the new return type of `pasteImpact`. The meaning is the same and nothing was loosened. Justified by the fix for spec "Changing a plate's type".
- `frontend/lib/products/optionSlots.test.ts`: the only test file that exists on the base. It has only additions. Justified by D4.
- No deleted tests or test files, no new `.skip` or `.only`, no golden files or mocks.
- `frontend/lib/catalogGrid/check.test.ts:166`: "Markup in a name" is covered by a source grep, not a render. D10 justifies this.

## Checks run
- `pnpm -C frontend lint` (from the worktree root): passed, exit 0, 0 errors, 74 warnings
- `pnpm -C frontend test` (from the worktree root): passed, 532 tests, 0 failed, 0 skipped (531 at the previous run, plus the new `paste.test.ts:122`)
- `npx tsc --noEmit -p .` (from `frontend/`): passed, exit 0
- `scripts/risk-areas.sh origin/main` (from the worktree root): flags `auth` on `frontend/lib/permissions/constants.ts` and `edge.ts`
- Scratch probe (in the scratchpad, not committed) against `state.ts` and `paste.ts`: a plate with only slots pasted to Ambos gives `{"plates":[],"platesLosingDetail":["Solo opciones"]}`; a plate pasted to Ingrediente is reported; "Plato" pasted onto a plate gives `null`; an unknown type gives `null`; an ingredient and its plate both pasted to Producto give `{"plates":["Bandeja"],"platesLosingDetail":["Bandeja"]}`
- Backend not changed, so no backend checks were run
- The working tree was clean at `825b373` apart from this file

## Flow from the code
```
User            CatalogGrid.tsx           lib/catalogGrid            Next proxy                    Backend
 | edit cell -->  dispatch edit       -->  state.ts:305 setCell
 |                :111 effect (1 s)   -->  draft.ts:137 saveDraft -> localStorage[catalog-grid:v1:org:user]
 |                useMemo             -->  toDocument.ts:63 (document + pointers)
 | Revisar ---->  :153 handleCheck    -->  check.ts:82 checkCatalog -> :49 postImport
 |                                          fetch /api/catalog/import/validate --> validate/route.ts:3
 |                                                                                 route.ts:7 -> proxy.ts:11 POST + Bearer --> RequirePermission(catalog:import)
 |                :158 setCheck(passedCheck)  check.ts:73 {digest, documentHash}
 |                :143 placeProblems  -->  check.ts:118 -> toDocument.ts:185 locateProblem
 |                :120 createEnabled  -->  check.ts:78 canCreate (current doc hash == checked hash)
 | Crear ------>  :163 handleCreate   -->  check.ts:91 createCatalog -> postImport(apply, {...doc, expected_digest})
 |                                          applied -> onCreated -> draft.ts:154 clearDraft
 |                :169 setCreated; :170 setCheck(null)
 |                else :175 setCheck(null); problems shown (draft kept)
 | change type -> :182 typeChangeImpact (state.ts:190) -> :187 confirm -> dispatch changeType (state.ts:228)
 | paste ------>  :232 handlePaste    -->  paste.ts pasteCells; :242 pasteImpact (state.ts:168, runs the reducer on a copy)
 |                :243-249 confirm if ingredients leave plates OR plates lose recipe/slots
 |                :251 dispatch paste (state.ts:248 applyPaste -> :228 changeType per type cell)
 | Cargar JSON -> :257 handleFile     -->  :262 confirm replace -> fromDocument.ts:307 size check -> :289 parse -> :194 documentToState
 |                :270 dispatch replace   (no endpoint call)
```

Differs from design.md:
- D3: an extension. `existing` entries from a loaded file that matched nothing are sent as `unmatched:<key>` refs, so the server reports them.
- D9: an extension. An emptied grid removes the stored draft instead of saving an empty one.
- D10 / tasks 3.1-3.3: the component tests are tests of pure modules, by the decision stated in the tasks.md header.
- D2 / spec "Rows and product types": now followed on both paths. A typed or pasted type change that removes plate detail or recipe links asks first.
- D1, D4, D5, D6, D7, D8: followed as written.

## Out of plan
- Every changed file is covered by a task.
- Task with no code: 4.1 (end to end), unchecked.
- Open question in design.md, still open: any request body limit in front of `laguna-web` (aws-infra). To be checked during 4.1 or on staging with a catalog of about 1.5 MiB.
