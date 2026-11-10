# Frontend handoff: plate option slots

For the POS (and the cloud board). The full contract is in `docs/api/products.md#plate-option-slots` and `docs/api/orders.md#plate-choices`.

## What changes for the POS

Option slots replace per-ingredient side dishes. The side-dish endpoints (`GET /api/products/:id/side-dishes`, `PUT`/`DELETE /api/products/:id/ingredients/:id/side-dish`) are gone, and a migration removes every side-dish row from the recipes; those plates are set up again as slots.

1. When a server adds a composite plate to a line, call `GET /api/products/:id/option-slots` (works on the box and in the cloud).
   - `slots` is non-empty → render the slot UI below.
   - `slots` is empty → the plate has nothing to choose.
2. Render each slot as a **single-choice group** (radio buttons or segmented control), in `position` order, with the `is_default` choice preselected. Every slot always has exactly one choice selected; there is no "none" unless the plate has a choice for it (e.g. "Sin ensalada").
3. On the line, send `"choices": ["<choice id>", ...]`, at most one id per slot; a slot left out takes its default. **Never send `side_dishes`**: any line carrying them gets `400`.
4. On update (`PUT /api/orders/:id`), resend each line's `choices` (read them from the line's `choices[].choice_id`). A line sent without `choices` goes back to the defaults.
5. Anything the slots do not offer (a swap the restaurant does not sell at that price) goes in the line's `notes`, as today. The backend never accepts it as a choice.
6. Choices never change the price. An upcharge is a separate product on its own line.

## What to show

- Order reads (`GET /api/orders/:id`, the active and closed lists, closed order detail) return `products[].choices`: `[{slot_id, slot_name, choice_id, choice_name}]`, one per slot, with the names the line was taken with. Show them on the ticket as `Slot: Choice` (e.g. `Acompañante: Papas`). They do not change if the menu is edited later.
- The kitchen feed (`/api/open-bill-products/:area/pending` and `/completed`, and the `open_bill_product.created` / `.updated` SSE events) carries the same `choices` array. `side_dishes` is still there, holding the summed ingredients of the chosen bundles (and the side dishes of orders taken before slots); show the choices when present, `side_dishes` otherwise.
- A changed choice re-sends `open_bill_product.updated`.

## Errors to surface

`400` with `error` text naming the problem, e.g. `invalid plate choice: the "Arroz" slot of Tilapia takes one choice`, or `... choice <id> is not an option of Tilapia` (the choice was removed from the menu since the line was taken: reload the slots and ask the server to pick again). A client that still sends side dishes gets `side dishes are no longer offered; send the plate's choices`.

## Rollout

A POS without slot support keeps working: it finds no side dishes (the endpoint is gone, so it sends none) and every slot plate's line is taken with the slot defaults. It cannot pick another choice until the POS release that renders slots is on the box; meanwhile non-default requests go in `notes`, and the kitchen does not deduct them.

Admin screens: the slot editor is a whole-list form. `GET` the list, edit it, `PUT` it back with the existing `id`s so open orders and the agent's cached ids stay valid. One default per slot, unique names, whole quantities ≥ 1. Keep the choosable parts out of the plate's recipe: the recipe is always consumed, and the chosen bundles are consumed on top.
