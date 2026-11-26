import type { DailyCloseReport } from "../../types/dailyClose.ts";
import type { FinancialSummary } from "../../types/financialSummary.ts";

export interface PaymentMethodFigure {
  method: string;
  label: string;
  collected: string;
  count: number;
}

export interface DayFigures {
  /** Start of the business day, as the backend reports it. */
  day: string;
  isEmpty: boolean;
  collected: string;
  orders: number;
  /** Null on a day without paid orders. */
  averageTicket: number | null;
  paymentMethods: PaymentMethodFigure[];
  /** Null when the summary could not be read. */
  spending: { expenses: string; purchases: string } | null;
}

const PAYMENT_LABELS: Record<string, string> = {
  cash: "Efectivo",
  credit_card: "Tarjeta de crédito",
  debit_card: "Tarjeta de débito",
  transfer_debit_bank: "Transferencia débito bancaria",
  transfer_credit_bank: "Transferencia crédito bancaria",
  transfer_debit_interbank: "Transferencia débito interbancaria",
};

export function paymentMethodLabel(code: string): string {
  return PAYMENT_LABELS[code] ?? (code === "" ? "Sin clasificar" : code);
}

export function dayFigures(close: DailyCloseReport, summary: FinancialSummary | null): DayFigures {
  const orders = close.total_orders;
  const collected = Number.parseFloat(close.total_collected);
  return {
    day: close.start_date,
    isEmpty: orders === 0,
    collected: close.total_collected,
    orders,
    averageTicket: orders > 0 && Number.isFinite(collected) ? collected / orders : null,
    paymentMethods: close.by_payment_method
      .filter((row) => row.count > 0)
      .map((row) => ({
        method: row.payment_method,
        label: paymentMethodLabel(row.payment_method),
        collected: row.collected,
        count: row.count,
      })),
    spending: summary
      ? { expenses: summary.expenses.total_amount, purchases: summary.purchases.total_amount }
      : null,
  };
}
