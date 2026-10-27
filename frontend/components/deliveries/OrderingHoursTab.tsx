"use client";

import { useMemo, useState } from "react";
import type {
  DayCode,
  OpeningWindow,
  OrderingType,
  OrganizationSettingsResponse,
} from "@/types/organizationSettings";
import {
  OrganizationSettingsError,
  organizationSettingsApi,
} from "@/lib/api/organizationSettings";
import {
  DAYS,
  describeAvailability,
  isPastMidnight,
  ORDERING_TYPE_LABELS,
  parseSettingsErrorField,
  sortSchedule,
  validateSchedule,
} from "@/lib/deliveries/schedule";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { Button, Card, CardBody } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "./Banners";

const TYPES: OrderingType[] = ["delivery", "takeaway"];

interface EditableWindow extends OpeningWindow {
  id: number;
}

type Drafts = Record<OrderingType, EditableWindow[]>;

let nextId = 1;

function draftsFrom(settings: OrganizationSettingsResponse | null): Drafts {
  const draft = (type: OrderingType) =>
    sortSchedule(settings?.ordering[type].schedule ?? []).map((w) => ({ ...w, id: nextId++ }));
  return { delivery: draft("delivery"), takeaway: draft("takeaway") };
}

/** The schedule as it will be sent: sorted, so the backend's error index maps back to a window. */
function toSchedule(windows: EditableWindow[]): { schedule: OpeningWindow[]; ids: number[] } {
  const sorted = sortSchedule(windows);
  return {
    schedule: sorted.map(({ day, opens, closes }) => ({ day, opens, closes })),
    ids: sorted.map((w) => w.id),
  };
}

interface OrderingHoursTabProps {
  settings: OrganizationSettingsResponse | null;
  loadError: string;
  onSaved: (settings: OrganizationSettingsResponse) => void;
}

