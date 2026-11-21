import { test } from "node:test";
import assert from "node:assert/strict";
import { calculateReceiptTotals } from "./receipt.ts";
import type { OpenBillWithProducts } from "@/types/order";

function order(deliveryFee?: string): OpenBillWithProducts {
  return {
    id: "o1",
    temporal_identifier: "D-1",
    total_amount: "37000",
    created_at: "",
    updated_at: "",
    order_type: "delivery",
    delivery_fee: deliveryFee,
    products: [
      {
        quantity: 1,
        product: { id: "p1", unit_price: "29629.63", vat_amount: "0", ico_amount: "2370.37" },
      },
    ],
  } as unknown as OpenBillWithProducts;
}

test("a delivery's fee is added to the receipt total", () => {
  const totals = calculateReceiptTotals(order("5000.0000"));
  assert.equal(totals.deliveryFee, 5000);
  assert.equal(Math.round(totals.total * 100) / 100, 37000);
});

test("an order without a fee totals its lines", () => {
  const totals = calculateReceiptTotals(order());
  assert.equal(totals.deliveryFee, 0);
  assert.equal(Math.round(totals.total * 100) / 100, 32000);
});
