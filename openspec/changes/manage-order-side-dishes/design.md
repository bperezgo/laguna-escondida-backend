## Context

See `proposal.md` — Why. The relevant existing machinery:

- Plates are `COMPOSITE` products. Their recipe lives in `product_ingredient` (composite → ingredient, with a `quantity`).
- Stock consumption is event-driven. `order_service` emits `OrderCreatedEvent` / `OrderUpdatedEvent` / `OrderDeletedEvent` on the event bus; `stock_event_handler` reacts and, via `expandComposite`, walks the `product_ingredient` recipe recursively and decrements stock only at leaf ingredients.
- The **same** `OrderCreatedEvent`/`OrderUpdatedEvent` is also consumed by `order_service.HandleOrderCreatedSSE` / `HandleOrderUpdatedSSE`, which enriches each line (prep area, product name, creator) into an `OpenBillProductSSE` and pushes it to the kitchen display for that area. One event, two consumers.
- Order lines (`OpenBillProductDetail`) already carry `Quantity` and free-text `Notes`. Side-dish intent lives in `Notes` today, invisible to stock.
- The event payload product (`OrderCreatedEventProduct`) is built from `OrderProductItem` by a **direct struct conversion** (`order_event.go:44`, `:163`) — the two structs must stay field-compatible.

## Goals / Non-Goals

**Goals:**
- Side-dish quantities on an order line drive stock consumption exactly.
- Configuration (which ingredients are side dishes, and their `default`/`min`/`max`) is data, editable per composite product — not code.
- Zero behavior change for lines that carry no side-dish selection.

**Non-Goals:**
- Cross-option rules (mutual exclusion, total caps) — deferred.
- Any price effect from side-dish changes.
- Changing how *fixed* ingredients are recorded or consumed.

## Decisions

### D1 — Definition: extend `product_ingredient`, don't add a parallel table
Add `is_side_dish bool`, `min_quantity int`, `max_quantity int` to `product_ingredient`, and **rename the existing `quantity` column to `default_quantity`** so the field's meaning is explicit for every ingredient. Fixed ingredients keep `is_side_dish = false`; their `default_quantity` is simply the exact amount always consumed.

- *Why*: side dishes **are** consumed ingredients; the stock engine already iterates this table. A separate table would fork the recipe and force the expansion to read two sources. The verbose name removes the "is this the default or a fixed amount?" ambiguity the plain `quantity` had once sides existed.
- *Alternative considered*: a companion `product_side_dish_option` table. Rejected — duplicates the composite→ingredient edge and its stock semantics.
- *Consequence*: an "offered alternative" (fries) is a row with `default_quantity = 0` (default off), `min_quantity = 0`, `max_quantity = N`. A `default_quantity = 0` fixed row never existed before; the expansion must treat `0` as "consume nothing", which it already does numerically.
- *Constraint*: `0 <= min_quantity <= default_quantity <= max_quantity` enforced at config write time.

### D2 — Order line stores resolved absolute quantities, not deltas
The line persists the concrete chosen quantity for each side dish (`{ingredient_product_id, quantity}`), the full resolved set, not just what changed.

- *Why*: it is an immutable record of what the customer actually got; both stock diffing and the kitchen ticket need concrete numbers; it decouples the historical line from later edits to the product's defaults.
- *Alternative considered*: store only deltas from default and re-resolve at read time. Rejected — re-resolution against a mutable definition makes past orders ambiguous.

### D3 — "Swap" is not a primitive
Swapping canasta → fries is `canasta: 2→0` plus `fries: 0→1`, each validated independently against its `[min, max]`. No swap entity, no substitution table.

- *Why*: the +/- UI already expresses this; a swap primitive would only be needed for the deferred XOR rules.

### D4 — Stock expansion honors the per-line selection
`expandComposite` splits ingredients: fixed rows expand at `default × line_qty` (unchanged); side-dish rows expand at `selected_qty × line_qty`, where `selected_qty` comes from the order line (falling back to `default` when the line specified nothing for that side dish).

- The selection must therefore travel on the event: `OrderCreatedEventProduct` (and the update/delete variants) gains the side-dish list. Because of the struct-conversion coupling (see Context), the field is added to `OrderProductItem` and `OrderCreatedEventProduct` together.
- `OrderUpdatedEvent` already diffs previous vs current products for stock; the diff must compare side-dish quantities too, not just line quantity, so editing sides adjusts stock. `OrderDeletedEvent` restores using the stored selection.

