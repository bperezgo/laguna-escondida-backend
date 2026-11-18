// Mirrors backend dto.DailyCloseReport (internal/domain/dto/daily_close.go).
// Money fields are decimal strings (shopspring/decimal serializes as string).

export interface DailyCloseFilters {
  date: string; // YYYY-MM-DD
}

export interface PaymentMethodBreakdown {
  payment_method: string;
  collected: string; // GROSS the customer paid (what must reconcile)
  net: string; // net of tax (reference)
  count: number;
}

export interface DailyCloseReport {
  start_date: string;
  end_date: string;
  total_orders: number;
  total_collected: string; // gross across all methods
  total_net: string;
  total_vat: string;
  total_ico: string;
  total_discount: string;
  total_tip: string;
  by_payment_method: PaymentMethodBreakdown[];
}
