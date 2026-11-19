"use client";

import { useCallback, useState } from "react";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { NotificationApiError, notificationsApi } from "@/lib/api/notifications";
import {
  notificationsPageState,
  type NotificationsLoadPart,
  type NotificationsPageState,
} from "@/lib/settings/notifications";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";
import { Button, Card, CardBody } from "@/components/ui";
import SettingsSectionShell from "../SettingsSectionShell";
import ChannelCard from "./ChannelCard";
import ChannelForm from "./ChannelForm";
import { muted } from "./styles";

interface NotificationsPageClientProps {
  initialState: NotificationsPageState;
}

async function loadPart<T>(load: () => Promise<T>): Promise<NotificationsLoadPart<T>> {
  try {
    return { status: 200, data: await load() };
  } catch (err) {
    if (err instanceof NotificationApiError) return { status: err.status, code: err.code };
    return { status: 500 };
  }
}

export default function NotificationsPageClient({ initialState }: NotificationsPageClientProps) {
  const { hasPermission, isLoading } = usePermissions();
  const canRead = hasPermission(PERMISSIONS.NOTIFICATION_CHANNELS_READ);
  const canManage = hasPermission(PERMISSIONS.NOTIFICATION_CHANNELS_MANAGE);
  const [state, setState] = useState<NotificationsPageState>(initialState);
  const [addingChannel, setAddingChannel] = useState(false);
  const [notice, setNotice] = useState("");

  const reload = useCallback(async (message?: string) => {
    const [channels, rules] = await Promise.all([
      loadPart(async () => (await notificationsApi.listChannels()).notification_channels ?? []),
      loadPart(async () => (await notificationsApi.listRules()).notification_rules ?? []),
    ]);
    setState(notificationsPageState({ channels, rules }));
    setNotice(message ?? "");
  }, []);

  let content;
  if (isLoading) {
    content = null;
  } else if (!canRead) {
    // On the edge the permissions are dropped, so the page shows nothing of the cloud's state.
    content = <ErrorBanner message="No tienes acceso a las notificaciones del restaurante." />;
  } else if (state.kind === "unavailable") {
    content = <NoticeBanner message="Las notificaciones no están habilitadas en este servidor." />;
  } else if (state.kind === "error") {
    content = <ErrorBanner message={state.message} />;
  } else {
    const addForm = (
      <Card>
        <CardBody>
          <h2 style={{ margin: "0 0 0.75rem", fontSize: "1.25rem" }}>Nuevo canal de Discord</h2>
          <ChannelForm
            onSaved={(channel) => {
              setAddingChannel(false);
              return reload(`Canal "${channel.name}" agregado. Usa Probar para enviarle un mensaje de prueba.`);
            }}
            onCancel={() => setAddingChannel(false)}
          />
        </CardBody>
      </Card>
    );

    content = (
      <>
        {!canManage && (
          <NoticeBanner message="Solo un administrador puede cambiar las notificaciones. Aquí puedes ver cómo están configuradas." />
        )}
        <p style={{ ...muted, margin: 0 }}>
          Cada canal es un webhook de Discord. Sus reglas deciden qué eventos se avisan allí. El webhook se guarda
          cifrado y nunca se vuelve a mostrar: solo sus últimos 4 caracteres.
        </p>
        {notice && <NoticeBanner message={notice} />}
        {state.channels.length === 0 && !addingChannel && (
          <Card>
            <CardBody>
              <p style={{ ...muted, marginTop: 0 }}>Todavía no hay canales de notificación.</p>
              {canManage && (
                <Button type="button" onClick={() => setAddingChannel(true)}>
                  Agregar canal
                </Button>
              )}
            </CardBody>
          </Card>
        )}
        {state.channels.map((channel) => (
          <ChannelCard
            key={channel.id}
            channel={channel}
            rules={state.rulesByChannel[channel.id] ?? []}
            canManage={canManage}
            onChanged={reload}
          />
        ))}
        {canManage && addingChannel && addForm}
        {canManage && !addingChannel && state.channels.length > 0 && (
          <div>
            <Button type="button" variant="secondary" onClick={() => setAddingChannel(true)}>
              Agregar canal
            </Button>
          </div>
        )}
      </>
    );
  }

  return <SettingsSectionShell title="Notificaciones">{content}</SettingsSectionShell>;
}