### D5 — Validation in the order service, at write time
`order_service` (create/update) loads the composite's side-dish definition and rejects a line whose selection references a non-side-dish ingredient or violates `[min, max]`. Config writes validate `0 <= min <= default <= max`.

### D6 — The preparation area sees the adjustments
The live kitchen feed (SSE) renders a line's resolved side dishes (e.g. "sin ensalada · 3 canasta · 1 fries") so the preparer keeps the information `Notes` used to carry. Kept minimal; deltas-vs-default highlighting is an optional UI nicety.

- *Revised during implementation*: the only printed document is the customer's "cuenta" (a billing receipt), not a kitchen ticket. Since side dishes have no price effect, they are noise on the cuenta, so the printed receipt intentionally does **not** carry them. The preparation area is served solely by the SSE feed. If a dedicated kitchen-ticket print path is added later, it should render side dishes at that point.

### D7 — Lean ID-based event; the kitchen enriches names
One `OrderCreatedEvent` (and its Updated/Deleted siblings) is consumed by **both** `stock_event_handler` and `order_service.HandleOrderCreatedSSE`. The side-dish selections ride on `OrderCreatedEventProduct.SideDishes` as `{ingredient_product_id, quantity}` — the **full resolved set**, mirroring the persisted line (D2). Because the event product is built from `OrderProductItem` by a direct struct conversion, the field is added to both structs together; `OrderUpdatedEvent` carries the same type in `PreviousProducts`/`CurrentProducts`, so update inherits it.

- **Stock** overlays the event's side-dish quantities onto the BOM: side-dish rows consume `event.SideDishes[ingredient].Quantity × lineQty`, fixed rows consume `default_quantity × lineQty`. Expansion stays in the handler — the event does **not** carry the fully expanded recipe.
- **Kitchen** keeps the event ID-based and resolves ingredient names in `HandleOrderCreatedSSE` (the seam that already resolves area/name/creator), placing a `[]{Name, Quantity}` list on `OpenBillProductSSE`. `HandleOrderUpdatedSSE` mirrors this so edited sides re-notify.
- *Why*: names are product attributes, not order data — carrying them on the bus bloats it and risks staleness; the SSE handler is the existing enrichment point.
- *Alternatives considered*: carry names in the event (rejected — staleness/bloat); carry the fully expanded recipe in the event (rejected — duplicates the recursive composite expansion that lives in the handler); send only overrides, not the full set (rejected — the event mirrors the persisted line per D2, and full sets make delete/restore self-contained).

## Risks / Trade-offs

- **Struct-conversion coupling** (`OrderCreatedEventProduct(p)`) silently breaks if fields drift → add the field to both structs in the same change; a compile check covers it.
- **Update-diff correctness**: if the diff ignores side-dish changes, editing sides on an existing order won't adjust stock → the diff key must include side-dish quantities, covered by service unit tests (edit a side dish up and down).
- **`default_quantity = 0` recipe rows**: introducing zero-default rows could confuse other readers of `product_ingredient` → audit readers; expansion already no-ops on 0.
- **Column rename blast radius**: renaming `quantity` → `default_quantity` touches every reader of `product_ingredient` (the DTO `Quantity` field, repository queries, the `stock_event_handler` expansion's `ingredient.Quantity`, `product_ingredient_service`, generated mocks, existing tests, and API docs), not just side-dish code → do the rename as one atomic sweep; `go build ./...` + `make generate-mocks` + `go test ./...` surface any straggler.
- **Sync ordering**: the definition is a cloud→edge pull entity (mirrors products/ingredients); order-line selections push edge→cloud with open_bills. A device must have the definition pulled before validating selections → definition follows the existing pull-reference pattern; selections carry their own quantities so consumption never depends on the cloud copy.
- **Backward compatibility**: legacy lines (no selection) must resolve to defaults → expansion falls back to `default_quantity` per side dish; a test asserts an unmodified plate consumes exactly today's amounts.
- **SSE name-resolution cost**: enriching side-dish names in `HandleOrderCreatedSSE` adds a per-line ingredient lookup → batch the ingredient-name fetch the way `GetProductPreparationResponsibilities` already batches, one query per order rather than per line.

## Migration Plan

1. Migration on `product_ingredient`: rename `quantity` → `default_quantity` (value-preserving), and add `is_side_dish` (default false), `min_quantity`, `max_quantity`. Every existing row stays a fixed ingredient — no behavior change.
2. New child storage for `open_bill_product` side-dish selections.
3. Ship definition endpoints + order payload support together so a device that sees the new order field also has the config to validate it.
4. Rollback: selections are additive; dropping the feature falls back to default-recipe consumption (today's behavior).
