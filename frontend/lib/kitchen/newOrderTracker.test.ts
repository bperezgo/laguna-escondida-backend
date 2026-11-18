import { test } from "node:test";
import assert from "node:assert/strict";
import { createNewOrderTracker } from "./newOrderTracker.ts";
import type { OrderType } from "@/types/commandItem";

const line = (open_bill_id: string, order_type: OrderType) => ({
  open_bill_id,
  order_type,
  temporal_identifier: `T-${open_bill_id}`,
});

test("a paid delivery order with three lines alerts exactly once", () => {
  const tracker = createNewOrderTracker();
  tracker.seed([]);
  const alerts = [
    tracker.observe(line("d1", "delivery")),
    tracker.observe(line("d1", "delivery")),
    tracker.observe(line("d1", "delivery")),
  ].filter(Boolean);
  assert.deepEqual(alerts, [
    { open_bill_id: "d1", order_type: "delivery", temporal_identifier: "T-d1" },
  ]);
});

test("a takeaway order alerts", () => {
  const tracker = createNewOrderTracker();
  assert.equal(tracker.observe(line("t1", "takeaway"))?.order_type, "takeaway");
});

test("a dine-in line never alerts", () => {
  const tracker = createNewOrderTracker();
  assert.equal(tracker.observe(line("m1", "dine_in")), null);
});

test("orders already on the board when it loads do not alert on the replay", () => {
  const tracker = createNewOrderTracker();
  tracker.seed([line("t1", "takeaway"), line("t2", "takeaway")]);
  assert.equal(tracker.observe(line("t1", "takeaway")), null);
  assert.equal(tracker.observe(line("t2", "takeaway")), null);
});

test("a line coming back to an order already shown does not alert", () => {
  const tracker = createNewOrderTracker();
  tracker.observe(line("t1", "takeaway"));
  assert.equal(tracker.observe(line("t1", "takeaway")), null);
});

test("after a reconnect, an order released while disconnected alerts once", () => {
  const tracker = createNewOrderTracker();
  tracker.seed([line("t1", "takeaway")]);
  // Replay on reconnect: the old order plus one paid during the drop.
  assert.equal(tracker.observe(line("t1", "takeaway")), null);
  assert.equal(tracker.observe(line("t2", "takeaway"))?.open_bill_id, "t2");
  assert.equal(tracker.observe(line("t2", "takeaway")), null);
});

test("after a reconnect with nothing missed, nothing alerts", () => {
  const tracker = createNewOrderTracker();
  tracker.seed([line("t1", "takeaway"), line("m1", "dine_in")]);
  assert.equal(tracker.observe(line("t1", "takeaway")), null);
  assert.equal(tracker.observe(line("m1", "dine_in")), null);
});
