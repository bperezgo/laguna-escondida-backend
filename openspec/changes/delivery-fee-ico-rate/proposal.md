## Why

The delivery fee's ICO rate is code, not data: `shared.DeliveryFeeICORate = 0.08` in the backend and "8%" in the zone editor's help text. Tax rates change by law, and a change today means a deploy. Worse, the invoice's "Domicilio" item reads the rate from that constant at submission time, so after a rate change a bill paid earlier would be invoiced with the new rate next to its old ICO amount.

## What Changes

- **A delivery zone has its own ICO rate**, like a product has its own `ico`. It is a fraction (`0.08` = 8%), `0 ≤ rate < 1`, defaulting to `0.08` when not given. Admins and managers set it with the zone, entered as a percent in the zone editor. The zone's base and ICO are derived from its fee at its rate.
- **Changing a zone's rate recomputes that zone's base and ICO** and affects only orders created afterwards, exactly like changing its fee.
- **A delivery order records the rate with the fee.** The order and its bill store `delivery_fee_ico_rate` next to the three amounts, and reads return it.
- **The invoice's "Domicilio" item uses the bill's recorded rate**, never a constant.
- **The constant is removed**, and the zone editor shows the zone's rate instead of the fixed "8%".
- Existing zones and orders get `0.08`, the rate their stored amounts were derived with, so no figure changes.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `delivery-zones`: the zone fee requirement now takes the zone's ICO rate instead of a fixed 8%.
- `delivery-fee-invoicing`: the "Domicilio" item is taxed at the rate the bill recorded.

Both requirements are added by the `delivery-fee` change, which is merged but not archived yet. **Archive `delivery-fee` before this change**, so these deltas apply on top of it.

## Impact

- Backend:
  - Migrations: cloud `000092` adds the rate to `delivery_zones` (`NOT NULL DEFAULT 0.08`), `open_bills` and `bills`; edge `000082` adds it to `open_bills` and `bills`.
  - Code: the zone aggregate, DTOs and repository; the order fulfillment snapshot; the bill and `DeliveryFeeItem`; the MCP zone admin tools.
- Frontend: the zone editor gets a rate field; the zone types.
- Docs: `backend/docs/api/delivery-zones.md`, `orders.md`, `invoices.md`.
- Not affected: product taxes, `dto.TaxConfig` (its `ICOPercent` is read nowhere and stays as is), the reports (they sum stored amounts), dine-in and takeaway.
