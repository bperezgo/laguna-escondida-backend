import { apiRequest } from "./config";
import type {
  CreateStaffReservationRequest,
  Reservation,
  ReservationAction,
  ReservationListResponse,
} from "@/types/reservation";

export const reservationsApi = {
  /** A business day (YYYY-MM-DD in the organization's timezone), oldest first. */
  listDay(day: string): Promise<ReservationListResponse> {
    return apiRequest<ReservationListResponse>(`/reservations?date=${encodeURIComponent(day)}`);
  },

  /** Every request still waiting for staff, starting at or after `from` (RFC 3339), any day. */
  listPending(from: string): Promise<ReservationListResponse> {
    const query = new URLSearchParams({ status: "requested", from });
    return apiRequest<ReservationListResponse>(`/reservations?${query}`);
  },

  /** Stored as confirmed: no message goes to the guest. */
  create(request: CreateStaffReservationRequest): Promise<Reservation> {
    return apiRequest<Reservation>("/reservations", {
      method: "POST",
      body: JSON.stringify(request),
    });
  },

  act(id: string, action: ReservationAction, reason?: string): Promise<Reservation> {
    const body = action === "decline" && reason ? JSON.stringify({ reason }) : undefined;
    return apiRequest<Reservation>(`/reservations/${encodeURIComponent(id)}/${action}`, {
      method: "POST",
      body,
    });
  },
};