export default function OrderingHoursTab({ settings, loadError, onSaved }: OrderingHoursTabProps) {
  const { hasPermission } = usePermissions();
  const canManage = hasPermission(PERMISSIONS.ORDERING_MANAGE);

  const [drafts, setDrafts] = useState<Drafts>(() => draftsFrom(settings));
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [backendErrors, setBackendErrors] = useState<Record<number, string>>({});

  // Client-side checks mirror the backend's, keyed by window id for display.
  const clientErrors = useMemo(() => {
    const byId: Record<number, string> = {};
    const listErrors: Partial<Record<OrderingType, string>> = {};
    for (const type of TYPES) {
      const { schedule, ids } = toSchedule(drafts[type]);
      for (const e of validateSchedule(schedule)) {
        if (e.index === null) listErrors[type] = e.message;
        else byId[ids[e.index]] = e.message;
      }
    }
    return { byId, listErrors };
  }, [drafts]);

  if (!settings) {
    return <ErrorBanner message={loadError || "Error al cargar los horarios y pausas."} />;
  }

  const hasClientErrors =
    Object.keys(clientErrors.byId).length > 0 || Object.keys(clientErrors.listErrors).length > 0;

  const change = (type: OrderingType, update: (windows: EditableWindow[]) => EditableWindow[]) => {
    setDrafts((d) => ({ ...d, [type]: update(d[type]) }));
    setDirty(true);
    setNotice("");
    setBackendErrors({});
  };

  const addWindow = (type: OrderingType, day: DayCode) =>
    change(type, (windows) => [...windows, { id: nextId++, day, opens: "11:00", closes: "15:00" }]);

  const editWindow = (type: OrderingType, id: number, field: "opens" | "closes", value: string) =>
    change(type, (windows) => windows.map((w) => (w.id === id ? { ...w, [field]: value } : w)));

  const removeWindow = (type: OrderingType, id: number) =>
    change(type, (windows) => windows.filter((w) => w.id !== id));

  const handleCancel = () => {
    setDrafts(draftsFrom(settings));
    setDirty(false);
    setError("");
    setBackendErrors({});
  };

  const handleSave = async () => {
    const sent = { delivery: toSchedule(drafts.delivery), takeaway: toSchedule(drafts.takeaway) };
    try {
      setSaving(true);
      setError("");
      setBackendErrors({});
      // Only the schedules: a user with ordering:manage but not ordering:pause can save.
      const saved = await organizationSettingsApi.update({
        ordering: {
          delivery: { schedule: sent.delivery.schedule },
          takeaway: { schedule: sent.takeaway.schedule },
        },
      });
      onSaved(saved);
      setDrafts(draftsFrom(saved));
      setDirty(false);
      setNotice("Horarios guardados.");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Error al guardar los horarios";
      setError(message);
      if (err instanceof OrganizationSettingsError) {
        const target = parseSettingsErrorField(err.field);
        if (target && target.index !== null) {
          const id = sent[target.type].ids[target.index];
          if (id !== undefined) setBackendErrors({ [id]: message });
        }
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "1rem", maxWidth: "960px" }}>
      {error && <ErrorBanner message={error} />}
      {notice && <NoticeBanner message={notice} />}
      {!canManage && (
        <NoticeBanner message="Solo puedes ver los horarios. Cambiarlos requiere el permiso ordering:manage." />
      )}
      <p style={{ margin: 0, color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
        Horas en la zona horaria del restaurante ({settings.timezone}). Si el cierre es antes de la
        apertura, la franja termina al día siguiente (por ejemplo, viernes 18:00 a 02:00).
      </p>

      {TYPES.map((type) => {
        const windows = drafts[type];
        return (
          <Card key={type}>
            <CardBody>
              <h2 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>
                {ORDERING_TYPE_LABELS[type]}
              </h2>
              <p style={{ margin: "0.25rem 0 1rem", color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
                <strong>Ahora:</strong> {describeAvailability(settings.availability[type], settings.timezone)}
                {windows.length === 0 && " · Sin horario: abierto siempre (si no está pausado)."}
              </p>
              {clientErrors.listErrors[type] && (
                <p style={{ color: "var(--color-danger)", margin: "0 0 0.75rem" }}>
                  {clientErrors.listErrors[type]}
                </p>
              )}

              <div style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}>
                {DAYS.map(({ code, label }) => {
                  const dayWindows = windows.filter((w) => w.day === code);
                  return (
                    <div
                      key={code}
                      style={{
                        display: "grid",
                        gridTemplateColumns: "minmax(90px, 110px) minmax(0, 1fr)",
                        gap: "0.75rem",
                        alignItems: "start",
                        padding: "0.5rem 0",
                        borderTop: "1px solid var(--color-border)",
                      }}
                    >
                      <div style={{ fontWeight: 600, paddingTop: "0.4rem", color: "var(--color-text-primary)" }}>
                        {label}
                      </div>
                      <div style={{ display: "flex", flexDirection: "column", gap: "0.4rem" }}>
                        {dayWindows.length === 0 && (
                          <span style={{ paddingTop: "0.4rem", color: "var(--color-text-muted)", fontSize: "0.9rem" }}>
                            sin horario
                          </span>
                        )}
                        {dayWindows.map((w) => {
                          const message = backendErrors[w.id] ?? clientErrors.byId[w.id];
                          return (
                            <WindowRow
                              key={w.id}
                              window={w}
                              disabled={!canManage || saving}
                              error={message}
                              onChange={(field, value) => editWindow(type, w.id, field, value)}
                              onRemove={() => removeWindow(type, w.id)}
                            />
                          );
                        })}
                        {canManage && (
                          <div>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => addWindow(type, code)}
                              disabled={saving}
                            >
                              + Agregar franja
                            </Button>
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </CardBody>
          </Card>
        );
      })}

      {canManage && (
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          <Button onClick={handleSave} disabled={!dirty || saving || hasClientErrors}>
            {saving ? "Guardando…" : "Guardar horarios"}
          </Button>
          <Button variant="secondary" onClick={handleCancel} disabled={!dirty || saving}>
            Descartar cambios
          </Button>
        </div>
      )}
    </div>
  );
}

interface WindowRowProps {
  window: OpeningWindow;
  disabled: boolean;
  error?: string;
  onChange: (field: "opens" | "closes", value: string) => void;
  onRemove: () => void;
}

function WindowRow({ window, disabled, error, onChange, onRemove }: WindowRowProps) {
  const inputStyle = {
    padding: "0.4rem 0.5rem",
    border: `1px solid ${error ? "var(--color-danger)" : "var(--color-border)"}`,
    borderRadius: "var(--radius-sm)",
    backgroundColor: "var(--color-surface)",
    color: "var(--color-text-primary)",
    fontSize: "0.95rem",
  };
  return (
    <div>
      <div style={{ display: "flex", alignItems: "center", gap: "0.5rem", flexWrap: "wrap" }}>
        <input
          type="time"
          aria-label="Abre"
          value={window.opens}
          disabled={disabled}
          onChange={(e) => onChange("opens", e.target.value)}
          style={inputStyle}
        />
        <span style={{ color: "var(--color-text-secondary)" }}>a</span>
        <input
          type="time"
          aria-label="Cierra"
          value={window.closes}
          disabled={disabled}
          onChange={(e) => onChange("closes", e.target.value)}
          style={inputStyle}
        />
        {isPastMidnight(window) && (
          <span style={{ fontSize: "0.8125rem", color: "var(--color-text-secondary)" }}>
            termina al día siguiente
          </span>
        )}
        {!disabled && (
          <Button size="sm" variant="ghost" onClick={onRemove} aria-label="Quitar franja">
            Quitar
          </Button>
        )}
      </div>
      {error && (
        <div style={{ color: "var(--color-danger)", fontSize: "0.8125rem", marginTop: "0.25rem" }}>
          {error}
        </div>
      )}
    </div>
  );
}
