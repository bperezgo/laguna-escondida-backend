export interface FinancialSummaryFilters {
  start_date: string; // YYYY-MM-DD
  end_date: string;   // YYYY-MM-DD
}

export interface RevenueData {
  total_amount: string;
  total_vat: string;
  total_ico: string;
  total_discount: string;
  total_tip: string;
  count: number;
}

export interface ExpenseCategoryData {
  category_id: string;
  category_name: string;
  category_code: string;
  total_amount: string;
  count: number;
}

export interface ExpensesData {
  total_amount: string;
  by_category: ExpenseCategoryData[];
  count: number;
}

export interface PurchasesData {
  total_amount: string;
  count: number;
}

export interface FinancialSummary {
  start_date: string;
  end_date: string;
  revenue: RevenueData;
  expenses: ExpensesData;
  purchases: PurchasesData;
  net_income: string;
}
