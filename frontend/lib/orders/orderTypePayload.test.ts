import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildCreateOrderRequest,
  EMPTY_CONTACT,
  identifierPrefix,
  missingOrderFields,
  retypeIdentifier,
  type OrderContact,
} from "./orderTypePayload.ts";

const base = {
  open_bill_id: "ob-1",
  temporal_identifier: "MESA-12",
  descriptor: null,
  products: [{ open_bill_product_id: "obp-1", product_id: "p-1", quantity: 1, notes: null }],
};

const filled: OrderContact = {
  customerName: "  Ana Gómez ",
  customerPhone: " 300 555 1234 ",
  deliveryAddress: " Cra 7 # 12-34 ",
  pin: { lat: 4.59810049, lng: -74.0760004 },
};

test("the dine-in payload is identical to today's, even with contact fields filled", () => {
  const today = JSON.stringify(base);
  assert.equal(JSON.stringify(buildCreateOrderRequest(base, "dine_in", filled)), today);
});

test("a takeaway sends the type, phone and name, but no address or location", () => {
  assert.deepEqual(buildCreateOrderRequest(base, "takeaway", filled), {
    ...base,
    order_type: "takeaway",
    customer_phone: "300 555 1234",
    customer_name: "Ana Gómez",
  });
});

test("a takeaway without a name omits customer_name", () => {
  const request = buildCreateOrderRequest(base, "takeaway", { ...filled, customerName: " " });
  assert.equal("customer_name" in request, false);
});

test("a delivery sends address and the pin with 6 decimals", () => {
  assert.deepEqual(buildCreateOrderRequest(base, "delivery", filled), {
    ...base,
    order_type: "delivery",
    customer_phone: "300 555 1234",
    customer_name: "Ana Gómez",
    delivery_address: "Cra 7 # 12-34",
    delivery_lat: "4.598100",
    delivery_lng: "-74.076000",
  });
});

test("dine-in never misses a field", () => {
  assert.deepEqual(missingOrderFields("dine_in", EMPTY_CONTACT), []);
});

test("takeaway needs only the phone", () => {
  assert.deepEqual(missingOrderFields("takeaway", EMPTY_CONTACT), ["teléfono"]);
  assert.deepEqual(missingOrderFields("takeaway", { ...EMPTY_CONTACT, customerPhone: "300" }), []);
});

test("a delivery without a pin says the location is missing", () => {
  const contact = { ...filled, pin: null };
  assert.deepEqual(missingOrderFields("delivery", contact), ["ubicación en el mapa"]);
});

test("a blank delivery misses phone, address and location", () => {
  assert.deepEqual(missingOrderFields("delivery", EMPTY_CONTACT), [
    "teléfono",
    "dirección",
    "ubicación en el mapa",
  ]);
});

test("each type has its identifier prefix", () => {
  assert.equal(identifierPrefix("dine_in"), "MESA-");
  assert.equal(identifierPrefix("takeaway"), "LLEVAR-");
  assert.equal(identifierPrefix("delivery"), "DOM-");
});

test("an untouched prefix follows the type", () => {
  assert.equal(retypeIdentifier("MESA-", "delivery"), "DOM-");
  assert.equal(retypeIdentifier("DOM-", "takeaway"), "LLEVAR-");
  assert.equal(retypeIdentifier("", "dine_in"), "MESA-");
});

test("an identifier typed past the prefix is kept", () => {
  assert.equal(retypeIdentifier("MESA-12", "delivery"), "MESA-12");
  assert.equal(retypeIdentifier("Juan", "takeaway"), "Juan");
});
