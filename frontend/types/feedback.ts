export interface FeedbackReservation {
  id: string;
  name: string;
  party_size: number;
  /** RFC 3339, UTC */
  starts_at: string;
}

export interface Feedback {
  id: string;
  feedback_request_id: string;
  contact_id: string;
  /** E.164 */
  contact_phone: string;
  /** 1–5 */
  rating: number;
  comment?: string;
  /** True for ratings of 1–3. */
  needs_attention: boolean;
  reservation: FeedbackReservation;
  created_at: string;
  /** The latest answer; the list is filtered and ordered on it. */
  updated_at: string;
}

export interface FeedbackListResponse {
  feedback: Feedback[];
  total: number;
}
