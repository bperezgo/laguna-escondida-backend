## Context

`delivery-fee` (PR #52) works out a zone's base and ICO once, in `delivery_zone.parseFee`, using `shared.DeliveryFeeICORate`. It copies the three amounts (`dto.DeliveryFeeAmounts`) to the order (`open_bills`) and the bill (`bills`). `bill.DeliveryFeeItem` builds the invoice item from the stored base and ICO, but reads the rate from the constant. Products already keep their own `ico` rate as a fraction, and the product form edits it as a percent (`fractionToPercentage`).

## Goals / Non-Goals

**Goals:**
- No rate in code. Every figure on an order, bill or invoice comes from values stored when the order was created.

**Non-Goals:**
- An organization-wide tax setting. The user chose a per-zone rate, matching products.
- Recomputing existing orders or bills when a zone's rate changes.
- Touching `dto.TaxConfig`.

## Decisions

- **The rate is a fourth field on `dto.DeliveryFeeAmounts`** (`DeliveryFeeICORate`, json `delivery_fee_ico_rate`). The struct is already embedded in the zone DTO and copied whole into `Fulfillment.InZone`. Because of that, the order, the bill and every read get the rate through the existing path without new plumbing, and the snapshot stays atomic: the rate always travels with the amounts it produced. The alternative, a separate zone field passed alongside, would need a second argument at every hop.
- **The aggregate parses fee and rate together.** `parseFee(rawFee, rawRate)` returns the four values. On update, a missing fee or rate keeps the stored one, and the base and ICO are recomputed whenever either one changes. Validation: decimal, `0 ≤ rate < 1`, at most 4 decimals. A new error code, `DELIVERY_ZONE_INVALID_ICO_RATE`, mirrors the fee's. `rate < 1` rejects `8`, the easy mistake of sending a percent.
- **The rate is a fraction on the API and a percent in the UI**, like products. The zone editor reuses the product form's percent↔fraction conversion (moved to a shared helper if it is component-local).
- **`DeliveryFeeItem` uses `*b.DeliveryFeeICORate`.** If a bill has a fee but no rate, it gets no item, the same as a missing base or ICO today. The migration backfill means that should not happen.
- **Columns:** `delivery_fee_ico_rate NUMERIC(5,4)`.
  - On `delivery_zones` it is `NOT NULL DEFAULT 0.08` with a `CHECK (>= 0 AND < 1)`.
  - On `open_bills` and `bills` it is nullable, backfilled with `0.08` where `delivery_fee IS NOT NULL`.
  - Cloud `000092`, edge `000082` (orders and bills only, matching `000081`).
- **MCP zone admin tools accept `delivery_fee_ico_rate`.** The customer tools don't expose it: the agent quotes the final fee, not its tax breakdown.

## Risks / Trade-offs

- [A tax change means re-saving every zone] → There are a handful of zones, and the editor shows the rate on each card. This is the trade-off the user accepted over a single setting.
- [Someone enters `8` instead of `0.08` through the API] → `rate < 1` rejects it. The UI always sends a fraction.
- [Archive order] → The deltas modify requirements that `delivery-fee` adds, so `delivery-fee` must be archived first.

## Migration Plan

The migrations are additive and run at boot. The backfill gives existing zones and orders the 0.08 their amounts were computed with, so no figure changes. Down: `DROP COLUMN IF EXISTS`. That is lossless only while every rate is 0.08.
