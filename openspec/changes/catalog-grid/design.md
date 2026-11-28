## Context

See proposal.md for the motivation and `specs/catalog-grid/spec.md` for the behavior. Constraints in the current code:

- The import endpoints and the document format come from `catalog-import`: `POST /api/catalog/import/validate` and `POST /api/catalog/import`, cloud only, `catalog:import` (admin) only, problems carrying a JSON-pointer `path`, `digest` from validate required as `expected_digest` on apply, 2 MiB body cap, 1,000 products cap.
- The SKU rule and its TypeScript generator (`lib/products/sku.ts`) come from `sku-suggestion`.
- `usePermissions()` gives `hasPermission` and `isEdge`. Plan gating is done by `pageVisible` in `lib/plans.ts`. The browser calls the backend through Next.js proxy routes under `app/api/`.
- `IngredientsEditor.tsx` and `OptionSlotsEditor.tsx` take a saved `product` and load and save through `recipeApi` and `productsApi`. Slot rules live in `lib/products/optionSlots.ts`, which is pure. `OptionSlotsEditor` offers `COMPOSITE` products as bundle candidates, while the import allows only `INGREDIENT` and `BOTH`.
- Taxes are shown as percentages. The product API stores them as fractions, but the import document takes percentages (`ico: 8`, `taxes_format: "percentage"`; the backend divides by 100).
- No data-grid library is installed. `BulkStockTable.tsx` is a plain table with inputs.

## Goals / Non-Goals

**Goals:**
- The grid only produces an import document. All business rules stay in the import validation.
- A document exported from the grid and a document produced by the agent load the same way.

**Non-Goals:**
- Running the import's validation rules in the browser (only the slot rules the panel already has).
- A server-side draft, multi-user editing, `.xlsx` files.

## Decisions

### D1. Page, access and proxy routes

The page is `app/(main)/products/importar/page.tsx`, linked from the products page. It renders the grid only when `!isEdge && hasPermission("catalog:import")`, and is visible on every plan, because onboarding comes before plan features matter. Two proxy routes, `app/api/catalog/import/validate/route.ts` and `app/api/catalog/import/route.ts`, forward the body and the session token like the other proxy routes, and pass the 2 MiB and 413 behavior through unchanged.

### D2. State: rows with stable ids that become refs

The grid state is a list of rows, each with a stable `id` (random, never shown), its cells, `skuTouched`, and, for plates, `recipe` and `slots` whose ingredient references are either `{ row: id }` or `{ existing: productId }`. A reducer in `lib/catalogGrid/state.ts` holds every action (edit a cell, add, duplicate, delete, change type, fill a column, paste, load a document). It is pure and unit-tested.

*Alternative:* keep the import document itself as the state. Rejected because the grid needs per-row UI state (touched SKU, selection, marked cells) and one flat row order across types.

### D3. Mapping rows to a document and problems back to rows

`lib/catalogGrid/toDocument.ts` builds the document. Rows are split by type into `ingredients` (`INGREDIENT`, `BOTH`), `sellables` and `plates`, `ref = row.id`. Linked existing products go to `existing` by SKU. Opening stock goes to `opening_stock`. Every value is sent explicitly, so no `defaults` section is used. Taxes are sent as the percentages the grid shows (19 means 19 %), as `catalog-import.md` defines; converting them to fractions would send 0.19 %. An `INGREDIENT` row also sends no preparation area, because the import refuses `prep` on it, so the grid disables that cell too. The function also returns a map from each JSON pointer it wrote (`/plates/3`, `/plates/3/slots/0/choices/1`) to a row id and cell or panel location, which is used to place each returned problem. A pointer not in the map is matched to its longest mapped prefix, and a problem with no match is listed above the grid.

`fromDocument.ts` does the reverse for loading a file (D7).

### D4. Editors: split display from saving

