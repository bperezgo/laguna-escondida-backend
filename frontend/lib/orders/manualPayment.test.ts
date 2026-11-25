import { test } from "node:test";
import assert from "node:assert/strict";
import type { OpenBill } from "../../types/order.ts";
import { manualPaymentLabel, manualPaymentStatus, reviewActionLabel } from "./manualPayment.ts";

const bill: OpenBill = {
  id: "o1",
  temporal_identifier: "WA-1",
  total_amount: "45000",
  created_at: "2026-11-24T12:00:00Z",
  updated_at: "2026-11-24T12:00:00Z",
};

test("only a manual order awaits confirmation", () => {
  assert.equal(manualPaymentStatus(bill), null);
  assert.equal(manualPaymentStatus({ ...bill, payment_strategy: "wompi" }), null);
  assert.deepEqual(manualPaymentStatus({ ...bill, payment_strategy: "manual", payment_expires_at: "2026-11-24T13:00:00Z" }), {
    expiresAt: "2026-11-24T13:00:00Z",
    underReview: false,
  });
});

test("a review mark means the payment is being checked", () => {
  const status = manualPaymentStatus({
    ...bill,
    payment_strategy: "manual",
    payment_expires_at: "2026-11-24T13:00:00Z",
    payment_review_started_at: "2026-11-24T12:30:00Z",
  });
  assert.ok(status);
  assert.equal(status.underReview, true);
  assert.equal(manualPaymentLabel(status), "Pago en revisión");
  assert.equal(reviewActionLabel(status), "Dejar de revisar");
});

test("an order not yet under review offers to review it", () => {
  const status = manualPaymentStatus({ ...bill, payment_strategy: "manual" });
  assert.ok(status);
  assert.equal(manualPaymentLabel(status), "Esperando confirmación de pago");
  assert.equal(reviewActionLabel(status), "Revisar pago");
  assert.equal(status.expiresAt, null);
});
