"use client";

import { useState } from "react";
import type { NotificationChannel, NotificationRule } from "@/types/notifications";
import { NotificationApiError, notificationsApi } from "@/lib/api/notifications";
import {
  channelTestResult,
  notificationSaveErrors,
  webhookHint,
  type ChannelTestResult,
} from "@/lib/settings/notifications";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";
import { Badge, Button, Card, CardBody } from "@/components/ui";
import ChannelForm from "./ChannelForm";
import RuleForm from "./RuleForm";
import RuleRow from "./RuleRow";
import { confirmBox, muted, row } from "./styles";

interface ChannelCardProps {
  channel: NotificationChannel;
  rules: NotificationRule[];
  canManage: boolean;
  onChanged: (message?: string) => Promise<void>;
}

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof NotificationApiError) {
    return notificationSaveErrors(err.status, err.message, err.code).form ?? fallback;
  }
  return fallback;
}

export default function ChannelCard({ channel, rules, canManage, onChanged }: ChannelCardProps) {
  const [editing, setEditing] = useState(false);
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [testResult, setTestResult] = useState<ChannelTestResult | null>(null);
  const [addingRule, setAddingRule] = useState(false);
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null);

  const run = async (action: () => Promise<void>, fallback: string) => {
    try {
      setBusy(true);
      setError("");
      await action();
    } catch (err) {
      setError(errorMessage(err, fallback));
    } finally {
      setBusy(false);
    }
  };

  const handleTest = async () => {
    setBusy(true);
    setTestResult(null);
    try {
      await notificationsApi.testChannel(channel.id);
      setTestResult(channelTestResult(204));
    } catch (err) {
      setTestResult(
        err instanceof NotificationApiError
          ? channelTestResult(err.status, err.code, err.destinationStatus)
          : channelTestResult(500),
      );
    } finally {
      setBusy(false);
    }
  };

  const toggleEnabled = () =>
    run(async () => {
      await notificationsApi.updateChannel(channel.id, { enabled: !channel.enabled });
      await onChanged(channel.enabled ? `Canal "${channel.name}" desactivado.` : `Canal "${channel.name}" activado.`);
    }, "Error al cambiar el canal.");

  const handleDelete = () =>
    run(async () => {
      await notificationsApi.deleteChannel(channel.id);
      await onChanged(`Canal "${channel.name}" eliminado con sus reglas.`);
    }, "Error al eliminar el canal.");

  return (
    <Card>
      <CardBody>
        <div style={{ ...row, marginBottom: "0.75rem" }}>
          <h2 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>{channel.name}</h2>
          <Badge tone="info" dot={false}>
            Discord
          </Badge>
          <Badge tone={channel.enabled ? "success" : "neutral"}>{channel.enabled ? "Activo" : "Inactivo"}</Badge>
          <code style={{ ...muted, marginLeft: "auto" }} title="Últimos 4 caracteres del webhook">
            {webhookHint(channel.settings?.webhook_url_hint)}
          </code>
        </div>

        {error && (
          <div style={{ marginBottom: "0.75rem" }}>
            <ErrorBanner message={error} />
          </div>
        )}
        {testResult && (
          <div style={{ marginBottom: "0.75rem" }}>
            {testResult.ok ? <NoticeBanner message={testResult.message} /> : <ErrorBanner message={testResult.message} />}
          </div>
        )}

        {canManage && editing && (
          <div style={{ marginBottom: "1rem" }}>
            <ChannelForm
              channel={channel}
              onSaved={async (saved) => {
                setEditing(false);
                await onChanged(`Canal "${saved.name}" guardado.`);
              }}
              onCancel={() => setEditing(false)}
            />
          </div>
        )}

        {canManage && !editing && !confirmingDelete && (
          <div style={{ ...row, marginBottom: "1rem" }}>
            <Button type="button" size="sm" onClick={handleTest} disabled={busy}>
              {busy ? "…" : "Probar"}
            </Button>
            <Button type="button" size="sm" variant="secondary" onClick={() => setEditing(true)} disabled={busy}>
              Editar
            </Button>
            <Button type="button" size="sm" variant="secondary" onClick={toggleEnabled} disabled={busy}>
              {channel.enabled ? "Desactivar" : "Activar"}
            </Button>
            <Button type="button" size="sm" variant="danger" onClick={() => setConfirmingDelete(true)} disabled={busy}>
              Eliminar
            </Button>
          </div>
        )}

        {confirmingDelete && (
          <div role="alertdialog" aria-label={`Confirmar eliminar ${channel.name}`} style={{ ...confirmBox, marginBottom: "1rem" }}>
            <span>
              ¿Eliminar el canal &quot;{channel.name}&quot;?{" "}
              {rules.length > 0 &&
                `También se eliminan sus ${rules.length === 1 ? "regla" : `${rules.length} reglas`} y los avisos pendientes.`}
            </span>
            <div style={row}>
              <Button type="button" variant="danger" onClick={handleDelete} disabled={busy}>
                {busy ? "Eliminando…" : "Sí, eliminar"}
              </Button>
              <Button type="button" variant="secondary" onClick={() => setConfirmingDelete(false)} disabled={busy}>
                Cancelar
              </Button>
            </div>
          </div>
        )}

        <h3 style={{ margin: "0 0 0.5rem", fontSize: "1rem", color: "var(--color-text-primary)" }}>Reglas</h3>
        {rules.length === 0 && !addingRule && (
          <p style={{ ...muted, marginTop: 0 }}>Este canal no tiene reglas: no recibe ningún aviso.</p>
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
          {rules.map((rule) =>
            canManage && editingRuleId === rule.id ? (
              <RuleForm
                key={rule.id}
                channelId={channel.id}
                rule={rule}
                onSaved={async () => {
                  setEditingRuleId(null);
                  await onChanged("Regla guardada.");
                }}
                onCancel={() => setEditingRuleId(null)}
              />
            ) : (
              <RuleRow
                key={rule.id}
                rule={rule}
                canManage={canManage}
                onEdit={() => setEditingRuleId(rule.id)}
                onChanged={onChanged}
              />
            ),
          )}
          {canManage && addingRule && (
            <RuleForm
              channelId={channel.id}
              onSaved={async () => {
                setAddingRule(false);
                await onChanged("Regla agregada.");
              }}
              onCancel={() => setAddingRule(false)}
            />
          )}
        </div>
        {canManage && !addingRule && (
          <div style={{ marginTop: "0.75rem" }}>
            <Button type="button" size="sm" variant="ghost" onClick={() => setAddingRule(true)}>
              + Agregar regla
            </Button>
          </div>
        )}
      </CardBody>
    </Card>
  );
}