`IngredientsEditor` and `OptionSlotsEditor` are each split into a part that edits plain data through `value` and `onChange` and takes the list of candidate ingredients as a prop, and the existing component that loads and saves through the API around it. The product page behaves as before. The plate panel uses the plain parts with candidates limited to `INGREDIENT` and `BOTH` (grid rows plus existing products), and runs `lib/products/optionSlots.ts` before closing.

*Alternative:* write new editors for the grid. Rejected because the slot rules and layout would exist twice.

### D5. Grid rendering without a new dependency

Plain table with inputs, as `BulkStockTable` does, plus arrow, Enter and Tab moves between cells and row checkboxes for selection. Rows are rendered in pages of 100 with the filter applied, so a 1,000-row draft never renders 10,000 inputs at once.

*Alternative:* a data-grid library. Rejected for the first version: a new dependency for something a table and a reducer cover. Revisit if paging feels wrong in use.

### D6. Check, digest and the "unchanged" rule

`Revisar` stores `{ digest, documentHash }`, where `documentHash` is a hash of the JSON the grid sent. `Crear catálogo` is enabled while the hash of the current document equals `documentHash`, and sends that same document with `expected_digest = digest`. The server still compares the digest itself, so this check only avoids a call that would be refused.

### D7. Loading a JSON document

The file is read in the browser with `File.text()` after checking `file.size <= 2 MiB`, parsed with `JSON.parse`, and checked against a hand-written shape check that rejects unknown sections and fields (same rule as the server). Ids are created fresh for each entry, refs are mapped to them, and `existing` entries are resolved against the organization's product list loaded by the page. An `existing` entry that matches nothing is kept as a marked link, so the user sees it before checking. No import endpoint is called on load. Parsed values are assigned field by field into new objects, never merged into existing ones.

### D8. Paste

A `paste` handler on the grid reads `text/plain`, splits it on lines and tabs (honoring spreadsheet quoting), and dispatches one `paste` action. Cells fill in the grid's column order, which puts price right after the name (name, price, type, category, area, unit, ICO, IVA, SKU, opening stock) so a pasted "name<TAB>price" menu lands in the right cells. An empty type or unit cell keeps the row's value. Type and unit cells accept the Spanish labels and the API values (`Ingrediente`/`INGREDIENT`, `g`, `kg`, ...), case-insensitively. Prices accept `32000`, `32.000` and `$32.000` (Colombian thousands separator).

### D10. Tests without a component renderer

The frontend runs only `node --test` over `.ts` modules. Every decision the components make lives in a pure module (`lib/catalogGrid/access.ts`, `check.ts`, `panel.ts`, `view.ts`, `proxy.ts`, plus the state and mapping modules), and the spec scenarios are tested there. The components only wire those modules to inputs. A source test checks that `components/catalog-grid/` never renders raw HTML.

### D9. Browser draft

The draft is saved to `localStorage` under `catalog-grid:v1:<organizationId>:<userId>`, 1 s after the last change, with every read and write in `try/catch`. The stored value carries a `version`. A value with an unknown version is ignored, not migrated. The page reads only its own key. The draft is removed after a successful create and when the user discards it.

## Security

| Boundary | Lower-trust principal | Input/action it controls | Control and where enforced | Protected resource | If the control fails |
|---|---|---|---|---|---|
| Page access | Signed-in user without `catalog:import` (manager, cook, ...) | Opening `/products/importar`, calling the proxy routes | Backend `RequirePermission(catalog:import)` on both import routes (`catalog-import`). The page check in D1 is only UI. | The organization's catalog | Without the backend check, any role could create products, so the backend check must not depend on the page |
| Edge UI | User of an edge box | Calling the import from the edge UI | Routes exist only on cloud (`catalog-import`). The page hides itself when `isEdge` (D1). | Edge/cloud single-writer catalog | A catalog written on the edge diverges from the cloud |
| Loaded JSON file | Author of the file (the user, or an AI client that may have read untrusted menu content) | Size, shape and text of the document | Size check before reading and shape check rejecting unknown fields (D7, `lib/catalogGrid/fromDocument.ts`). Text rendered through React interpolation only, no raw-HTML rendering. Field-by-field copy, no deep merge. The server validates again on check and create. | The user's browser session, the catalog | Oversized file freezes the tab. Markup in a name runs script in the operations UI session (stored XSS on the user's own session). A polluted prototype changes grid behavior |
| Browser draft | Another account or organization later using the same browser | Reading a draft left in `localStorage` | Key bound to organization and user, the page reads only its own key, draft removed after create or discard (D9, `lib/catalogGrid/draft.ts`) | The organization's unpublished menu and prices | Organization B sees, or sends as its own, organization A's draft |

