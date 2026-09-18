## Why

Plates are composite products with a fixed bill-of-materials, but customers routinely ask to drop a side dish (no salad) or take more/less of another (3 canasta patacón instead of 2). Servers record this today as free text in the order line `Notes`, so stock still decrements the default recipe regardless of what the customer actually got — the on-hand inventory drifts from reality. This change makes side-dish modifications structured so stock consumption reflects the real plate.

Linear ticket: **BSP-33**.

## What Changes

- Add a **definition layer**: a composite product's ingredients can be marked as swappable "side dishes", each with a `default`, `min`, and `max` quantity. Fixed ingredients (the protein, oil, base) are unchanged and keep consuming their exact recipe amount. An "offered alternative" (e.g. fries) is simply a side dish whose default is `0`.
- Add a **transaction layer**: an order line records the resolved quantity chosen per side dish. The server's +/- adjustments must stay within each side dish's `[min, max]`.
- **Stock correctness**: the stock event handler consumes the per-line side-dish quantities (create, update-diff, and delete-restore) instead of the default recipe for side-dish ingredients; fixed ingredients still expand from the default BOM.
- A "swap" needs no special primitive — it is decrementing one side dish and incrementing another, each within its own limits.
- Price is **not** affected: side-dish changes never raise or lower the plate price. The `min`/`max` bounds are the guardrail against abuse, replacing the price signal.
- Backward compatible: an order line with no side-dish selections resolves to the defaults, i.e. today's behavior.

Out of scope (deferred to a follow-up ticket): cross-option constraints such as mutual exclusion ("if fries then no canasta") or a total-side-dish cap. This change implements only per-side-dish `min`/`max`.

## Capabilities

### New Capabilities
- `product-side-dish-configuration`: Defining, on a composite product, which ingredients are customer-configurable side dishes and the `default`/`min`/`max` quantity each may take.
- `order-side-dish-selection`: Recording per-order-line side-dish quantities within the configured bounds, and consuming stock according to those quantities across order create/update/delete.

### Modified Capabilities
- (none — the project has no existing specs; the behaviors above are introduced by the two new capabilities.)

## Impact

- **Domain DTOs**: `OrderProductItem` and `OrderCreatedEventProduct` gain a side-dish selection field. Note: `order_event.go` builds the event product via a direct struct conversion (`OrderCreatedEventProduct(p)`), so the field must be added to both structs in lockstep or that conversion breaks.
- **Domain services**: `stock_event_handler` expansion honors per-line side-dish quantities on create, update-diff, and delete-restore; `product_ingredient_service` / `order_service` gain validation against `[min, max]` and the allowed side-dish set.
- **Persistence**: migrate `product_ingredient` — rename `quantity` → `default_quantity` (value-preserving) and add `is_side_dish`, `min_quantity`, `max_quantity`; store resolved side-dish selections per `open_bill_product` line. The rename touches every reader of the column (DTO field, repository, stock expansion, service, mocks, tests, docs).
- **API + docs**: product-config endpoints to manage side-dish options; order create/update payloads accept side-dish selections; update `docs/api/`.
- **Sync**: the side-dish definition is a cloud→edge pull concern (follows the product/ingredient pattern); order-line selections ride with open_bills. Detailed in design.md.
- **Kitchen ticket / print**: the cook currently reads free-text `Notes`; structured selections should render on the ticket ("sin ensalada, 3 canasta, 1 fries"). Confirm in/out of scope in design.md.
