/** The ordering types that have settings; dine-in is never gated. */
export type OrderingType = "delivery" | "takeaway";

export type DayCode = "mon" | "tue" | "wed" | "thu" | "fri" | "sat" | "sun";

/** Read in the organization's timezone; `closes` earlier than `opens` runs past midnight. */
export interface OpeningWindow {
  day: DayCode;
  /** 24-hour "HH:MM". */
  opens: string;
  closes: string;
}

export interface OrderTypeSettings {
  paused: boolean;
  paused_reason: string | null;
  /** Empty means always open (when not paused). */
  schedule: OpeningWindow[];
}

export interface OrderingSettings {
  delivery: OrderTypeSettings;
  takeaway: OrderTypeSettings;
}

export type AvailabilityReason = "paused" | "outside_hours";

export interface Availability {
  available: boolean;
  reason: AvailabilityReason | null;
  paused_reason: string | null;
  /** RFC 3339 with the organization's offset. */
  closes_at: string | null;
  next_opens_at: string | null;
}

export interface OrderingAvailability {
  delivery: Availability;
  takeaway: Availability;
}

/** Auto-confirm policy for WhatsApp reservations. A request failing any rule waits for staff. */
export interface ReservationSettings {
  auto_confirm_enabled: boolean;
  /** 1–50 */
  max_party_size: number;
  /** 0–10080 */
  min_notice_minutes: number;
  /** Required while enabled. */
  daily_guest_cap: number | null;
  /** Empty means any time. */
  schedule: OpeningWindow[];
}

export interface OrganizationSettingsResponse {
  ordering: OrderingSettings;
  reservations: ReservationSettings;
  availability: OrderingAvailability;
  timezone: string;
}

/** A field left out is unchanged. Pause fields need ordering:pause, schedules ordering:manage. */
export interface OrderTypeSettingsPatch {
  paused?: boolean;
  paused_reason?: string | null;
  schedule?: OpeningWindow[];
}

/** Needs settings:reservations:update. A cap can't be set back to null; turn the policy off instead. */
export interface ReservationSettingsPatch {
  auto_confirm_enabled?: boolean;
  max_party_size?: number;
  min_notice_minutes?: number;
  daily_guest_cap?: number;
  schedule?: OpeningWindow[];
}

export interface UpdateOrganizationSettingsRequest {
  ordering?: {
    delivery?: OrderTypeSettingsPatch;
    takeaway?: OrderTypeSettingsPatch;
  };
  reservations?: ReservationSettingsPatch;
}
