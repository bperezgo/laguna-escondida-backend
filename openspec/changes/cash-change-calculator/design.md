## Context

See proposal.md for the motivation. `PaymentModal` already computes the bill `total` through `calculateReceiptTotals(openBill)` and holds `paymentType` in local state. Payment goes through `PinConfirmModal` (edge) or directly (cloud) to `payOrder`. Frontend tests run with `node --test` on `*.test.ts` files, which suits pure helpers in `lib/` but not React components.

## Goals / Non-Goals

**Goals:**
- Keep all arithmetic and suggestion logic in a pure, unit-tested helper. The component only wires state to it.
- Work the same on the waitress phone (<768px) and on the desktop/edge box.

**Non-Goals:**
- Component-level tests (the repo has no React test setup).
- Reworking the existing totals formatting in the modal.

## Decisions

**1. Pure helper `lib/orders/cashChange.ts`.**
It exports:
- `suggestCashAmounts(total): number[]`, returning up to 3 amounts.
- `computeCashChange(total, received): { kind: "empty" } | { kind: "change", amount } | { kind: "short", missing }`.

This keeps the spec scenarios testable as plain unit tests, which is the alternative to putting the logic inline in the component, where it would be untested.

**2. Suggestion rule.**
- For each bill size in `[5000, 10000, 20000, 50000, 100000]`, compute `ceil(total / b) * b`.
- If the result equals the total, bump it to the next multiple, `+ b`.
- Deduplicate, sort ascending, and keep the first 3.

The 2.000 bill is left out on purpose. With it, a 47.500 total suggests 48.000/50.000/60.000 and pushes out 100.000, which is the bill customers actually hand over most often. "Exacto" covers paying the exact amount.

**3. Whole currency units.**
- The total is rounded with `Math.round` before computing, which guards against float noise from the per-line `parseFloat` math in `calculateReceiptTotals`.
- The received input is parsed as digits only. Thousands separators like `100.000` are stripped, so typing `100.000` or `100000` both work.
- Display uses `formatMoney` (es-CO, 0 decimals).

**4. Gate in `requestPayment`.**
- When `paymentType === "cash"` and the result is `short`, the pay button is disabled.
- `requestPayment` also refuses with an error, as defense in depth, matching the existing offline-card pattern.
- An `empty` result never blocks payment.

**5. State lifetime.**
- `received` is component state.
- An effect clears it whenever `paymentType` changes away from cash.
- Closing the modal unmounts it, so nothing persists.

**6. Confirmation message.**
When the result is `change`, the `PinConfirmModal` message appends `Cambio: <formatMoney(amount)>`.

## Risks / Trade-offs

- [The displayed total uses `toFixed(2)`, but the calculator rounds to whole pesos] → For COP bills with no cents these agree. If a bill ever has cents, the change could differ by under $1, which is acceptable for a cashier aid.
- [The numeric input on a phone may show a keyboard without a "." key] → Use `inputMode="numeric"` and strip non-digits. The cashier types `100000`.

## Implementation map

- `frontend/lib/orders/cashChange.ts` (new)
- `frontend/lib/orders/cashChange.test.ts` (new)
- `frontend/components/orders/PaymentModal.tsx`
- `frontend/lib/format.ts` (read-only, for `formatMoney`)
- `frontend/components/ui` (read-only, for `Input` / `Button`)
