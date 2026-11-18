import { apiRequest } from './config';
import type { FinancialSummary, FinancialSummaryFilters } from '@/types/financialSummary';

export const financialSummaryApi = {
  /**
   * Get financial summary for a date range.
   * Converts YYYY-MM-DD dates to RFC3339 format expected by the backend.
   */
  async getSummary(filters: FinancialSummaryFilters): Promise<FinancialSummary> {
    const params = new URLSearchParams();
    params.set('start_date', `${filters.start_date}T00:00:00Z`);
    params.set('end_date', `${filters.end_date}T23:59:59Z`);
    return apiRequest<FinancialSummary>(`/financial/summary?${params}`);
  },
};
