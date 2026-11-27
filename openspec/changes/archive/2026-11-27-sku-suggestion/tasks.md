Backend commands run from `backend/`, frontend commands from `frontend/`.

## 1. Backend

- [x] 1.1 Delete the unused `internal/domain/product/` package and add a test in `internal/domain/aggregate/product` that `COC-BAN-PAI` is accepted and `COC BAN/PAI` is rejected. Verify: `go build ./...` and `go test ./internal/domain/aggregate/product/...` pass.
- [x] 1.2 Add the generator `sku_generate.go` in `internal/domain/aggregate/product` following the "SKU generation rule" requirement, with a table-driven test holding every row of the spec's example table. Verify: the test passes and has one case per table row.

## 2. Frontend

- [x] 2.1 Add `lib/products/sku.ts` with the same generator and `lib/products/sku.test.ts` holding every row of the spec's example table. Verify: `pnpm test` passes and the test has one case per table row.
- [x] 2.2 Fill the SKU in `ProductForm.tsx` (D2) and pass the organization's taken SKUs from `ProductsPageClient.tsx`. Verify: component tests for "Suggestion follows the name", "Suggestion avoids an existing SKU", "User's SKU is kept" and "Editing never regenerates".

## 3. Finish

- [x] 3.1 Run `make lint` and `go test ./...` from `backend/`, and `pnpm lint` and `pnpm test` from `frontend/`. Verify: all pass, or remaining lint findings are only in files this change did not touch.
