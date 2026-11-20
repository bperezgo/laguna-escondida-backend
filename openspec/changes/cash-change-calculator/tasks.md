## 1. Cash change helper

- [x] 1.1 Create `frontend/lib/orders/cashChange.test.ts` covering the spec scenarios: suggestions for 47.500 → [50.000, 60.000, 100.000], 12.300 → [15.000, 20.000, 50.000] and 50.000 → [55.000, 60.000, 100.000]; change for 100.000 on 47.500 = 52.500; exact amount = 0; short 45.000 on 47.500 → missing 2.500; empty received → empty; received input with thousands separators parses correctly. Verify with `pnpm test` from `frontend/` that it fails (red).
- [x] 1.2 Implement `frontend/lib/orders/cashChange.ts` (`suggestCashAmounts`, `computeCashChange`, received-input parser) per design.md decisions 1–3. Verify that `pnpm test` from `frontend/` passes.

## 2. PaymentModal integration

- [ ] 2.1 Add the cash block to `frontend/components/orders/PaymentModal.tsx`, shown only when `paymentType === "cash"`: a "Recibido" input (`inputMode="numeric"`), an "Exacto" button plus suggestion buttons, and a prominent change result or a red "Faltan $X" warning, using `formatMoney`. Verify manually: select Efectivo on a $47.500 bill, tap 100.000, and see "Cambio $52.500".
- [ ] 2.2 Clear the received amount when the payment type changes away from cash. Verify manually: enter an amount, switch to a transfer type, switch back, and the field is empty.
- [ ] 2.3 Disable "Pagar y Cerrar Cuenta" and refuse in `requestPayment` when cash is short. Leave payment allowed when received is empty. Verify manually: 45.000 on 47.500 disables the button, and an empty field still allows paying.
- [ ] 2.4 Append "Cambio: $X" to the `PinConfirmModal` message when change applies. Verify manually on edge mode (PIN required) that the confirmation shows the change.
- [x] 2.5 Confirm the `payOrder` payload is unchanged (no new fields in `PayOrderRequest`). Verify by reviewing the diff of `types/billOwner.ts`, which should show no change.

## 3. Checks

- [ ] 3.1 Run `pnpm lint` and `pnpm test` from `frontend/` and verify both pass. Check the cash block at phone width (<768px) and verify there is no horizontal overflow.
