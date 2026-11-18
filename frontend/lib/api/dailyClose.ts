import { apiRequest } from './config';
import type { DailyCloseReport } from '@/types/dailyClose';

export const dailyCloseApi = {
  /**
   * Get the end-of-day money reconciliation for a single business day (YYYY-MM-DD).
   * The backend defaults to today (America/Bogota) when date is omitted.
   */
  async getDailyClose(date: string): Promise<DailyCloseReport> {
    const params = new URLSearchParams();
    if (date) params.set('date', date);
    const qs = params.toString();
    return apiRequest<DailyCloseReport>(`/reports/daily-close${qs ? `?${qs}` : ''}`);
  },
};
