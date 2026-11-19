import type {
  CreateNotificationChannelRequest,
  CreateNotificationRuleRequest,
  NotificationChannel,
  NotificationChannelListResponse,
  NotificationRule,
  NotificationRuleListResponse,
  UpdateNotificationChannelRequest,
  UpdateNotificationRuleRequest,
} from "@/types/notifications";

/** Keeps the backend's status, code, field and destination status, e.g. 502 with Discord's 404. */
export class NotificationApiError extends Error {
  status: number;
  code?: string;
  field?: string;
  destinationStatus?: number;

  constructor(message: string, status: number, code?: string, field?: string, destinationStatus?: number) {
    super(message);
    this.name = "NotificationApiError";
    this.status = status;
    this.code = code;
    this.field = field;
    this.destinationStatus = destinationStatus;
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...options,
    headers: { "Content-Type": "application/json", ...options?.headers },
  });
  if (response.status === 401) {
    window.location.href = "/signin";
    throw new NotificationApiError("Unauthorized", 401);
  }
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new NotificationApiError(
      body.error || `HTTP error! status: ${response.status}`,
      response.status,
      body.code,
      body.field,
      body.destination_status,
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

function json(method: string, body: unknown): RequestInit {
  return { method, body: JSON.stringify(body) };
}

export const notificationsApi = {
  listChannels(): Promise<NotificationChannelListResponse> {
    return request("/notification-channels");
  },
  createChannel(body: CreateNotificationChannelRequest): Promise<NotificationChannel> {
    return request("/notification-channels", json("POST", body));
  },
  updateChannel(id: string, body: UpdateNotificationChannelRequest): Promise<NotificationChannel> {
    return request(`/notification-channels/${encodeURIComponent(id)}`, json("PUT", body));
  },
  deleteChannel(id: string): Promise<void> {
    return request(`/notification-channels/${encodeURIComponent(id)}`, { method: "DELETE" });
  },
  testChannel(id: string): Promise<void> {
    return request(`/notification-channels/${encodeURIComponent(id)}/test`, { method: "POST" });
  },
  listRules(): Promise<NotificationRuleListResponse> {
    return request("/notification-rules");
  },
  createRule(body: CreateNotificationRuleRequest): Promise<NotificationRule> {
    return request("/notification-rules", json("POST", body));
  },
  updateRule(id: string, body: UpdateNotificationRuleRequest): Promise<NotificationRule> {
    return request(`/notification-rules/${encodeURIComponent(id)}`, json("PUT", body));
  },
  deleteRule(id: string): Promise<void> {
    return request(`/notification-rules/${encodeURIComponent(id)}`, { method: "DELETE" });
  },
};
