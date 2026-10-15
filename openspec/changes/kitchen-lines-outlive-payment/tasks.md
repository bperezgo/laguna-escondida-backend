## 1. Schema

- [x] 1.1 Add migration `000061_paid_orders_and_line_timing` adding nullable `open_bills.paid_at`, `open_bills_products.started_at`, `open_bills_products.completed_at`; verify `make migrate-up` then `make migrate-down` run clean locally and existing rows read back with all three NULL

## 2. Domain

- [x] 2.1 Add `paidAt` and `IsPaid()` to the open-bill aggregate (through `NewAggregateFromRepository` and `ToDTO`), and make `CancelProduct` return a new `ErrCannotCancelPaidOrderLine` when the order is paid; verify `go build ./...`
- [x] 2.2 Add `startedAt` / `completedAt` to `OpenBillProduct` with the transition rules from design D4 (in-progress sets the start if empty; complete sets the completion; uncomplete clears the completion, and the start when coming from `in_progress`); expose them through the line DTOs; verify `go build ./...`
- [x] 2.3 Add `ClosePaid(ctx, id, paidAt)` and `FindAggregateForKitchen(ctx, id)` to the open-bill port, then run `make generate-mocks`; verify `go build ./...`
- [x] 2.4 Change `OrderService.PayOrder` to call `ClosePaid` instead of `Delete` and to append an `update` outbox snapshot carrying `ClosedAt`/`PaidAt` instead of the delete tombstone. **Ask before changing the existing `PayOrder` tests**, then cover: paid instant recorded, snapshot carries both instants, paying an already-paid order fails as not found. Verify `go test ./internal/domain/service/...`
- [x] 2.5 Switch `CompleteOpenBillProduct`, `UncompleteOpenBillProduct`, `SetOpenBillProductInProgress` and `CancelOpenBillProduct` to `FindAggregateForKitchen`. **Ask before changing existing tests**, then add `TestOrderService_*_PaidOrder` cases: complete, uncomplete and in-progress succeed on a paid order; cancel returns `ErrCannotCancelPaidOrderLine`; the outbox snapshot for a paid order carries `ClosedAt`/`PaidAt`. Verify `go test ./internal/domain/service/...`
- [x] 2.6 Add service tests for the line timestamps: in-progress then complete records both; complete from `created` records only completion; uncomplete clears completion and keeps start; in-progress back to created clears start; `UpdateOrder` on an open order leaves existing timestamps untouched. Verify the tests pass

## 3. Repository and sync

- [x] 3.1 Add the kitchen-visible predicate helper (design D3) and use it in `FindPendingByArea`, `FindCompletedByAreaBetween` and the new `FindAggregateForKitchen`; implement `ClosePaid`; read and write `paid_at`, `started_at`, `completed_at` in the models and in `UpdateProductStatus`; switch completion time to `COALESCE(completed_at, updated_at)` in both kitchen reads, including the completed-review window. Verify with a testcontainers acceptance test: a paid order's unfinished line is on the board, a voided order's is not, a historic closed order (`paid_at` NULL) is not, and a completed paid line shows in the review with its `completed_at`
- [x] 3.2 Add `ClosedAt`/`PaidAt` to `dto.OpenBillSyncPayload` and `StartedAt`/`CompletedAt` to `dto.OpenBillSyncProduct` (`omitempty`), fill them in `appendOpenBillOutbox` / `openBillSyncProducts`, and make `open_bill_sync_applier.go` assign `deleted_at`, `paid_at` and the line timestamps from the payload. Verify with acceptance tests: (a) a pay snapshot closes and marks paid; (b) a later line update to that paid order keeps it closed and paid; (c) a payload with none of the new fields reopens a closed order exactly as today
- [x] 3.3 Map `ErrCannotCancelPaidOrderLine` to HTTP 409 in `OrderHandler.CancelOpenBillProductHandler`; verify with a handler or acceptance test

## 4. Docs

- [x] 4.1 Update `docs/api/orders.md`: line-state endpoints work on paid orders, cancel on a paid order returns 409 with its error body, `completed_at` is the real completion instant, and pay closes the order but not its kitchen lines. Add a curl example of completing a line after payment to `docs/examples/curl-examples.md`. Verify the example runs against `make run`

## 5. Verification and rollout

- [x] 5.1 Run `make lint`, `go build ./...`, `go test ./...` and `make test-acceptance`; all clean
- [x] 5.2 End-to-end on a local edge + cloud pair: create a dine-in order with a kitchen line, pay it, see the line still on the kitchen board, complete it, push, and confirm in the cloud the order is closed, has `paid_at`, and the line is completed with `completed_at`
- [ ] 5.3 Before the edge release, confirm the production cloud runs this change's applier (check the deployed version) — the edge must not ship first (design D5)
