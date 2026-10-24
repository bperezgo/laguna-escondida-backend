## 1. Prerequisites

- [x] 1.1 Confirm `split-cloud-edge-binaries` is merged and `internal/bootstrap/testdata/routes_edge.golden` exists. Verify: `go test ./internal/bootstrap -run TestRoutes_MatchGolden` passes on `main`.

## 2. Split the migration sets

- [x] 2.1 Move `migrations/000001..000071` to `migrations/shared/` with `git mv`. Add empty `cloud/` and `edge/` directories, each holding a `.keep` file. Embed `shared` plus one mode directory per binary as a single `fs.FS`. Verify: both binaries migrate an empty database to v71, and `test/acceptance/migration` passes.
- [x] 2.2 Add `TestEdgeCloudSharedSchemaParity` (design D3). It migrates two containers with shared plus cloud and with shared plus edge, then compares columns, types, nullability, defaults, keys and foreign keys of the common tables. It also asserts the edge's expected table set. Verify: it passes with both sets at v71. A scratch `cloud/000072` that adds a column to `bills` makes it fail and name `bills`; remove that file afterwards.

## 3. Edge wiring: the restaurant floor only

- [x] 3.1 Move the expense, purchase-entry, supplier and financial services, their repositories, handlers and routes (including the daily close and `GET /api/products/:id/suppliers`) from `buildCore` into the cloud assembly. Verify: `go build ./...`; the cloud golden file is unchanged; the edge golden file loses exactly those routes. Regenerate the edge golden file and review the diff in the PR.
- [x] 3.2 Register only the read routes on the edge for products, categories, ingredients, side dishes, responsibilities, users and roles. Remove the admin-key `POST /api/users` from the edge. Verify: the edge golden diff removes exactly the write routes. A bootstrap test asserts that no edge route reaches `BulkCreateProducts`, the catalog writes or the user writes. In the cloud, the moved guard tests (agent routes) still pass.
- [x] 3.3 Add `orderError.ErrDeliveryCloudOnly` and the `DeliveryEnabled` option (design D5). Wire an empty `DeliveryZoneFinder` on the edge, and map the error to `422 DELIVERY_CLOUD_ONLY` in the order handler. Verify with new `OrderService` tests:
  - delivery refused before the zone lookup when disabled;
  - takeaway unaffected;
  - delivery inside a zone accepted when enabled.

  A handler test checks the 422 body. Update `docs/api/orders.md` and the curl example.

## 4. Edge sync compatibility

- [x] 4.1 The edge pull applier skips suppliers when the `suppliers` table is absent (detected once at startup, design D6), and the cursor still advances. Verify with integration tests on a trimmed edge database and an untrimmed one: a pull with products, users and suppliers applies the products and users on both. On the untrimmed database the suppliers are applied too, and no error occurs on either.
- [x] 4.2 Confirm the cloud's pull still returns suppliers and its push still applies `purchase_entry`. Verify: the existing cloud pull and push tests pass unchanged, plus an acceptance test where a queued pre-update purchase-entry op is pushed from a trimmed edge.

## 5. The guarded trim

- [x] 5.1 Add edge migration `000072_trim_edge_schema.{up,down}.sql`. The up drops `fk_open_bills_delivery_zone` and the 13 tables listed in the spec. The down recreates them empty, as at v71, so local rigs can go back down. Verify: `TestEdgeCloudSharedSchemaParity` passes, with the edge table-set assertion updated and the foreign key as the only allowed difference.
- [x] 5.2 Add the pre-check `edgeTrimBlockers` (design D2). The edge runner checks it at v71 and stops at v71 when there are blockers, logging each table and count at error level. Verify with integration tests:
  - clean box (pulled suppliers, seeded categories, confirmed purchase entries) → trimmed to v72;
  - one expense → stays at v71, boots, and logs `expenses: 1`;
  - extra category → blocked;
  - unconfirmed purchase-entry outbox op → blocked; once it's marked synced, the next start trims.
- [x] 5.3 Verify the untrimmed box keeps working: run the edge acceptance suite (`test/acceptance/sync`) against an edge database blocked at v71 with the new edge build. Orders, pull and push must all pass.

## 6. Docs and handoff

- [x] 6.1 Docs:
  - in each `docs/api/*.md` for a removed edge endpoint, add a "Cloud only" note;
  - document the migration-set rule ("a change to a shared table goes in both `cloud/` and `edge/`") in `docs/playbooks/ARCHITECTURE.md`;
  - add a box-blocker runbook (export, re-enter in the cloud, delete, restart) to the sync playbook.

  Verify: `grep -rn 'Cloud only' docs/api` covers every route the edge golden file dropped.
- [ ] 6.2 Write the frontend handoff in the PR:
  - mark Gastos, Finanzas, Cierre de Caja, Entradas de Compra, Proveedores, Facturas and Documentos Soporte `cloudOnly`;
  - hide the product, ingredient, responsibility and user edit actions on the box;
  - the box returns `DELIVERY_CLOUD_ONLY` for deliveries.

## 7. Verify end to end

- [ ] 7.1 Upgrade a copy of an edge database at v71 holding realistic data (scratch `edge_box`). Check that it trims, signs in, pulls, takes and pushes an order, serves the catalog reads, returns `404` on a product edit and `422` on a delivery, and prints if a printer is configured. Repeat with one expense inserted and check that it stays at v71 and still takes orders.
- [x] 7.2 Run `make lint`, `go build ./...`, `go test ./...` and `make test-acceptance`. All must pass.
- [ ] 7.3 Before the release, read the pre-check counts on Laguna's real box and record them in the PR (Migration Plan step 3).
