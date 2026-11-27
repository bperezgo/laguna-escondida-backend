import { test } from "node:test";
import assert from "node:assert/strict";
import { cardBrand, cardLabel, formatCardNumber, formatExpiry, luhnValid, parseExpiry, validateCard } from "./card.ts";

const november2026 = new Date(2026, 10, 26);
const good = { number: "4242 4242 4242 4242", expiry: "12/29", cvc: "123", holder: "Rosa Pérez" };

test("the number is grouped in fours as it is typed", () => {
  assert.equal(formatCardNumber("42424242"), "4242 4242");
  assert.equal(formatCardNumber("4242424242424242"), "4242 4242 4242 4242");
  assert.equal(formatCardNumber("4242-4242 42x"), "4242 4242 42");
  assert.equal(formatCardNumber("378282246310005"), "3782 822463 10005");
});

test("the brand is told from the prefix", () => {
  assert.equal(cardBrand("4"), "visa");
  assert.equal(cardBrand("4242 4242"), "visa");
  assert.equal(cardBrand("5"), null);
  assert.equal(cardBrand("55"), "mastercard");
  assert.equal(cardBrand("5105105105105100"), "mastercard");
  assert.equal(cardBrand("2223"), "mastercard");
  assert.equal(cardBrand("37"), "amex");
  assert.equal(cardBrand("6011"), null);
  assert.equal(cardBrand(""), null);
});

test("a wrong check digit is refused", () => {
  assert.ok(luhnValid("4242424242424242"));
  assert.ok(luhnValid("4111 1111 1111 1111"));
  assert.ok(!luhnValid("4242424242424241"));
  assert.ok(!luhnValid("4242"));
  assert.equal(validateCard({ ...good, number: "4242 4242 4242 4241" }, november2026).number, "Revisa el número de la tarjeta.");
});

test("the expiry is MM/AA and not in the past", () => {
  assert.equal(formatExpiry("1229"), "12/29");
  assert.equal(formatExpiry("1"), "1");
  assert.deepEqual(parseExpiry("12/29"), { month: "12", year: "29" });
  assert.equal(parseExpiry("13/29"), null);
  assert.equal(parseExpiry("1229"), null);
  assert.equal(validateCard({ ...good, expiry: "12/25" }, november2026).expiry, "La tarjeta está vencida.");
  assert.equal(validateCard({ ...good, expiry: "10/26" }, november2026).expiry, "La tarjeta está vencida.", "a past month of this year");
  assert.equal(validateCard({ ...good, expiry: "11/26" }, november2026).expiry, undefined, "good through the end of its month");
  assert.equal(validateCard({ ...good, expiry: "13/29" }, november2026).expiry, "Usa el formato MM/AA.");
});

test("a complete card has no errors and an empty one names every field", () => {
  assert.deepEqual(validateCard(good, november2026), {});
  assert.deepEqual(Object.keys(validateCard({ number: "", expiry: "", cvc: "", holder: "" }, november2026)).sort(), ["cvc", "expiry", "holder", "number"]);
  assert.ok(validateCard({ ...good, cvc: "12" }, november2026).cvc);
  assert.equal(validateCard({ ...good, number: "378282246310005", cvc: "1234" }, november2026).cvc, undefined);
});

test("a saved card reads as brand and last four", () => {
  assert.equal(cardLabel("VISA", "4242"), "Visa terminada en 4242");
  assert.equal(cardLabel("mastercard", "1234"), "Mastercard terminada en 1234");
  assert.equal(cardLabel(null, "4242"), "Tarjeta terminada en 4242");
  assert.equal(cardLabel("visa", null), null);
});
