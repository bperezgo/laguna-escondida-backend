import { apiRequest } from "./config";
import type { FeedbackListResponse } from "@/types/feedback";

export const feedbackApi = {
  /** Days are YYYY-MM-DD in the organization's timezone, both inclusive. */
  list(from: string, to: string): Promise<FeedbackListResponse> {
    const query = new URLSearchParams({ from, to });
    return apiRequest<FeedbackListResponse>(`/feedback?${query}`);
  },
};