Attack classes considered (CLIENT-SIDE, DATA-ISOLATION-AND-LIFECYCLE, RESOURCE-EXHAUSTION-AND-AVAILABILITY, AI-AND-LLM): DOM-based XSS from loaded or pasted names, prototype pollution from parsed JSON, browser-storage disclosure across accounts and tenants on a shared browser, oversized input freezing the client, and model-produced documents treated as data, not instructions (the grid only displays them, and creation needs the user's click after a server check). Restricted actions remain enforced on the backend. Hiding the page is not a control.

Review (security-audit, guidance mode, 2026-11-27) over the frontend diff. No confirmed finding is open:
- **Page access.** The page and the products-page link use `gridAccess` (`lib/catalogGrid/access.ts`), and `catalog:import` is in `CLOUD_ONLY_PERMISSIONS`, so on a box nobody holds it. The backend's `RequirePermission` stays the control.
- **Proxy routes.** `forwardCatalogImport` forwards the raw body and the cookie token, and returns the backend's status and body as text, so a 403 or a non-JSON 413 comes back unchanged. The `access_token` cookie is `SameSite=Strict` (`lib/auth/sessionResponse.ts`), so a cross-site POST carries no session.
- **Loaded file.** The size is checked before `File.text()`. `fromDocument.ts` reads only own keys, refuses any key outside each object's allowed list (a `__proto__` key is refused, with a test), and builds new rows field by field. Names and server messages are rendered through React text interpolation only, with a test that keeps raw-HTML APIs out of `components/catalog-grid/`.
- **Draft.** The key binds organization and user, and the page reads only its own key. A stored value is rebuilt field by field. Fixed during review: revived cell marks are now limited to real column names, where the check used to accept inherited keys such as `constructor`. No result crossed a boundary. A draft stays readable in the browser's own storage to anyone with that browser's developer tools until it is created or discarded, which is the accepted residual of D9.

Needs validation:
- **Proxy body limit.** Checked in source: the app sets no body limit (`next.config.js`), App Router route handlers read `request.text()` without a cap, and the Next.js proxy limit (`proxyClientMaxBodySize`) does not apply because `proxy.ts` excludes `/api`. Still open: any limit in front of `laguna-web` (load balancer or proxy in aws-infra), which is not in this repo. Owner: the repo owner, during task 4.1 or on staging, by checking and then creating a catalog of about 1.5 MiB through the deployed UI.

## Risks / Trade-offs

- [Paging 100 rows makes scanning a long menu slower] → Filter by type plus a name search. Revisit the rendering (D5) if users complain.
- [The draft lives in one browser] → Stated in the page. Export to JSON is the way to move it.
- [Splitting the editors could change the product page] → The product page keeps its current component tests, run before and after the split.
- [Problem paths not in the map after the server changes its paths] → Longest-prefix fallback and the list above the grid keep every problem visible.
- [`OptionSlotsEditor` offers composites today and the import refuses them] → Candidates are a prop, so the grid passes only `INGREDIENT` and `BOTH`. The product page keeps its current list.

## Migration Plan

Frontend only. Deploy after `catalog-import` and `sku-suggestion`. Before those, the page would get 404 from the proxy routes. Rollback is reverting the PR. Drafts left in browsers are ignored.
