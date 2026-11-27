## Why

Typing a SKU for every product is the most tedious part of loading a catalog, and owners invent inconsistent codes. A readable SKU can be derived from the product's preparation area and name, so the system should suggest one instead of asking for it. The same rule is needed by the catalog import (agent documents) and the catalog grid, so it must be defined once.

## What Changes

- One SKU generation rule: `AREA-WORD-WORD-WORD`, uppercase, accents folded, filler words dropped, up to 3 name words of 3 letters each, a numeric suffix (`-2`, `-3`, ...) on collision. Examples: "Bandeja paisa" in Cocina → `COC-BAN-PAI`; ingredient "Arroz" → `ING-ARR`.
- The product create form fills the SKU from name, product type and preparation area while the user types, until the user edits the SKU field. Editing an existing product never changes its SKU.
- The suggestion avoids SKUs already used in the organization.
- Remove the unused `internal/domain/product/sku.go`, whose stricter format (`[a-zA-Z0-9]+`, no dashes) contradicts the SKU check the product code actually uses (`aggregate/product/sku.go`, which accepts `-`). The accepted SKU format does not change.
- A table of example name → SKU pairs in the spec, used as test cases by both the frontend generator and the backend generator (the backend generator is used by `catalog-import`).

## Capabilities

### New Capabilities

- `product-sku`: the SKU format the system accepts, the generation rule, and the suggestion in the product form.

### Modified Capabilities

None.

## Impact

- Frontend: new SKU generator in `lib/products/`, `ProductForm.tsx` fills the SKU, `ProductsPageClient.tsx` passes the organization's SKUs to the form.
- Backend: new SKU generator next to `aggregate/product/sku.go` (used by `catalog-import`), deletion of the unused `domain/product/sku.go`, a test that dashed SKUs are accepted.
- No API, schema or sync change. Edges already accept dashed SKUs.
- `catalog-import` depends on this change for SKU generation (its D6 decision points here).
- **Trust boundary: no.** The change touches no auth, roles, tenant scoping, endpoints, sync or MCP tools. The suggestion is computed in the browser from the products the user can already see, and the backend still validates every SKU.
