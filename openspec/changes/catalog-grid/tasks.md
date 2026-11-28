Frontend only, commands run from `frontend/`.

Tests: the frontend runs only `node --test` over pure `.ts` modules (no jsdom, no component renderer). By decision during apply, the route and component checks below are pure-logic tests of the module that owns each decision (`lib/catalogGrid/*`, `lib/products/*`); the components stay thin over them. Start only after `catalog-import` and `sku-suggestion` are merged (the endpoints, the `catalog:import` permission and `lib/products/sku.ts` must exist).

## 1. Groundwork

- [x] 1.1 Add the `catalog:import` permission constant and the two proxy routes `app/api/catalog/import/validate/route.ts` and `app/api/catalog/import/route.ts` (D1). Verify: route tests that the body and token are forwarded and a 413 or 403 from the backend comes back unchanged.
- [x] 1.2 Add the catalog import document types in `types/catalogImport.ts`, matching `backend/docs/api/catalog-import.md`. Verify: `pnpm lint` passes (typecheck).
- [x] 1.3 Split `IngredientsEditor` and `OptionSlotsEditor` into plain `value`/`onChange` parts with candidates as a prop, keeping the API-bound components (D4). Verify: the existing product-page tests pass unchanged, and new tests cover the plain parts.

## 2. Grid state and mapping

- [x] 2.1 Implement the reducer in `lib/catalogGrid/state.ts` (D2): add, duplicate, delete, edit, change type with recipe removal, SKU suggestion with `skuTouched`, tax defaults and the three column actions limited to sold rows. Verify: unit tests for "Ingredient row has no price", "Changing a plate's type", "Deleting an ingredient used by plates", "Ingredient becomes a sellable", "Ingredient becomes Ambos", a plate linked to an ingredient row placed after it, "Two rows with the same name", "Typed SKU kept", "New row defaults", "Clear ICO and fill IVA" and "Fill selected rows".
- [x] 2.2 Implement `lib/catalogGrid/toDocument.ts` with the pointer map (D3). Verify: unit tests that a grid with every type, a recipe, a slot, an existing ingredient and opening stock produces the expected document, taxes as percentages (as `catalog-import.md` defines; see D3), and that a problem path maps to the right row, cell and panel, including the prefix fallback.
- [x] 2.3 Implement `lib/catalogGrid/fromDocument.ts` with the size and shape checks (D7). Verify: unit tests for "Oversized file", "Malformed document", "Round trip", unmatched `existing`, and that a `__proto__` key in the file does not change `Object.prototype`.
- [x] 2.4 Implement paste parsing in `lib/catalogGrid/paste.ts` (D8). Verify: unit tests for "Paste names and prices", "Bad value in a paste", Colombian price formats and the 1,000-row cap.
- [x] 2.5 Implement `lib/catalogGrid/draft.ts` (D9). Verify: unit tests for "Reload keeps work", "Another organization's draft not shown", "Storage unavailable" and the removal after create.

## 3. UI

- [x] 3.1 Build the page and grid components (`app/(main)/products/importar/`, `components/catalog-grid/`): table with keyboard moves, selection, type filter, name search, paging by 100, suggested SKUs shown differently, tax column menus, and a link from the products page. Verify: component tests for "Admin on the cloud", "Role without the permission" and "Edge box", with no proxy call in the last two.
- [x] 3.2 Build the plate panel on the plain editors, with candidates limited to `INGREDIENT` and `BOTH` rows and existing products. Verify: component tests for "Recipe from grid rows", "Existing ingredient picked", "Plate offered as an ingredient" and "Slot without a default".
- [x] 3.3 Wire Revisar and Crear catálogo (D6), problems on rows and above the grid, the created summary, draft clearing, and JSON export and load with confirmation. Verify: component tests for "Problem shown on its row", "Edit after a passing check", "Successful create", "Create refused by the server", "Agent document reviewed" and "Markup in a name".

## 4. End to end

- [ ] 4.1 Against a local cloud backend, paste a 50-product menu, add two plates with recipes and slots, check, fix a reported conflict and create. Verify: the products appear on the products page with the suggested SKUs, and an order with a slot choice decrements the right ingredients.

## 5. Finish

- [x] 5.1 Run `pnpm lint` and `pnpm test` from `frontend/`. Verify: both pass, or remaining lint findings are only in files this change did not touch.

## 6. Security review

- [x] 6.1 Run the security-audit skill in guidance mode (focused review, not the full audit) over this change's diff against design.md "Security". Fix confirmed findings, and record in design.md how the needs-validation item (Next.js proxy body limit) was checked or who will check it. Verify: design.md Security is updated and no confirmed finding is left open.
