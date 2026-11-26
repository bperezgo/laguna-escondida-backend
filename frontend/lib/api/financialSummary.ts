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

  /** Summary between two instants (RFC3339), e.g. the bounds a daily close reports. */
  async getSummaryBetween(start: string, end: string): Promise<FinancialSummary> {
    const params = new URLSearchParams({ start_date: start, end_date: end });
    return apiRequest<FinancialSummary>(`/financial/summary?${params}`);
  },
};
