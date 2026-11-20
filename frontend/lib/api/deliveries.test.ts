import { test } from "node:test";
import assert from "node:assert/strict";
import { DeliveryApiError, deliveriesApi } from "./deliveries.ts";

function stubFetch(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  globalThis.fetch = (async (url: string, init?: RequestInit) => {
    calls.push({ url, init });
    return new Response(JSON.stringify(body), { status });
  }) as typeof fetch;
  return calls;
}

test("the dashboard list forwards only the filters given", async () => {
  const calls = stubFetch(200, { orders: [] });

  await deliveriesApi.list();
  await deliveriesApi.list({ status: "out_for_delivery", order_type: "delivery" });

  assert.equal(calls[0].url, "/api/deliveries/orders");
  assert.equal(calls[1].url, "/api/deliveries/orders?status=out_for_delivery&order_type=delivery");
});

test("a courier claims with no body; a manager names the courier", async () => {
  const calls = stubFetch(200, { id: "o1" });

  await deliveriesApi.dispatch("o1");
  await deliveriesApi.dispatch("o1", { courier_name: "Mensajeros Ya" });

  assert.equal(calls[0].url, "/api/deliveries/orders/o1/dispatch");
  assert.equal(calls[0].init?.method, "POST");
  assert.equal(calls[0].init?.body, undefined);
  assert.equal(calls[1].init?.body, JSON.stringify({ courier_name: "Mensajeros Ya" }));
});

test("fail sends the reason and the note", async () => {
  const calls = stubFetch(200, { id: "o1" });

  await deliveriesApi.fail("o1", { reason: "wrong_address", note: "no existe" });

  assert.equal(calls[0].url, "/api/deliveries/orders/o1/fail");
  assert.equal(calls[0].init?.body, JSON.stringify({ reason: "wrong_address", note: "no existe" }));
});

test("a lost claim keeps the code and the order's current status", async () => {
  stubFetch(409, {
    error: "the order is out_for_delivery and cannot dispatch",
    code: "INVALID_DELIVERY_TRANSITION",
    current_status: "out_for_delivery",
  });

  await assert.rejects(deliveriesApi.dispatch("o1"), (err: unknown) => {
    assert.ok(err instanceof DeliveryApiError);
    assert.equal(err.status, 409);
    assert.equal(err.code, "INVALID_DELIVERY_TRANSITION");
    assert.equal(err.currentStatus, "out_for_delivery");
    return true;
  });
});

test("a validation error keeps its field", async () => {
  stubFetch(400, { error: "invalid reason", code: "VALIDATION_ERROR", field: "reason" });

  await assert.rejects(deliveriesApi.fail("o1", { reason: "other" }), (err: unknown) => {
    assert.ok(err instanceof DeliveryApiError);
    assert.equal(err.field, "reason");
    return true;
  });
});

test("the courier queue and the history read their own paths", async () => {
  const calls = stubFetch(200, {});

  await deliveriesApi.mine();
  await deliveriesApi.history("o 1");
  await deliveriesApi.couriers();

  assert.deepEqual(calls.map((c) => c.url), [
    "/api/deliveries/mine",
    "/api/deliveries/orders/o%201/history",
    "/api/deliveries/couriers",
  ]);
});
