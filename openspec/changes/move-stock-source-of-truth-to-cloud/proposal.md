## Why

On-hand stock lives on the Windows box. The edge is the single writer and the cloud holds a read-only mirror, enforced by one applier: `stock_sync_applier.go:56-63` assigns the cloud's `amount` from the edge's snapshot, last-snapshot-wins. Every stock task that is *not* a sale — recording a supplier invoice, correcting a count, seeding a product that just arrived — is done in the office, and the office cannot write.

That rule is already leaking. `POST /api/purchase-entries` is registered in **both** modes (`cmd/main.go:431`) and its stock subscriber runs in both (`cmd/main.go:284-290`), so an invoice recorded against the cloud *does* increase the cloud's number — and the edge's next snapshot for that product erases it. The increase is lost silently, today, with no error anywhere.

The planned fix, `dispatch-cloud-actions-to-edge`, builds an entire cloud→edge action protocol (47 tasks: a `node_actions` lifecycle, an authenticated op feed, executors, expiry sweeps, basis tolerances, rejection reasons) so that the office can *ask* the box to change a number. That machinery exists only to preserve edge ownership. Two findings say ownership is the thing to change instead:

- **Nothing reads on-hand to make a decision.** There is no availability check anywhere in the codebase; a sale whose decrement fails is logged and the sale proceeds (`stock_event_handler.go:189-195`). `StockService` is consumed only by `stock_handler.go`. The edge's relationship to stock is write-only.
- **The cloud already receives every movement as a signed delta.** Each stock write appends a `historic_stock` row carrying `change` and a cross-node `op_id` (`dto/stock.go:38-48`), pushed as `SyncEntityHistoricStock` and applied exactly once through `sync_inbox`, one transaction per op.

The cloud already holds a complete, deduplicated movement log. It throws that log away as accounting and assigns the number from the edge's absolute instead. Stop assigning, start folding, and the office can write stock directly — no action protocol, no dispatch lifecycle, no tolerance windows.

## What Changes

- **The cloud folds movements into on-hand.** The `historic_stock` applier becomes the writer of `stock.amount`: `amount += change`, in the same transaction that records the inbox row, so the existing exactly-once guarantee carries the fold. A product with no stock row yet is created at `change`.
- **The cloud stops applying the edge's absolute.** The `stock` snapshot op is no longer assigned to `amount`; the edge stops emitting it. Absolutes are what made a second writer impossible, and after this change there is only one writer of the absolute — the cloud.
- **Human-authored stock writes move to cloud mode.** `POST /api/stock`, `PUT /api/stock/:product_id/add-or-decrease`, `DELETE /api/stock/:product_id` and `POST /api/stock/bulk` move out of the edge branch (`cmd/main.go:481-484`) into the cloud branch. The box writes stock only as a consequence of things that must work offline: sales decrements, and a purchase entry if a delivery is recorded on site.
- **Every movement stays a delta, including the batch count.** `POST /api/stock/bulk` keeps converting a counted amount into `change = counted - current`, now computed against the cloud's own number, so the delta lands on exactly the base it was derived from and commutes with concurrent edge sales. No movement ever sets an absolute.
- **The ledger and the amount tell the same story.** `stock.amount == SUM(historic_stock.change)` per product becomes an enforced invariant on the cloud, checkable by a reconciliation job. Cutover writes one reconciling `opening_balance` row per product so the sum holds from the flip forward, because pre-sync history was never pushed. `historic_stock` gains a `kind` so the ledger explains itself (sale, purchase, adjustment, count, opening balance).
- **`stock` flips from a push entity to a pull entity.** The edge keeps a display-only cache refreshed by a new daily job, so the restaurant sees correct numbers from the start of each day including office-side purchases and counts. Daily rather than on the existing reference-pull cron, because `stock.updated_at` churns on every sale and the edge needs freshness far less than it needs the bandwidth.
- **The edge keeps no movement history.** It still writes its local `historic_stock` row as the source of the op it pushes, but nothing on the edge reads it; the cloud is where movement history is queried.
- **Recording an invoice in the cloud stops being a bug** and becomes the intended path — once the fold and the snapshot retirement are both in place.
- **BREAKING (operational, and an API surface move):** the four stock write endpoints answer on the cloud and return `404` on the edge, inverting today's registration. The frontend's stock screens must target the cloud.
- **Supersedes `dispatch-cloud-actions-to-edge`,** whose only action type was `stock.adjust`. That change is removed; its transport idea (a cloud→edge op feed) stays available for the still-undelivered cloud-origin `bill` CUFE/Tascode ops, which this change does not address.

Out of scope: gating a sale on availability (nothing gates today and this change does not add it); multi-edge fan-out; delivering the cloud's stranded `bill` ops; the frontend warning UI for batch counts (backend exposes the lag, the web app renders it); pruning the edge's now-unread `historic_stock` rows.

## Capabilities

### New Capabilities
- `cloud-stock-ownership`: The cloud side — folding replicated movement deltas into on-hand exactly once, refusing to take an absolute from any peer, authoring purchases, adjustments, deletes and batch counts locally, the ledger-equals-amount invariant, and the cutover baseline that makes it true.
- `edge-stock-cache`: The edge side — a display-only copy of the cloud's numbers refreshed on a daily pull, sale decrements that keep working offline and replicate as deltas, no human-authored stock writes, and the sync-lag signal the batch screen needs to warn a counter that the cloud is behind.

### Modified Capabilities
- (none — the project has no durable specs yet; `openspec list --specs` returns nothing.)

## Impact

- **Appliers:** `historic_stock_sync_applier.go` becomes the fold and the only writer of replicated on-hand; `stock_sync_applier.go` stops assigning `amount` (retained only if a peer-reported value is kept for drift detection).
- **Sync direction:** `stock` moves out of the push entity set and into the pull reference set — `SyncReferenceService.ChangesSince` and its reader gain stock, the edge gains a stock applier for pulled rows and a new scheduler job. `historic_stock` stays push, edge → cloud.
- **Services:** `StockService` and `StockEventHandler` stop appending `stock` snapshot ops; the cloud must not append sync ops for its own stock writes, since no peer consumes them (mechanism in `design.md`).
- **Routes:** four stock write endpoints move from the edge branch to the cloud branch of `cmd/main.go`; `GET /api/stock` stays registered in both.
- **Schema:** `historic_stock.kind`; no change to `stock`.
- **Config:** a daily `STOCK_PULL_CRON` on the edge.
- **Cutover:** drain the edge outbox, flip the appliers, write one `opening_balance` row per product. No data migration — the cloud mirror already equals the edge's amounts by construction.
- **Docs:** `docs/playbooks/SYNC_ACCEPTANCE_SPEC.md` gains the inverted stock flow and new invariants and must retire the "edge is the single writer of stock" statement; `docs/api/stock.md` (does not exist yet); the `ingest-invoice` skill's claim that a purchase entry auto-updates stock becomes true for the cloud.
