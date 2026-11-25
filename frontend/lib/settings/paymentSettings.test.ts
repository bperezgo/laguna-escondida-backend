import { test } from "node:test";
import assert from "node:assert/strict";
import type { PaymentSettings } from "../../types/paymentSettings.ts";
import {
  draftFrom,
  hasPaymentErrors,
  isPaymentDraftChanged,
  paymentRequest,
  paymentSaveErrors,
  paymentSettingsCardState,
  validatePaymentDraft,
  wompiMissingNotice,
} from "./paymentSettings.ts";

const manual: PaymentSettings = {
  strategy: "manual",
  manual_instructions: "Llave Bre-B 3001234567",
  manual_window_minutes: 60,
};

test("loaded settings show the card", () => {
  assert.deepEqual(paymentSettingsCardState({ status: 200, settings: manual }), { kind: "loaded", settings: manual });
});

test("load failures are errors", () => {
  for (const status of [403, 404, 500]) {
    assert.equal(paymentSettingsCardState({ status }).kind, "error");
  }
});

test("manual needs instructions", () => {
  const errors = validatePaymentDraft({ ...draftFrom(manual), manual_instructions: "   " });
  assert.match(errors.manual_instructions ?? "", /transferencia/);
  assert.equal(hasPaymentErrors(errors), true);
});

test("wompi does not need instructions", () => {
  const errors = validatePaymentDraft({ ...draftFrom(manual), strategy: "wompi", manual_instructions: "" });
  assert.equal(hasPaymentErrors(errors), false);
});

test("the window is a whole number from 10 to 1440", () => {
  for (const bad of ["9", "1441", "12.5", "", "abc"]) {
    assert.ok(validatePaymentDraft({ ...draftFrom(manual), manual_window_minutes: bad }).manual_window_minutes, bad);
  }
  for (const good of ["10", "60", "1440"]) {
    assert.equal(validatePaymentDraft({ ...draftFrom(manual), manual_window_minutes: good }).manual_window_minutes, undefined, good);
  }
});

test("instructions are limited to 1000 characters", () => {
  const errors = validatePaymentDraft({ ...draftFrom(manual), manual_instructions: "a".repeat(1001) });
  assert.match(errors.manual_instructions ?? "", /1000/);
});

test("the request trims the instructions and sends the window as a number", () => {
  assert.deepEqual(paymentRequest({ strategy: "manual", manual_instructions: "  Llave  ", manual_window_minutes: "45" }), {
    strategy: "manual",
    manual_instructions: "Llave",
    manual_window_minutes: 45,
  });
});

test("an untouched draft is not a change", () => {
  assert.equal(isPaymentDraftChanged(draftFrom(manual), manual), false);
  assert.equal(isPaymentDraftChanged({ ...draftFrom(manual), strategy: "wompi" }, manual), true);
});

test("a refused save is shown next to the field the backend names", () => {
  assert.ok(paymentSaveErrors(400, "x", "manual_instructions").manual_instructions);
  assert.ok(paymentSaveErrors(400, "x", "manual_window_minutes").manual_window_minutes);
  assert.deepEqual(Object.keys(paymentSaveErrors(403, "x")), ["form"]);
  assert.deepEqual(paymentSaveErrors(500, "boom"), { form: "boom" });
});

test("wompi without an integration warns that orders are refused", () => {
  assert.match(wompiMissingNotice("wompi", false) ?? "", /se rechazan/);
  assert.equal(wompiMissingNotice("wompi", true), null);
  assert.equal(wompiMissingNotice("manual", false), null);
});
