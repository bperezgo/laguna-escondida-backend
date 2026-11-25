import type { OpenBill } from "../../types/order.ts";

export interface ManualPaymentStatus {
  /** When the order is cancelled if still unpaid; null if the backend sent none. */
  expiresAt: string | null;
  underReview: boolean;
}

/** An open order of the manual strategy is waiting for staff to confirm a transfer; any other order is not. */
export function manualPaymentStatus(bill: OpenBill): ManualPaymentStatus | null {
  if (bill.payment_strategy !== "manual") return null;
  return { expiresAt: bill.payment_expires_at ?? null, underReview: Boolean(bill.payment_review_started_at) };
}

export function manualPaymentLabel(status: ManualPaymentStatus): string {
  return status.underReview ? "Pago en revisión" : "Esperando confirmación de pago";
}

export function reviewActionLabel(status: ManualPaymentStatus): string {
  return status.underReview ? "Dejar de revisar" : "Revisar pago";
}
