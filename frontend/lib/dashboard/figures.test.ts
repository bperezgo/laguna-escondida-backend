import { test } from "node:test";
import assert from "node:assert/strict";
import type { DailyCloseReport } from "../../types/dailyClose.ts";
import type { FinancialSummary } from "../../types/financialSummary.ts";
import { dayFigures } from "./figures.ts";

const close = (over: Partial<DailyCloseReport> = {}): DailyCloseReport => ({
  start_date: "2026-11-25T05:00:00Z",
  end_date: "2026-11-26T04:59:59Z",
  total_orders: 12,
  total_collected: "1920000",
  total_net: "1700000",
  total_vat: "150000",
  total_ico: "70000",
  total_discount: "0",
  total_tip: "0",
  total_delivery_fee: "0",
  total_delivery_fee_base: "0",
  total_delivery_fee_ico: "0",
  by_payment_method: [
    { payment_method: "cash", collected: "1240000", net: "1100000", count: 7 },
    { payment_method: "credit_card", collected: "680000", net: "600000", count: 5 },
  ],
  ...over,
});

const summary = (expenses: string, purchases: string): FinancialSummary => ({
  start_date: "2026-11-25T05:00:00Z",
  end_date: "2026-11-26T04:59:59Z",
  revenue: {
    total_amount: "0",
    total_vat: "0",
    total_ico: "0",
    total_discount: "0",
    total_tip: "0",
    count: 0,
    total_delivery_fee: "0",
    total_delivery_fee_base: "0",
    total_delivery_fee_ico: "0",
  },
  expenses: { total_amount: expenses, by_category: [], count: 0 },
  purchases: { total_amount: purchases, count: 0 },
  net_income: "0",
});

const EMPTY_DAY = { total_orders: 0, total_collected: "0", by_payment_method: [] };

test("a day with sales gives the collected amount, the orders and the average ticket", () => {
  const figures = dayFigures(close(), summary("150000", "300000"));
  assert.equal(figures.isEmpty, false);
  assert.equal(figures.collected, "1920000");
  assert.equal(figures.orders, 12);
  assert.equal(figures.averageTicket, 160000);
  assert.deepEqual(
    figures.paymentMethods.map((m) => [m.label, m.collected, m.count]),
    [
      ["Efectivo", "1240000", 7],
      ["Tarjeta de crédito", "680000", 5],
    ]
  );
  assert.deepEqual(figures.spending, { expenses: "150000", purchases: "300000" });
});

test("a day without paid orders is empty and has no average ticket", () => {
  const figures = dayFigures(close(EMPTY_DAY), summary("0", "0"));
  assert.equal(figures.isEmpty, true);
  assert.equal(figures.averageTicket, null);
  assert.deepEqual(figures.paymentMethods, []);
});

test("expenses and purchases come through on an empty day", () => {
  const figures = dayFigures(close(EMPTY_DAY), summary("80000", "45000"));
  assert.deepEqual(figures.spending, { expenses: "80000", purchases: "45000" });
});

test("a missing summary leaves the sales intact", () => {
  const figures = dayFigures(close(), null);
  assert.equal(figures.orders, 12);
  assert.equal(figures.spending, null);
});

test("a payment method without payments is left out, and an unknown one keeps its code", () => {
  const figures = dayFigures(
    close({
      by_payment_method: [
        { payment_method: "cash", collected: "0", net: "0", count: 0 },
        { payment_method: "nequi", collected: "50000", net: "50000", count: 1 },
      ],
    }),
    null
  );
  assert.deepEqual(figures.paymentMethods.map((m) => m.label), ["nequi"]);
});

test("the day is the start the backend reports", () => {
  assert.equal(dayFigures(close(), null).day, "2026-11-25T05:00:00Z");
});
