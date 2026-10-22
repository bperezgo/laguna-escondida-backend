## 1. Schema and shared types

- [x] 1.1 Add migration `000059_add_kind_to_historic_stock` adding `historic_stock.kind VARCHAR NOT NULL DEFAULT 'unknown'` with a CHECK covering `sale`, `purchase`, `adjustment`, `count`, `opening_balance`, `unknown`; verify `make migrate-up` then `make migrate-down` runs clean on a local database and existing rows survive
- [x] 1.2 Add `dto.StockMovementKind` and the `Kind` field on `dto.HistoricStock` and `dto.HistoricStockSyncPayload`, defaulting to `unknown` when a peer sends no kind; verify `go build ./...` and that the existing sync payload round-trips in a repository test
- [x] 1.3 Set the kind at every existing write site (sale decrement and restore, purchase increment, `CreateStock`, `AddOrDecreaseStock`, `BulkStockCreationOrUpdating`); verify a service test asserts the kind written for each path

## 2. Cloud: fold movements into on-hand

- [x] 2.1 Extend `HistoricStockSyncApplier` to apply the movement — `amount += change` on the product's live stock row, creating the row at `change` when none exists, joining the ambient transaction via `GetTxOrDB`; verify an integration test covers fold, row creation, and a result that goes negative
- [x] 2.2 Verify the fold is exactly-once by test: replay the same op id through `ApplyPush` and assert the amount moved once and one ledger row exists (the inbox early-return in `sync_service.go:50` is the guarantee; the test pins it)
- [x] 2.3 Add a test asserting order independence: apply the same set of movements for one product in two different orders and assert the same final amount
- [x] 2.4 Confirm the applier still dedupes its own insert on `op_id` and that a conflicting insert does not double-apply the amount; verify with an integration test that inserts the same ledger row twice

## 3. Cloud: stop accepting an absolute

- [x] 3.1 Change `StockSyncApplier` so a replicated snapshot no longer assigns `amount` (keep the delete/tombstone path); verify an integration test asserts the amount is unchanged after applying a snapshot that disagrees with it
- [x] 3.2 Add a test proving a peer snapshot cannot erase a cloud-authored increase: record a purchase, then apply an older snapshot, assert the purchase survives
- [x] 3.3 Decide and document in code whether `SyncEntityStock` remains registered at all on the cloud once it no longer writes; verify the applier registry in `cmd/main.go` matches the decision and the sync integration suite passes

## 4. Cloud: stock authoring moves here

- [x] 4.1 Move `POST /api/stock`, `PUT /api/stock/:product_id/add-or-decrease`, `DELETE /api/stock/:product_id` and `POST /api/stock/bulk` from the edge branch (`cmd/main.go:481-484`) to the cloud branch; verify a routing test asserts each returns `404` in edge mode and is served in cloud mode
- [x] 4.2 Confirm `BulkStockCreationOrUpdating` still derives `change = counted - current` and skips zero-change items, now against the cloud's own amounts; verify existing `stock_service_test.go` cases still pass unchanged — ask before altering any existing assertion
- [x] 4.3 Add a service test for the count-then-sale ordering: apply a count that sets on-hand to 20, then fold a replicated `-1`, assert 19
- [x] 4.4 Ensure every authoring path writes its movement with the right kind and that no authoring path writes an absolute to the ledger; verify by asserting the ledger rows produced by each endpoint

## 5. Cloud: reconciliation and the freshness signal

- [x] 5.1 Implement a reconciliation use case that compares each product's `amount` against `SUM(historic_stock.change)` and returns the diverged products with both values; verify a service test covers a matching product, a diverged product, and a product with no movements
- [x] 5.2 Schedule the reconciliation as a cloud cron in report-only mode, logging divergences without correcting them; verify the scheduler test covers registration and that a job error is logged rather than propagated
- [x] 5.3 Expose how recently the cloud last applied a movement replicated from the edge (`MAX(sync_inbox.applied_at)` over edge-origin ops) through a read endpoint the batch screen can call; verify a handler test covers a fresh edge, a stale edge, and a node that has never pushed
- [x] 5.4 Document the endpoint in `docs/api/stock.md` so the frontend can render the staleness warning; verify the doc includes a realistic response and the meaning of the value

## 6. Edge: emit deltas only, cache the cloud's numbers

