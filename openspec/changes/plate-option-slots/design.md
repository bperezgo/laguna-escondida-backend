## Context

Side dishes today are `product_ingredients` rows with `is_side_dish`, `min_quantity` and `max_quantity` (migration 57). `OrderService.resolveSideDishSelections` checks each selection against its own row and returns the full `[]dto.SideDishSelection` for the line, which is stored in `open_bill_product_side_dishes` (migration 58), carried on the open-bill sync payload and order events, and used by `StockEventHandler` as a per-ingredient override when it expands the composite's recipe. Recipe rows replicate to the box as a pull entity. The customer MCP tools read `GET /api/products/:id/side-dishes` and send `side_dishes` quantities.

See proposal.md for the motivation and specs/ for the behavior.

## Goals / Non-Goals

**Goals:**
- Every rule lives in the server; the MCP tool and the POS only pick ids from a closed list.
- Reuse the existing resolved-selection pipeline (storage, sync, events, stock) instead of building a second one.
- Slots are the only way to offer a choice; per-ingredient side dishes are removed.

**Non-Goals:**
- Price changes per choice, choice availability (sold out, schedules), multi-select slots, staff override of the rules.
- Converting existing side-dish plates into slots automatically; they are set up again by hand.
- Dropping the side-dish columns and the line side-dish table, which old orders and older boxes still use.

## Decisions

### D1. Slots of bundled choices, single-select, default required

A slot is "exactly one choice; the default if omitted"; "without" is a configured choice with an empty bundle (Option A in exploration). A bundle (ingredient → qty) expresses the restaurant's exchange rates (2 canastas = 1 fries = 1 patacón) directly.

- *Alternative: a group with a points budget* (canasta = 1, fries = 2, budget = 2). Rejected: the agent would still send quantities, and the weights are hard for an admin to reason about.
- *Alternative: a per-slot "removable" flag.* Rejected: it splits "omitted" from "explicitly none" in the API and makes removal implicit; an explicit "Sin X" choice is visible on the ticket and only exists where configured.

### D2. Separate tables, not more columns on `product_ingredients`

New shared tables, organization-scoped with soft delete:

```
plate_option_slots         (id, organization_id, composite_product_id, name, position, ...)
plate_option_choices       (id, organization_id, slot_id, name, position, is_default, ...)
plate_option_choice_items  (id, organization_id, choice_id, ingredient_product_id, quantity, ...)
open_bill_product_choices  (id, organization_id, open_bill_product_id, slot_id, slot_name,
                            choice_id, choice_name, ...)
```

Bundle ingredients reference products directly and need not be recipe rows, so an admin does not configure fries twice. A partial unique index enforces one default per slot. Line choices snapshot slot and choice names, so renames and removals never rewrite history (spec: "A line records the choices it was taken with").

- *Alternative: require bundle ingredients to be `is_side_dish` recipe rows* and reuse the override map unchanged. Rejected: double configuration, and the old min/max on those rows would be meaningless and confusing.

### D3. Replace the whole slot list per plate

