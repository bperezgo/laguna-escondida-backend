## 1. Definition layer — schema & DTOs

- [x] 1.1 Add migration to `platform/postgres/migrations/` that renames `product_ingredient.quantity` → `default_quantity` (value-preserving) and adds `is_side_dish` (default false), `min_quantity`, `max_quantity`; verify migrate up/down succeeds on a testcontainer DB and existing amounts carry over
- [x] 1.2 Rename `Quantity` → `DefaultQuantity` on the `product_ingredient` DTO and sweep every reference (repository queries, `stock_event_handler` expansion `ingredient.Quantity`, `product_ingredient_service`); add `IsSideDish`, `MinQuantity`, `MaxQuantity` and side-dish request DTOs; run `make generate-mocks` and verify `go build ./...` and existing `go test ./...` pass

## 2. Definition layer — ports, service, tests

- [x] 2.1 Extend `ProductIngredientRepository` port to persist/read side-dish fields; run `make generate-mocks` and verify mocks regenerate cleanly
- [x] 2.2 Implement side-dish config in `product_ingredient_service` (set/clear side dish; enforce `0 <= min <= default <= max`); verify service unit tests for accept and reject-bounds cases pass
- [x] 2.3 Implement repository read/write of side-dish fields in `platform/postgres/repository`; verify an integration test round-trips a side-dish option
- [x] 2.4 Add/extend handler + route to configure and list a product's side-dish options; verify the list endpoint returns default/min/max

## 3. Order line — DTOs & event plumbing

- [x] 3.1 Add `SideDishSelection{IngredientProductID, Quantity}` and a selections slice to `OrderProductItem` in `domain/dto/open_bill.go`; verify `go build ./...`
- [x] 3.2 Add the same field (the full resolved set) to `OrderCreatedEventProduct` in `domain/dto/order_event.go` and fix the direct struct conversions (`OrderCreatedEventProduct(p)`); verify a test asserts selections survive the event `Data()` JSON round-trip and appear in `PreviousProducts`/`CurrentProducts` of the update event
- [x] 3.3 Persist and read resolved side-dish selections per `open_bill_product` line (child storage + migration); verify a repository test round-trips a line with selections

## 4. Order line — validation

- [x] 4.1 In `order_service` create/update, load the product's side-dish config and validate each selection is an allowed option within `[min, max]`; verify unit tests cover reject-above-max, reject-non-side-dish, and remove-to-zero
- [x] 4.2 Resolve unspecified side dishes to their defaults when building the line; verify a unit test shows a no-selection line resolves to default quantities

## 5. Stock consumption

- [x] 5.1 Update `stock_event_handler.expandComposite` to consume side-dish rows at the line's selected quantity and fixed rows at default; verify a unit test: default plate consumes today's amounts (backward compat)
- [x] 5.2 Handle create consumption from selections; verify a unit test: salad 0 / canasta 3 consumes 0 salad and 3 canasta and default fixed ingredients
- [x] 5.3 Include side-dish quantities in the `OrderUpdated` stock diff; verify unit tests: raising canasta 2→3 decrements one more, lowering 3→1 restores two
- [x] 5.4 Restore side-dish stock on `OrderDeleted` from the stored selection; verify a unit test restores the recorded quantities

## 6. Kitchen feed & printed ticket

- [x] 6.1 Add `SideDishes []{Name, Quantity}` to `OpenBillProductSSE`; in `HandleOrderCreatedSSE` resolve ingredient names (batch the lookup, one query per order) and populate it; verify an order-service test asserts the SSE payload for a line carries the resolved side dishes with names
- [x] 6.2 Mirror the enrichment in `HandleOrderUpdatedSSE` (~`order_service.go:937`) so edited side dishes re-notify the area; verify a test shows an updated line re-notifies with the new selections
- [x] 6.3 ~~Render resolved side dishes on the printed ticket in `print_service`~~ — dropped: the only printed document is the customer "cuenta" (a billing receipt), where side dishes (no price effect) are noise; the preparation area is served by the SSE feed (6.1/6.2). See design D6.

## 7. Sync

- [x] 7.1 Wire the side-dish definition as a cloud→edge pull reference (mirror the product/ingredient pull recipe); verify a sync test pulls a side-dish option to the edge
- [x] 7.2 Ensure order-line selections push edge→cloud with open_bills; verify a sync test replicates a line's selections

## 8. Docs & validation

- [x] 8.1 Update `docs/api/` for the side-dish config endpoints and the order create/update payload; add curl examples
- [x] 8.2 Run `make lint`, `go build ./...`, and `go test ./...`; verify all pass and `openspec validate manage-order-side-dishes --strict` succeeds
