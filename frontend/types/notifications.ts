export type NotificationChannelType = "discord";

export type NotificationEvent = "order.created" | "order.paid" | "reservation.requested" | "reservation.confirmed";

export type NotificationOrderType = "dine_in" | "takeaway" | "delivery";
export type NotificationReservationSource = "whatsapp" | "phone" | "walk_in" | "staff";
export type NotificationConfirmedBy = "staff" | "auto";

/** Never carries the webhook: the backend returns `…` plus its last 4 characters. */
export interface NotificationChannel {
  id: string;
  name: string;
  type: NotificationChannelType;
  enabled: boolean;
  settings: { webhook_url_hint: string };
  created_at: string;
  updated_at: string;
}

export interface NotificationChannelListResponse {
  notification_channels: NotificationChannel[];
}

export interface CreateNotificationChannelRequest {
  name: string;
  type: NotificationChannelType;
  enabled?: boolean;
  settings: { webhook_url: string };
}

/** Omit `settings` to keep the stored webhook. */
export interface UpdateNotificationChannelRequest {
  name?: string;
  enabled?: boolean;
  settings?: { webhook_url: string };
}

export interface NotificationConditions {
  order_types?: NotificationOrderType[];
  sources?: NotificationReservationSource[];
  confirmed_by?: NotificationConfirmedBy[];
  min_party_size?: number;
}

export interface NotificationRule {
  id: string;
  channel_id: string;
  event: NotificationEvent;
  conditions: NotificationConditions;
  title_prefix: string;
  expires_after_minutes: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface NotificationRuleListResponse {
  notification_rules: NotificationRule[];
}

export interface CreateNotificationRuleRequest {
  channel_id: string;
  event: NotificationEvent;
  conditions?: NotificationConditions;
  title_prefix?: string;
  expires_after_minutes?: number;
  enabled?: boolean;
}

/** `event` and `channel_id` can't change. */
export interface UpdateNotificationRuleRequest {
  conditions?: NotificationConditions;
  title_prefix?: string;
  expires_after_minutes?: number;
  enabled?: boolean;
}

export interface NotificationErrorResponse {
  error: string;
  code?: string;
  field?: string;
  destination_status?: number;
}
