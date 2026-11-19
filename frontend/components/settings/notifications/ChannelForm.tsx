"use client";

import { useState, type FormEvent } from "react";
import type { NotificationChannel } from "@/types/notifications";
import { NotificationApiError, notificationsApi } from "@/lib/api/notifications";
import { notificationSaveErrors, type NotificationFormErrors } from "@/lib/settings/notifications";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Button, Input } from "@/components/ui";
import { row } from "./styles";

interface ChannelFormProps {
  channel?: NotificationChannel;
  onSaved: (channel: NotificationChannel) => void | Promise<void>;
  onCancel: () => void;
}

const FORM_FIELDS = ["name", "settings.webhook_url"];

export default function ChannelForm({ channel, onSaved, onCancel }: ChannelFormProps) {
  const [name, setName] = useState(channel?.name ?? "");
  const [webhook, setWebhook] = useState("");
  const [errors, setErrors] = useState<NotificationFormErrors>({ fields: {} });
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const fields: Record<string, string> = {};
    if (name.trim() === "") fields.name = "El nombre es obligatorio.";
    if (!channel && webhook.trim() === "") fields["settings.webhook_url"] = "El webhook es obligatorio.";
    if (Object.keys(fields).length > 0) {
      setErrors({ fields });
      return;
    }
    try {
      setSaving(true);
      setErrors({ fields: {} });
      const url = webhook.trim();
      const saved = channel
        ? await notificationsApi.updateChannel(channel.id, {
            name: name.trim(),
            ...(url ? { settings: { webhook_url: url } } : {}),
          })
        : await notificationsApi.createChannel({ name: name.trim(), type: "discord", settings: { webhook_url: url } });
      await onSaved(saved);
    } catch (err) {
      if (err instanceof NotificationApiError) {
        const mapped = notificationSaveErrors(err.status, err.message, err.code, err.field);
        const unplaced = Object.entries(mapped.fields).find(([field]) => !FORM_FIELDS.includes(field));
        setErrors(unplaced ? { fields: {}, form: unplaced[1] } : mapped);
      } else {
        setErrors({ fields: {}, form: "Error al guardar el canal." });
      }
    } finally {
      setWebhook("");
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} autoComplete="off" style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
      {errors.form && <ErrorBanner message={errors.form} />}
      <Input
        label="Nombre"
        placeholder="Domicilios"
        maxLength={80}
        value={name}
        disabled={saving}
        onChange={(e) => setName(e.target.value)}
        error={errors.fields.name}
      />
      <Input
        label={channel ? "Nuevo webhook de Discord" : "Webhook de Discord"}
        type="password"
        autoComplete="off"
        spellCheck={false}
        placeholder="https://discord.com/api/webhooks/…"
        value={webhook}
        disabled={saving}
        onChange={(e) => setWebhook(e.target.value)}
        error={errors.fields["settings.webhook_url"]}
        helper={
          channel
            ? "Déjalo vacío para mantener el webhook actual."
            : "En Discord: Editar canal → Integraciones → Webhooks → Copiar URL del webhook."
        }
      />
      <div style={row}>
        <Button type="submit" disabled={saving}>
          {saving ? "Guardando…" : channel ? "Guardar cambios" : "Agregar canal"}
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel} disabled={saving}>
          Cancelar
        </Button>
      </div>
    </form>
  );
}
