import { test } from "node:test";
import assert from "node:assert/strict";
import {
  deliverableZone,
  deliveryFeeLabel,
  feeAmount,
  isFreeDelivery,
  parseDeliveryFeeInput,
} from "./fee.ts";

test("a fee typed with or without thousands separators", () => {
  assert.deepEqual(parseDeliveryFeeInput("5000"), { ok: true, value: "5000" });
  assert.deepEqual(parseDeliveryFeeInput("5.000"), { ok: true, value: "5000" });
  assert.deepEqual(parseDeliveryFeeInput(" 12.500 "), { ok: true, value: "12500" });
  assert.deepEqual(parseDeliveryFeeInput("4629,63"), { ok: true, value: "4629.63" });
});

test("an empty fee is free delivery", () => {
  assert.deepEqual(parseDeliveryFeeInput(""), { ok: true, value: "0" });
  assert.deepEqual(parseDeliveryFeeInput("0"), { ok: true, value: "0" });
});

test("negative, non-numeric or over-precise fees are refused", () => {
  for (const text of ["-1000", "abc", "5000.123", "5,000.5"]) {
    assert.equal(parseDeliveryFeeInput(text).ok, false, text);
  }
});

test("the API's decimal strings read as amounts", () => {
  assert.equal(feeAmount("5000.0000"), 5000);
  assert.equal(feeAmount(undefined), 0);
  assert.equal(isFreeDelivery("0"), true);
  assert.equal(isFreeDelivery("5000"), false);
  assert.equal(isFreeDelivery(undefined), true);
});

test("a deliverable check carries the zone's fee", () => {
  assert.deepEqual(deliverableZone({ deliverable: true, zone_name: "Centro", delivery_fee: "5000" }), {
    zoneName: "Centro",
    deliveryFee: 5000,
  });
  assert.deepEqual(deliverableZone({ deliverable: true, zone_name: "Norte", delivery_fee: "0" }), {
    zoneName: "Norte",
    deliveryFee: 0,
  });
  assert.equal(deliverableZone({ deliverable: false }), null);
});

test("the fee label says free when it is 0", () => {
  const format = (n: number) => `$${n}`;
  assert.equal(deliveryFeeLabel(5000, format), "Domicilio $5000");
  assert.equal(deliveryFeeLabel("5000.0000", format), "Domicilio $5000");
  assert.equal(deliveryFeeLabel(0, format), "Domicilio gratis");
});
