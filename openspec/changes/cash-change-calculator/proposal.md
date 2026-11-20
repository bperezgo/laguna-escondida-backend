## Why

When a customer pays a bill in cash, the cashier works out the change in their head. With totals like $47.500 and a $100.000 bill, mistakes are easy at a busy till, and they only surface at the daily close as a cash difference.

## What Changes

- When **Efectivo** is the selected payment type, the payment screen shows a cash block with an optional "Recibido" amount field.
- Quick buttons set the received amount in one tap: **Exacto** (the total) plus up to 3 suggested round amounts based on Colombian bill sizes.
- If the received amount covers the total, the screen shows the **change to give back**, big and easy to read.
- If the received amount is short, the screen shows **how much is missing** and payment cannot be submitted.
- The payment confirmation message includes the change to give back.
- The calculator is a cashier aid only. Nothing is sent to or stored by the backend, and leaving "Recibido" empty keeps today's behavior.

## Capabilities

### New Capabilities
- `cash-payment-change`: the change calculator shown while closing a bill in cash, covering received amount, suggested amounts, change or missing amount, and the payment gate.

### Modified Capabilities
(none)

## Impact

- **Frontend only**: `frontend/components/orders/PaymentModal.tsx`, plus a small pure helper with unit tests under `frontend/lib/orders/`.
- **No API change**: `PayOrderRequest` and `backend/docs/api` are untouched. No backend, edge sync or migration work.
- Works the same offline on the edge box, since it is purely client-side.
- Out of scope: saving amount received or change on the bill, split payments (cash + card), and switching the modal's other totals from `toFixed(2)` to `formatMoney`.
