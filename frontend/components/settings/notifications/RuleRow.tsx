"use client";

import { useState } from "react";
import type { NotificationRule } from "@/types/notifications";
import { NotificationApiError, notificationsApi } from "@/lib/api/notifications";
import { expiryLabel, notificationSaveErrors, parseTitlePrefix, ruleSummary } from "@/lib/settings/notifications";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Badge, Button } from "@/components/ui";
import { confirmBox, muted, row } from "./styles";

interface RuleRowProps {
  rule: NotificationRule;
  canManage: boolean;
  onEdit: () => void;
  onChanged: (message?: string) => Promise<void>;
}

function prefixLabel(prefix: string): string {
  const { roleId, text } = parseTitlePrefix(prefix);
  const parts = [];
  if (roleId) parts.push(`menciona el rol ${roleId}`);
  if (text) parts.push(`empieza con "${text}"`);
  return parts.join(", ");
}

export default function RuleRow({ rule, canManage, onEdit, onChanged }: RuleRowProps) {
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const run = async (action: () => Promise<void>, fallback: string) => {
    try {
      setBusy(true);
      setError("");
      await action();
    } catch (err) {
      setError(
        err instanceof NotificationApiError
          ? (notificationSaveErrors(err.status, err.message, err.code).form ?? fallback)
          : fallback,
      );
    } finally {
      setBusy(false);
      setConfirmingDelete(false);
    }
  };

  const prefix = prefixLabel(rule.title_prefix);

  return (
    <div
      style={{
        padding: "0.75rem",
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-sm)",
        display: "flex",
        flexDirection: "column",
        gap: "0.5rem",
      }}
    >
      <div style={row}>
        <strong style={{ color: "var(--color-text-primary)" }}>{ruleSummary(rule)}</strong>
        <Badge tone={rule.enabled ? "success" : "neutral"}>{rule.enabled ? "Activa" : "Inactiva"}</Badge>
      </div>
      <span style={muted}>
        Vence si no se entrega en {expiryLabel(rule.expires_after_minutes)}
        {prefix && ` · ${prefix}`}
      </span>
      {error && <ErrorBanner message={error} />}
      {canManage && !confirmingDelete && (
        <div style={row}>
          <Button type="button" size="sm" variant="secondary" onClick={onEdit} disabled={busy}>
            Editar
          </Button>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={busy}
            onClick={() =>
              run(async () => {
                await notificationsApi.updateRule(rule.id, { enabled: !rule.enabled });
                await onChanged(rule.enabled ? "Regla desactivada." : "Regla activada.");
              }, "Error al cambiar la regla.")
            }
          >
            {rule.enabled ? "Desactivar" : "Activar"}
          </Button>
          <Button type="button" size="sm" variant="danger" onClick={() => setConfirmingDelete(true)} disabled={busy}>
            Eliminar
          </Button>
        </div>
      )}
      {confirmingDelete && (
        <div role="alertdialog" aria-label="Confirmar eliminar regla" style={confirmBox}>
          <span>¿Eliminar la regla &quot;{ruleSummary(rule)}&quot;?</span>
          <div style={row}>
            <Button
              type="button"
              variant="danger"
              disabled={busy}
              onClick={() =>
                run(async () => {
                  await notificationsApi.deleteRule(rule.id);
                  await onChanged("Regla eliminada.");
                }, "Error al eliminar la regla.")
              }
            >
              {busy ? "Eliminando…" : "Sí, eliminar"}
            </Button>
            <Button type="button" variant="secondary" onClick={() => setConfirmingDelete(false)} disabled={busy}>
              Cancelar
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
