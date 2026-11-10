## 1. Schema

- [x] 1.1 Add a shared migration creating `plate_option_slots`, `plate_option_choices` (partial unique index: one non-deleted default per slot), `plate_option_choice_items` and `open_bill_product_choices`, all with `organization_id` and soft delete (D2); verify `make migrate-up` then `make migrate-down` round-trips on a local DB
- [x] 1.2 Include the new tables in the edge schema (they are not trimmed) and verify the migration embed and edge-trim tests pass

## 2. Domain: configuration

- [x] 2.1 Add DTOs in `domain/dto` (slot, choice, choice item, replace request, `LineChoice`) and domain errors for slot validation, side-dish/slot conflict and invalid line choices; verify `go build ./...`
- [x] 2.2 Add a `PlateOptionSlotRepository` port (find by composite, replace for composite, has-slots check, find changed for sync) and run `make generate-mocks`; verify the mocks compile
- [x] 2.3 Implement `PlateOptionSlotService.Get` and `Replace` with the validation rules from spec "A composite plate can be configured with option slots" and the exclusivity check against side-dish rows; verify with `plate_option_slot_service_test.go` covering every scenario of the first two requirements (90%+ on the service)
- [x] 2.4 Make `ProductIngredientService.ConfigureSideDish` reject a plate that has slots (409); add the test case in `product_ingredient_service_test.go` (ask before changing existing cases)

## 3. Domain: orders and stock

- [x] 3.1 Add `choices []string` to order line requests and `Choices []LineChoice` to line read/sync/event DTOs, plus a `SetChoices` mutator on `OpenBillProduct`; verify `go build ./...`
- [x] 3.2 Branch `resolveSideDishSelections` for slot plates (D4): validate choice ids, fill defaults, sum bundles into `[]SideDishSelection`, return the resolved `[]LineChoice`; reject choices on non-slot plates and side-dish quantities on slot plates. Verify with `order_service_test.go` cases for every scenario of "An order line for a plate with slots carries exactly one choice per slot", on create and on update
- [x] 3.3 Carry resolved choices into the aggregate, the open-bill sync payload and order events on create and update; verify with service tests asserting the outbox payload contains the choices
- [x] 3.4 Consume selection entries that are not recipe rows in `StockEventHandler` (D5), expanding composite bundle items; verify with stock handler tests: canastas from two slots = 3, fries ×2, a choice change returns the old bundle, voiding returns everything, and legacy side-dish plates unchanged

## 4. Persistence and sync

- [x] 4.1 Implement the GORM `PlateOptionSlotRepository` (replace in the unit of work, keep ids of resent choices, soft-delete omitted ones, organization-scoped); verify with an acceptance test against testcontainers (`DB_HOST= make test-acceptance`)
- [x] 4.2 Persist and load `open_bill_product_choices` in `open_bill_repository.go` alongside side dishes; verify reads return the snapshot names after a choice is renamed (acceptance)
- [x] 4.3 Accept optional line choices in `open_bill_sync_applier.go`, tolerating their absence from old nodes; verify with an applier test for both payload shapes
- [x] 4.4 Add the slot list as a cloud→edge pull reference entity, one document per plate (D6), following the reference-entity recipe; verify with `sync_reference_service_test.go` and an acceptance test where a PUT in the cloud reaches the box on the next pull

## 5. HTTP

- [x] 5.1 Add a `PlateOptionSlotHandler`: `GET /api/products/:id/option-slots` in both modes, `PUT` cloud-only; map validation errors to 400 and the conflict to 409; wire it in `internal/bootstrap`; verify with handler tests and updated `routes_cloud.golden` / `routes_edge.golden` (the edge serves no PUT)
- [x] 5.2 Map invalid line-choice errors to 400 in `order_handler.go` and return `choices` on order reads and kitchen-feed lines and SSE events; verify with handler and SSE tests

## 6. MCP

- [x] 6.1 Customer toolset: `get_side_dish_options` returns slots and choices when the plate has slots (D8), `list_menu` flags slot plates, and `create_delivery_order` / `update_my_order` accept `choices` and return resolved choice names; update the tool descriptions. Verify with `tools_customer_test.go` cases for the modified customer-agent-access scenarios
- [x] 6.2 Full toolset: add get/set plate option slot tools next to the product-ingredient tools; verify with a tool test and the tool count in `docs/mcp-server.md`

## 7. Docs and verification

- [x] 7.1 Document the option-slot endpoints in `docs/api/products.md` and line `choices` (request, response, errors) in `docs/api/orders.md` in the products.md format, and add curl examples (configure the tilapia, order it with "Papas") to `docs/examples/curl-examples.md`
- [x] 7.2 Write a frontend handoff note in the change folder: POS slot rendering, and moving a plate to slots only after the POS supports it
- [x] 7.3 Run `make lint`, `go build ./...`, `go test ./...` and `DB_HOST= make test-acceptance` and verify all pass

## 8. Slots replace side dishes

- [x] 8.1 Add twin cloud/edge migrations deleting side-dish recipe rows; verify with a migration acceptance test on both sets
- [x] 8.2 Remove the side-dish endpoints, service methods, port method, DTO fields, errors and the slot/side-dish conflict; refuse `side_dishes` on order lines with a 400; verify with service, handler and golden route tests
- [x] 8.3 Remove the side-dish MCP paths and tools, keeping `get_side_dish_options` returning slots only; verify with MCP tests and the tool count
- [x] 8.4 Update the API docs, MCP docs and frontend handoff; run `make lint`, `go test ./...` and `DB_HOST= make test-acceptance`
