## Context

See proposal.md for the motivation. Current state:

- The SKU check the product code uses is `backend/internal/domain/aggregate/product/sku.go` (`^[a-zA-Z0-9\-]+$`). A second file, `backend/internal/domain/product/sku.go` (`^[a-zA-Z0-9]+$`), is imported nowhere. It misled the `catalog-import` design into assuming dashes are rejected.
- There is no database uniqueness on SKU. Collisions are avoided only by the generator.
- `frontend/components/products/ProductForm.tsx` validates SKU as required and at most 255 characters. Areas come from `PREPARATION_AREAS` in `frontend/types/product.ts` (`kitchen`, `bar`, `grill`, `sushi`, `desserts`), but the backend accepts any area string.
- `ProductsPageClient.tsx` already holds the organization's product list, so the form can get the taken SKUs without a new request.
- Apps never import each other's code, so the rule has to exist in TypeScript (form, grid) and in Go (`catalog-import` fills in SKUs missing from agent documents).

## Goals / Non-Goals

**Goals:**
- One rule, two implementations that cannot drift unnoticed.
- The suggestion is free (no request per keystroke).

**Non-Goals:**
- Regenerating or migrating SKUs of existing products.
- A database uniqueness constraint on SKU.
- A backend endpoint that suggests SKUs.

## Decisions

### D1. Generate in each app, check both against one example table

The generator is a pure function `(name, productType, area, takenSKUs) → sku`, in `frontend/lib/products/sku.ts` and in `backend/internal/domain/aggregate/product/sku_generate.go`. The example table in `specs/product-sku/spec.md` is normative. Each app's tests hold that table as a table-driven test, copied row by row.

*Alternative:* one shared fixtures file at the repo root, read by both test suites. Rejected because it makes each app's tests depend on files outside its folder, which the cross-app rules avoid. *Alternative:* a backend suggestion endpoint. Rejected because it costs a request per keystroke and fails offline on the edge.

### D2. The form tracks whether the user touched the SKU

`ProductForm` keeps a `skuTouched` flag, false on create and true on edit. Typing in the SKU field sets it, and emptying the field clears it. While it is false, a change of name, type or area recomputes the SKU. Taken SKUs come from `ProductsPageClient` as a prop (a `Set` of uppercase SKUs).

### D3. Delete the unused SKU package

Delete `backend/internal/domain/product/` and add a test to `aggregate/product` that `COC-BAN-PAI` is accepted. The accepted format does not change.

## Security — N/A

No trust boundary is touched: the generator runs on data the user can already see, and the backend validates every SKU as before.

## Risks / Trade-offs

- [The TypeScript and Go generators drift] → Both run the same example table, and a change to the rule must update the spec table first.
- [Two people create the same product at once and get the same suggestion] → No uniqueness in the database today, as before. The catalog import checks collisions against the whole organization.
- [The product list in the browser is stale, so a suggestion collides] → Same consequence as a hand-typed duplicate today. Accepted.
- [Prefixes are Spanish labels while area values are English] → The mapping is fixed in the rule. New areas fall back to their first 3 letters.

## Migration Plan

No migration. Ship the frontend and backend together in one PR. Rollback is reverting the PR. SKUs that were already suggested stay valid.
