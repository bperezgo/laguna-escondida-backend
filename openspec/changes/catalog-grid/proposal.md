## Why

`catalog-import` makes a whole catalog importable as one checked document, but the only way to produce that document is an AI client over MCP. Many owners will want to type or paste their own menu, and anyone who gets a document from the agent needs a place to review and fix it before creating anything. A spreadsheet-like grid in the operations UI covers both cases with the same import endpoints.

## What Changes

- New page **Catálogo inicial** (`/products/importar`) in the operations UI, cloud only, for users who hold `catalog:import`.
- One grid row per product, with a **Tipo** column (Ingrediente, Producto, Plato, Ambos) that decides which cells apply. Columns: name, type, category, preparation area, unit, price, ICO, IVA, SKU, opening stock, and a detail cell for plates.
- SKU suggested on every row with the `product-sku` rule, avoiding the organization's SKUs and the other rows. A SKU the user typed is kept.
- New rows start with ICO 8 % and IVA 0 %. Each tax column has a menu to fill every row, fill the selected rows, or clear the column. Rows that are not sold have no tax or price cells.
- A **plate panel** per plate row to edit its recipe and option slots, picking ingredients from the grid's rows or from the organization's existing ingredients.
- **Paste from a spreadsheet** into the grid.
- **Revisar** runs the import validation and shows each problem on its row or panel. **Crear catálogo** runs the import, only after a check that passed on the unchanged grid.
- The draft is saved in the browser automatically, scoped to organization and user, and cleared after the catalog is created.
- **Export and import of the draft as a JSON import document**, so a document produced by the agent can be reviewed in the grid before it is applied.

Out of scope: drafts stored on the server or shared between users; editing existing products from the grid; reading `.xlsx` or PDF files; an in-app agent.

## Capabilities

### New Capabilities

- `catalog-grid`: the catalog grid page, its rows, SKU suggestion, tax column actions, plate panel, paste, check and create through the import endpoints, browser draft, and JSON export and import.

### Modified Capabilities

None. The grid is a client of `catalog-import` (not yet archived) and uses `product-sku` from `sku-suggestion`, without changing their requirements.

## Impact

- Frontend: new page under `app/(main)/products/importar/`, new components under `components/catalog-grid/`, new state and mapping code under `lib/catalogGrid/`, two new proxy routes under `app/api/catalog/import/`, a link from the products page. The recipe and option-slot editors are split into a part that edits plain data and the existing part that saves through the API.
- Backend: none. Depends on `catalog-import` (endpoints, `catalog:import` permission) and `sku-suggestion` being merged first.
- **Trust boundary: yes.** Roles (the page is for `catalog:import` holders, enforced by the backend), a user-loaded file (a JSON document that may come from an AI) parsed in the browser, and browser storage holding the organization's draft catalog.
