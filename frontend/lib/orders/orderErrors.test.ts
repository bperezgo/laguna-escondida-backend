import { test } from "node:test";
import assert from "node:assert/strict";
import { orderErrorMessage } from "./orderErrors.ts";

const now = new Date("2026-11-01T15:00:00-05:00");

test("a paused delivery gives the pause reason", () => {
  assert.equal(
    orderErrorMessage({
      code: "ORDERING_PAUSED",
      order_type: "delivery",
      paused_reason: "Sin domiciliarios",
    }),
    'Los domicilios están pausados: "Sin domiciliarios"',
  );
});

test("a paused takeaway without a reason", () => {
  assert.equal(
    orderErrorMessage({ code: "ORDERING_PAUSED", order_type: "takeaway", paused_reason: null }),
    "Los pedidos para llevar están pausados.",
  );
});

test("outside hours gives the next opening", () => {
  assert.equal(
    orderErrorMessage(
      {
        code: "OUTSIDE_ORDERING_HOURS",
        order_type: "takeaway",
        next_opens_at: "2026-11-02T11:30:00-05:00",
      },
      now,
    ),
    "Los pedidos para llevar están fuera de horario. Abre mañana a las 11:30.",
  );
});

test("outside hours with no opening in the next week", () => {
  assert.equal(
    orderErrorMessage({ code: "OUTSIDE_ORDERING_HOURS", order_type: "delivery", next_opens_at: null }),
    "Los domicilios están fuera de horario y no abre en los próximos 7 días.",
  );
});

test("a pin outside every zone", () => {
  assert.equal(
    orderErrorMessage({ code: "OUTSIDE_DELIVERY_ZONE" }),
    "La ubicación está fuera de la zona de entrega.",
  );
});

test("an invalid phone", () => {
  assert.equal(orderErrorMessage({ code: "INVALID_PHONE" }), "El teléfono no es válido.");
});

test("a missing field names it when the backend does", () => {
  assert.equal(
    orderErrorMessage({
      code: "ORDER_MISSING_FIELD",
      error: "delivery_address is required for this order type",
    }),
    "Falta la dirección.",
  );
  assert.equal(
    orderErrorMessage({ code: "ORDER_MISSING_FIELD" }),
    "Falta un dato obligatorio del pedido.",
  );
});

test("any other error is left to the caller", () => {
  assert.equal(orderErrorMessage({ code: "SOMETHING_ELSE", error: "boom" }), null);
  assert.equal(orderErrorMessage({}), null);
});
