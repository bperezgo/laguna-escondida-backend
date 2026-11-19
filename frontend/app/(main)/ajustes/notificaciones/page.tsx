import { serverApiRequest } from "@/lib/api/server";
import type { NotificationChannelListResponse, NotificationRuleListResponse } from "@/types/notifications";
import { notificationsPageState, type NotificationsLoadPart } from "@/lib/settings/notifications";
import NotificationsPageClient from "@/components/settings/notifications/NotificationsPageClient";

async function loadPart<T, B>(path: string, pick: (body: B) => T | undefined): Promise<NotificationsLoadPart<T>> {
  try {
    const response = await serverApiRequest<B>(path);
    const body = await response.json().catch(() => ({}));
    if (!response.ok) return { status: response.status, code: (body as { code?: string }).code };
    return { status: response.status, data: pick(body as B) };
  } catch {
    return { status: 500 };
  }
}

export default async function NotificationsPage() {
  const [channels, rules] = await Promise.all([
    loadPart("/notification-channels", (b: NotificationChannelListResponse) => b.notification_channels ?? []),
    loadPart("/notification-rules", (b: NotificationRuleListResponse) => b.notification_rules ?? []),
  ]);
  return <NotificationsPageClient initialState={notificationsPageState({ channels, rules })} />;
}
