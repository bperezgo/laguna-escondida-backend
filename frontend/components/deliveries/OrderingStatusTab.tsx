"use client";

import { useState } from "react";
import type {
  OrderingType,
  OrganizationSettingsResponse,
} from "@/types/organizationSettings";
import { organizationSettingsApi } from "@/lib/api/organizationSettings";
import { describeAvailability, ORDERING_TYPE_LABELS } from "@/lib/deliveries/schedule";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { Badge, Button, Card, CardBody, Input } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "./Banners";

const TYPES: OrderingType[] = ["delivery", "takeaway"];
const MAX_REASON_LENGTH = 255;

interface PauseDraft {
  paused: boolean;
  reason: string;
}

type Drafts = Record<OrderingType, PauseDraft>;

function draftsFrom(settings: OrganizationSettingsResponse | null): Drafts {
  const draft = (type: OrderingType): PauseDraft => ({
    paused: settings?.ordering[type].paused ?? false,
    reason: settings?.ordering[type].paused_reason ?? "",
  });
  return { delivery: draft("delivery"), takeaway: draft("takeaway") };
}

interface OrderingStatusTabProps {
  settings: OrganizationSettingsResponse | null;
  loadError: string;
  onSaved: (settings: OrganizationSettingsResponse) => void;
}

export default function OrderingStatusTab({ settings, loadError, onSaved }: OrderingStatusTabProps) {
  const { hasPermission } = usePermissions();
  const canPause = hasPermission(PERMISSIONS.ORDERING_PAUSE);

  const [drafts, setDrafts] = useState<Drafts>(() => draftsFrom(settings));
  const [saving, setSaving] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  if (!settings) {
    return <ErrorBanner message={loadError || "Error al cargar los horarios y pausas."} />;
  }

  const dirty = TYPES.some(
    (type) =>
      drafts[type].paused !== settings.ordering[type].paused ||
      (drafts[type].paused &&
        drafts[type].reason.trim() !== (settings.ordering[type].paused_reason ?? "")),
  );

  const update = (type: OrderingType, change: Partial<PauseDraft>) => {
    setNotice("");
    setDrafts((d) => ({ ...d, [type]: { ...d[type], ...change } }));
  };

  const handleSave = async () => {
    try {
      setSaving(true);
      setError("");
      // Only the pause fields: a user with ordering:pause but not ordering:manage can save.
      const saved = await organizationSettingsApi.update({
        ordering: {
          delivery: {
            paused: drafts.delivery.paused,
            paused_reason: drafts.delivery.paused ? drafts.delivery.reason.trim() || null : null,
          },
          takeaway: {
            paused: drafts.takeaway.paused,
            paused_reason: drafts.takeaway.paused ? drafts.takeaway.reason.trim() || null : null,
          },
        },
      });
      onSaved(saved);
      setDrafts(draftsFrom(saved));
      setNotice("Cambios guardados.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al guardar");
    } finally {
      setSaving(false);
    }
  };

  const handleRefresh = async () => {
    try {
      setRefreshing(true);
      setError("");
      onSaved(await organizationSettingsApi.get());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al actualizar");
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "1rem", maxWidth: "760px" }}>
      {error && <ErrorBanner message={error} />}
      {notice && <NoticeBanner message={notice} />}
      {!canPause && (
        <NoticeBanner message="Solo puedes ver el estado. Pausar requiere el permiso ordering:pause." />
      )}

      {TYPES.map((type) => {
        const availability = settings.availability[type];
        const draft = drafts[type];
        return (
          <Card key={type}>
            <CardBody>
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  gap: "0.75rem",
                  flexWrap: "wrap",
                }}
              >
                <h2 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>
                  {ORDERING_TYPE_LABELS[type]}
                </h2>
                <Badge tone={availability.available ? "success" : availability.reason === "paused" ? "danger" : "warning"}>
                  {availability.available ? "Recibiendo" : availability.reason === "paused" ? "Pausado" : "Cerrado"}
                </Badge>
              </div>
              <p style={{ margin: "0.5rem 0 1rem", color: "var(--color-text-secondary)" }}>
                <strong>Ahora:</strong> {describeAvailability(availability, settings.timezone)}
              </p>

              <fieldset
                disabled={!canPause || saving}
                aria-label={`Estado de ${ORDERING_TYPE_LABELS[type]}`}
                style={{ border: "none", margin: 0, padding: 0, display: "flex", flexDirection: "column", gap: "0.75rem" }}
              >
                <div style={{ display: "flex", gap: "1.5rem", flexWrap: "wrap" }}>
                  <label style={{ display: "flex", alignItems: "center", gap: "0.4rem", cursor: "pointer" }}>
                    <input
                      type="radio"
                      name={`${type}-status`}
                      checked={!draft.paused}
                      onChange={() => update(type, { paused: false })}
                    />
                    Recibiendo
                  </label>
                  <label style={{ display: "flex", alignItems: "center", gap: "0.4rem", cursor: "pointer" }}>
                    <input
                      type="radio"
                      name={`${type}-status`}
                      checked={draft.paused}
                      onChange={() => update(type, { paused: true })}
                    />
                    Pausado
                  </label>
                </div>
                {draft.paused && (
                  <Input
                    label="Motivo (opcional)"
                    value={draft.reason}
                    maxLength={MAX_REASON_LENGTH}
                    placeholder="cocina saturada"
                    helper="Lo ven el personal y el agente de WhatsApp."
                    onChange={(e) => update(type, { reason: e.target.value })}
                  />
                )}
              </fieldset>
            </CardBody>
          </Card>
        );
      })}

      <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
        {canPause && (
          <Button onClick={handleSave} disabled={!dirty || saving}>
            {saving ? "Guardando…" : "Guardar"}
          </Button>
        )}
        <Button variant="secondary" onClick={handleRefresh} disabled={refreshing}>
          {refreshing ? "Actualizando…" : "Actualizar estado"}
        </Button>
      </div>
      <p style={{ margin: 0, fontSize: "0.8125rem", color: "var(--color-text-muted)" }}>
        Una pausa dura hasta que alguien la quite. Las mesas (consumo en el local) nunca se pausan.
      </p>
    </div>
  );
}