`PUT /api/products/:id/option-slots` takes the full ordered list and replaces it in one unit of work; `GET` returns it. Validating "exactly one default" and unique names on a whole document avoids invalid intermediate states that per-row CRUD would expose. Choices sent with an existing id keep it (so open orders and the agent's cached ids stay valid); omitted ones are soft-deleted.

### D4. Resolve choices into the existing `[]SideDishSelection`

Line resolution reads the plate's slots: if it has any, validate the line's `choices` (D1 rules), pick the default for each omitted slot, and sum the chosen bundles per ingredient into `[]SideDishSelection`. It also returns the resolved `[]LineChoice` for recording. Storage, the open-bill sync payload, `OrderCreatedEvent`/`OrderUpdatedEvent` and the update diff keyed by side dishes keep working unchanged, so editing a line's choices already returns the old bundle and consumes the new one.

### D5. Stock consumes selection entries that are not recipe rows

`expandComposite` applies the selection map only to the plate's direct recipe rows. For slot plates the map's ingredients are generally not recipe rows, so after walking the recipe, each unmatched map entry is consumed via `adjustStockForProduct(ingredient, qty × line qty)`. That expands composite bundle items too (a "canasta de patacón con hogao" with its own recipe). Legacy plates never have unmatched entries, so their behavior does not change. Because the box is the stock writer, the same code runs there with no extra work.

### D6. One pull entity per plate's slot list

The cloud replicates a plate's full slot list as one reference document keyed by composite product id with an `updated_at`, bumped on every PUT. The box replaces its local copy on apply. This propagates removals without per-row tombstones and matches D3. It follows the existing reference-entity recipe (DTO, port method, repository query, applier, pull result counter). An old box ignores the unknown field in the pull response; an updated box treats a missing field as "no changes".

### D7. Line choices travel on the open-bill sync line and kitchen DTOs

`OpenBillSyncProduct`, the order read DTOs and the kitchen SSE line gain an optional `choices` array. Old nodes omit it and the applier accepts that; the cloud then shows the side dishes but not the choice names for those lines.

### D8. MCP: slots first, choice ids only

The customer client reads `GET /api/products/:id/option-slots`. `get_side_dish_options` keeps its name (the WhatsApp agent calls it) and returns slots and choices without quantities. Order line input takes `choices: [choice_id]` and no side dishes. The tool description carries the "defaults unless the customer asks; anything else is a note" instruction. The full toolset gains get/set tools for slots, next to the product-ingredient tools, so the admin can configure plates through Claude.

### D9. Errors

Choice errors are `400`, with a message that names the slot or the unknown choice (the agent relays it). A line carrying `side_dishes` is `400` and says to send the plate's choices.

### D10. Slots replace side dishes

The side-dish endpoints, service methods, port method, DTO fields and MCP tools are removed. A migration (twin cloud and edge, since the shared set is frozen and recipe deletions do not travel in the pull) deletes recipe rows marked as side dishes: left in the recipe they would be consumed on every order on top of the chosen bundles. The columns and `open_bill_product_side_dishes` stay: chosen bundles are still stored as the line's resolved selections, old orders keep theirs, and the push applier accepts side dishes from boxes that predate the change. The stock handler is unchanged, so an order taken before the change and voided after it returns what it consumed.

- *Alternative: clear the flags and keep the rows as fixed ingredients.* Rejected: the admin sets those plates up again as slots, and the leftover rows would then be consumed twice.

## Risks / Trade-offs

- [A POS or agent build that still sends `side_dishes`] → 400. The old POS only sent them for plates with side-dish rows, which the migration removes, so in practice it sends none and its lines take the slot defaults.
- [Plates that had side dishes lose those parts until set up again] → their stock is under-consumed meanwhile. Accepted by the restaurant, which sets them up again as slots.
- [A choice is removed while an unpaid agent order still references it] → `update_my_order` resending the old lines fails. Mitigation: the error names the choice; the tool description tells the agent to re-read the options and ask the customer.
- [Swaps written only in notes] (POS without slots yet, or a request the config does not allow) → stock reflects the defaults, not what was served. This is accepted: notes are the human-confirmed exception path.
- [The agent writes a forbidden swap in notes] → the cook reads every line's notes and staff can review them, so a swap the configuration does not allow is confirmed by a person before it is served. No system change in scope.
- [A cross-organization ingredient id in a bundle] → caught by the same-organization lookup; covered by a service test.

## Migration Plan

1. Deploy the migrations and the cloud code; side-dish rows leave the recipes.
2. Deploy the box; its next pull brings any slot configuration (none at first).
3. Ship the POS slot UI and update the agent's runtime prompt for the new tool shapes.
4. Per plate that had side dishes: PUT its slots.

Rollback: revert the code. The down migrations drop the four tables; the deleted side-dish rows are not restored and would be configured again. Existing lines keep their stored side dishes, so stock and reads stay correct.