- [x] 6.1 Introduce a stock movement emitter injected into `StockService` and `StockEventHandler` — real on the edge (appends the outbox op), no-op on the cloud — replacing the unconditional `appendStockOutbox`/outbox append inside `createAndSyncHistoric`; verify service tests assert an op is queued in the edge configuration and none in the cloud configuration
- [x] 6.2 Stop emitting the `stock` snapshot op from every edge write path while keeping the `historic_stock` movement op; verify a service test asserts exactly one queued op per movement and that it carries the change
- [x] 6.3 Add stock to the pull reference reader and `SyncReferenceService.ChangesSince` (amount, unit of measure, timestamps, `deleted_at`); verify a repository integration test covers changed, unchanged, and soft-deleted rows against a cursor
- [x] 6.4 Add `sync_state` storage for a stock-pull cursor independent of `last_pulled_cursor`; verify an integration test covers first read, advance, and that a stale advance does not move it backwards
- [x] 6.5 Implement the edge's stock refresh — replace local amounts from the pulled rows and advance its own cursor in one transaction; verify a service test covers a first full refresh, an incremental refresh, a soft-deleted row, and an unreachable cloud leaving amounts and cursor untouched
- [x] 6.6 Add the daily refresh job to `EdgeSyncScheduler` with its own cron expression, logging counts and never failing fatally; verify the scheduler test covers registration and error containment
- [x] 6.7 Add a test asserting a refresh does not discard queued movements: queue local sales, run a refresh, assert the outbox is intact and the movements still push

## 7. Configuration and wiring

- [x] 7.1 Add `STOCK_PULL_CRON` (edge, daily default) to `internal/platform/config/config.go`, validated only in edge mode; verify config tests cover the default and an invalid expression
- [x] 7.2 Wire the cloud branch of `cmd/main.go` — folding applier, no-op movement emitter, the four authoring routes, the reconciliation cron, the freshness endpoint; verify the binary boots in cloud mode and the routes respond
- [x] 7.3 Wire the edge branch of `cmd/main.go` — real movement emitter, stock pull client and service, the daily job, and no stock authoring routes; verify the binary boots in edge mode and logs one no-op refresh cycle
- [x] 7.4 Run `make generate-mocks` (or `make regenerate-mocks`) for any new or changed port and verify the mocks appear in `internal/domain/ports/mocks/` without hand edits

## 8. Acceptance tests (Tier 1, two-node)

- [x] 8.1 Extend `test/acceptance/sync/harness_test.go` with helpers to author stock on the cloud and to run one edge refresh synchronously (no sleeps, no cron); verify the helpers compile and the existing acceptance suite still passes
- [x] 8.2 Add `TestSync_Stock_CloudFoldsEdgeSales` — sell on the edge, push, assert the cloud's amount decreased by the sale and the ledger row is present
- [x] 8.3 Add `TestSync_Stock_CloudPurchaseSurvivesEdgeTraffic` — record a purchase on the cloud, then push edge sales, assert the cloud's amount reflects both and the purchase was not erased
- [x] 8.4 Add `TestSync_Stock_EdgeNeverSendsAbsolute` — inspect what the edge queues for a movement and assert no op carries an on-hand amount the cloud could assign
- [x] 8.5 Add `TestSync_Stock_OfflineThenDrains` — sell against an unreachable cloud, assert sales complete and stay queued, then restore and assert every movement folds once
- [x] 8.6 Add `TestSync_Stock_DailyRefreshAdoptsCloudNumbers` — record a purchase and a count on the cloud, run one edge refresh, assert the edge's amounts match the cloud's
- [x] 8.7 Add `TestSync_Stock_LedgerSumsToAmount` — after a mixed run of sales, a purchase and a count, assert `amount == SUM(change)` for every touched product on the cloud
- [x] 8.8 Run the whole suite with `RUN_ACCEPTANCE_TESTS=true make test-acceptance`

## 9. Cutover, documentation and verification

- [x] 9.1 Write the cutover runbook implementing the Migration Plan order (drain the edge outbox, flip the appliers, write opening balances, move the routes, ship the edge refresh) with the rollback boundary called out; verify a dry run on the local docker rig per `docs/playbooks/SYNC_LOCAL_TESTING.md`
- [x] 9.2 Implement the opening-balance step as a repeatable command that writes one `opening_balance` movement per product so `SUM(change)` equals the adopted amount, and is a no-op on second run; verify an integration test covers both runs and a product with no movements
- [x] 9.3 Update `docs/playbooks/SYNC_ACCEPTANCE_SPEC.md` — describe the inverted stock flow, retire the "edge is the single writer of stock" statement and `SYNC-INV-12`'s surrounding assumptions where they conflict, and add the new invariants (the cloud never adopts a peer's absolute; a movement folds exactly once; on-hand equals the sum of its movements) mapped to the tests from section 8; verify each new test's doc comment carries its invariant id
- [x] 9.4 Write `docs/api/stock.md` (it does not exist yet) covering the cloud-only write endpoints, the shared read endpoint, the freshness signal, and the batch-count staleness caveat; add working examples to `docs/examples/curl-examples.md`
- [x] 9.5 Update the `ingest-invoice` skill and `docs/api/` purchase-entry documentation to state that recording a purchase in the cloud now updates stock for real; verify the skill's stated warnings no longer contradict the behavior
- [x] 9.6 Run `go build ./...`, `make lint`, `make test`, and the integration suite with `RUN_INTEGRATION_TESTS=true`; fix every finding and confirm all suites are green
