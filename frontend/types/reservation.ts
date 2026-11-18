export type ReservationStatus =
  | "requested"
  | "confirmed"
  | "visited"
  | "declined"
  | "cancelled"
  | "no_show";

export type ReservationSource = "whatsapp" | "phone" | "walk_in" | "staff";

export interface ReservationAttribution {
  source?: string;
  ad_id?: string;
  clicked_at?: string;
  campaign_id?: string;
  campaign_name?: string;
}

export interface ReservationOrigin {
  channel_id?: string;
  conversation_id?: string;
  attribution?: ReservationAttribution;
}

export interface Reservation {
  id: string;
  contact_id: string;
  /** E.164 */
  contact_phone: string;
  name: string;
  party_size: number;
  /** RFC 3339, UTC */
  starts_at: string;
  notes?: string;
  source: ReservationSource;
  status: ReservationStatus;
  decline_reason?: string;
  cancelled_by?: "customer" | "staff";
  visited_at?: string;
  /** Once true, a visited reservation can no longer be marked as a no-show. */
  feedback_requested: boolean;
  origin: ReservationOrigin;
  created_at: string;
  updated_at: string;
}

export interface ReservationListResponse {
  reservations: Reservation[];
  total: number;
}

export type ReservationAction = "confirm" | "decline" | "cancel" | "no-show";

/** What staff can create from the web; a whatsapp request only comes from the agent. */
export interface CreateStaffReservationRequest {
  customer_phone: string;
  name: string;
  party_size: number;
  /** RFC 3339 with the organization's offset. */
  starts_at: string;
  notes?: string;
  source: "phone" | "staff";
}
