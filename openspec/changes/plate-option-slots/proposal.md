## Why

A plate's side dishes are configured one by one, each with its own `[min, max]`, so the system cannot say "white *or* coconut rice", "fries *instead of* the two canastas" or "a canasta only *in place of* the salad". A line with every side at its maximum is accepted. Staff catch this on the floor; the WhatsApp agent cannot, and a customer can talk it into an order the restaurant would never serve at that price.

## What Changes

- A composite plate can have **option slots**: ordered, named groups of **choices**, exactly one of them the default. A choice is a **bundle** of ingredient quantities (e.g. "Canastas" = 2 canasta de patacón). An empty bundle means "without" (e.g. "Sin ensalada") and exists only where configured.
- An order line for such a plate carries the chosen **choice ids**. Every slot ends with exactly one choice; an omitted slot takes its default. A choice not on the plate or two choices for one slot are rejected and nothing is created or changed. The rule is the same for staff and agent; exceptions stay in the line's notes.
- The server expands choices into ingredient quantities, so stock consumption, order sync and stored side dishes work on that result. The line also records its choices, so reads and the kitchen show "Arroz de coco".
- Choices never change the price; upcharges stay separate products.
- **BREAKING**: option slots **replace** per-ingredient side dishes. The side-dish endpoints and MCP tools are removed, a line sending `side_dishes` is rejected, and a migration deletes the side-dish recipe rows so those plates are set up again as slots. Old orders keep their recorded side dishes, and pushes from boxes not yet updated are still accepted.
- Slot configuration is written in the cloud only and reaches the edge box through the pull.
- **BREAKING (MCP)**: `get_side_dish_options` keeps its name but returns only slots and choices; the order tools take choice ids and no `side_dishes`; `configure_side_dish` and `clear_side_dish` are removed.

## Capabilities

### New Capabilities

- `plate-option-slots`: configuring slots and choices on a composite plate, and resolving, validating and recording an order line's choices.

### Modified Capabilities

- `customer-agent-access`: the menu and side-dish tools expose slots; the order tools take choice ids and keep the agent to the defaults unless the customer asks.
- `edge-schema`: slot configuration joins the cloud-only catalog writes the box serves read-only.

## Impact

- **Schema**: new tables for slots, choices, choice items and a line's chosen choices; a migration deletes side-dish recipe rows (the columns stay).
- **Domain**: DTOs, ports, a configuration service; `OrderService` resolves slots only and refuses side-dish quantities; the side-dish service methods go; the stock handler consumes bundle ingredients that are not recipe rows.
- **Sync**: a new cloud→edge pull entity; open-bill sync lines gain their choices (optional field).
- **HTTP**: cloud-only endpoints under `/api/products/:id/option-slots`; order lines accept `choices`; API docs and curl examples updated.
- **MCP / agent prompt**: customer tools change shape (see BREAKING).
- **POS frontend**: renders slots as single-choice groups; the side-dish steppers and editor go.
