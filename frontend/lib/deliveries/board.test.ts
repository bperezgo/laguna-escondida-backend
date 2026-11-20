import { test } from "node:test";
import assert from "node:assert/strict";
import type { DeliveryOrder } from "../../types/delivery.ts";
import { DeliveryApiError } from "../api/deliveries.ts";
import {
  actionsFor,
  deliveriesTabs,
  deliveryErrorMessage,
  dispatchRequest,
  failRequest,
  groupByStatus,
  timeInStatus,
} from "./board.ts";

function order(partial: Partial<DeliveryOrder>): DeliveryOrder {
  return {
    id: "o1",
    temporal_identifier: "019a0000-0000-7000-8000-0000000000d1",
    order_type: "delivery",
    delivery_status: "ready",
    status_since: "2026-11-20T17:00:00Z",
    paid: true,
    paid_at: "2026-11-20T16:40:00Z",
    created_at: "2026-11-20T16:30:00Z",
    lines: [],
    ...partial,
  };
}

const allow = (granted: string[]) => (p: string) => granted.includes(p);

test("Pedidos comes first, and only with deliveries:read", () => {
  assert.deepEqual(deliveriesTabs(allow(["deliveries:read", "ordering:read", "delivery-zones:read"])), [
    "pedidos", "estado", "horarios", "zonas",
  ]);
  assert.deepEqual(deliveriesTabs(allow(["ordering:read"])), ["estado", "horarios"]);
  assert.deepEqual(deliveriesTabs(allow(["deliveries:carry"])), [], "a courier sees no dashboard tab");
});

test("orders fall into their status column, longest waiting first", () => {
  const groups = groupByStatus([
    order({ id: "late", delivery_status: "ready", status_since: "2026-11-20T17:10:00Z" }),
    order({ id: "early", delivery_status: "ready", status_since: "2026-11-20T17:00:00Z" }),
    order({ id: "road", delivery_status: "out_for_delivery" }),
    order({ id: "done", delivery_status: "delivered" }),
    order({ id: "cooking", delivery_status: "preparing" }),
    order({ id: "bad", delivery_status: "failed" }),
  ]);
  assert.deepEqual(groups.ready.map((o) => o.id), ["early", "late"]);
  assert.deepEqual(groups.out_for_delivery.map((o) => o.id), ["road"]);
  assert.deepEqual(groups.delivered.map((o) => o.id), ["done"]);
  assert.deepEqual(groups.preparing.map((o) => o.id), ["cooking"]);
  assert.deepEqual(groups.failed.map((o) => o.id), ["bad"]);
});

test("time in status reads in minutes and hours", () => {
  const now = new Date("2026-11-20T18:05:00Z");
  assert.equal(timeInStatus("2026-11-20T18:04:40Z", now), "ahora");
  assert.equal(timeInStatus("2026-11-20T17:53:00Z", now), "12 min");
  assert.equal(timeInStatus("2026-11-20T17:05:00Z", now), "1 h");
  assert.equal(timeInStatus("2026-11-20T16:00:00Z", now), "2 h 5 min");
  assert.equal(timeInStatus("2026-11-18T16:00:00Z", now), "2 d");
});

test("actions follow the transition table", () => {
  assert.deepEqual(actionsFor(order({ delivery_status: "preparing" })), ["dispatch"]);
  assert.deepEqual(actionsFor(order({ delivery_status: "ready" })), ["dispatch"]);
  assert.deepEqual(actionsFor(order({ delivery_status: "out_for_delivery" })), ["deliver", "fail"]);
  assert.deepEqual(actionsFor(order({ delivery_status: "failed" })), ["redispatch"]);
  assert.deepEqual(actionsFor(order({ delivery_status: "delivered" })), []);
  assert.deepEqual(actionsFor(order({ order_type: "takeaway", delivery_status: "ready" })), ["pickup"]);
  assert.deepEqual(actionsFor(order({ order_type: "takeaway", delivery_status: "delivered" })), []);
});

test("the dispatch dialog needs a courier or a third-party name", () => {
  assert.deepEqual(dispatchRequest({ kind: "courier", courierUserId: "" }), { error: "Elige un repartidor." });
  assert.deepEqual(dispatchRequest({ kind: "courier", courierUserId: "u1" }), { body: { courier_user_id: "u1" } });
  assert.deepEqual(dispatchRequest({ kind: "third_party", courierName: "  " }), {
    error: "Escribe el nombre de quien lo lleva.",
  });
  assert.deepEqual(dispatchRequest({ kind: "third_party", courierName: " Mensajeros Ya " }), {
    body: { courier_name: "Mensajeros Ya" },
  });
  assert.ok("error" in dispatchRequest({ kind: "third_party", courierName: "x".repeat(81) }));
});

test("the fail dialog needs a reason and keeps the note short", () => {
  assert.deepEqual(failRequest("", "nota"), { error: "Elige el motivo." });
  assert.deepEqual(failRequest("refused", "  "), { body: { reason: "refused" } });
  assert.deepEqual(failRequest("wrong_address", " no existe "), { body: { reason: "wrong_address", note: "no existe" } });
  assert.ok("error" in failRequest("other", "n".repeat(501)));
});

test("a 409 names where the order is now; a lost claim reads as another courier's", () => {
  const conflict = new DeliveryApiError("x", 409, "INVALID_DELIVERY_TRANSITION", undefined, "out_for_delivery");
  assert.equal(deliveryErrorMessage(conflict, true), "Otro repartidor ya lo tomó.");
  assert.equal(deliveryErrorMessage(conflict), "El pedido cambió: ahora está en camino. Recarga la lista.");
  assert.equal(
    deliveryErrorMessage(new DeliveryApiError("x", 409, "INVALID_DELIVERY_TRANSITION", undefined, null)),
    "Este pedido ya no se puede mover.",
  );
  assert.equal(deliveryErrorMessage(new DeliveryApiError("x", 403)), "No puedes hacer este cambio en este pedido.");
  assert.equal(
    deliveryErrorMessage(new DeliveryApiError("x", 400, "VALIDATION_ERROR", "courier_user_id")),
    "Ese usuario no es un repartidor activo.",
  );
  assert.equal(deliveryErrorMessage(new Error("boom")), "No se pudo guardar el cambio. Intenta de nuevo.");
});
