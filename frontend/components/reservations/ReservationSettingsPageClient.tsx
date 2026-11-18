"use client";

import { useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import type {
  DayCode,
  OpeningWindow,
  OrganizationSettingsResponse,
  ReservationSettings,
} from "@/types/organizationSettings";
import {
  OrganizationSettingsError,
  organizationSettingsApi,
} from "@/lib/api/organizationSettings";
import { DAYS, sortSchedule } from "@/lib/deliveries/schedule";
import {
  describeNotice,
  describePolicy,
  draftFrom,
  hasErrors,
  isEmptyPatch,
  MAX_NOTICE_MINUTES,
  MAX_PARTY_SIZE,
  parsePolicyErrorField,
  patchFrom,
  validatePolicy,
  type PolicyDraft,
  type PolicyField,
} from "@/lib/reservations/settings";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { Button, Card, CardBody, Input } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";
import WindowRow from "@/components/schedule/WindowRow";

interface EditableWindow extends OpeningWindow {
  id: number;
}

interface EditableDraft extends Omit<PolicyDraft, "schedule"> {
  schedule: EditableWindow[];
}

let nextId = 1;

function editableFrom(settings: ReservationSettings): EditableDraft {
  const draft = draftFrom(settings);
  return { ...draft, schedule: draft.schedule.map((w) => ({ ...w, id: nextId++ })) };
}

/** Sorted the way it is sent, so a schedule error's index maps back to a window. */
function sortedWindows(draft: EditableDraft): EditableWindow[] {
  return sortSchedule(draft.schedule);
}

interface ReservationSettingsPageClientProps {
  initialSettings: OrganizationSettingsResponse | null;
  loadError: string;
}

export default function ReservationSettingsPageClient({
  initialSettings,
  loadError,
}: ReservationSettingsPageClientProps) {
  const { hasPermission } = usePermissions();
  const canEdit = hasPermission(PERMISSIONS.SETTINGS_RESERVATIONS_UPDATE);

  const [saved, setSaved] = useState<ReservationSettings | null>(initialSettings?.reservations ?? null);
  const [draft, setDraft] = useState<EditableDraft | null>(() => (saved ? editableFrom(saved) : null));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [backendField, setBackendField] = useState<{ field?: PolicyField; windowId?: number; message: string } | null>(
    null,
  );

  const windows = useMemo(() => (draft ? sortedWindows(draft) : []), [draft]);
  const errors = useMemo(() => {
    if (!draft || !saved) return null;
    return validatePolicy({ ...draft, schedule: windows }, saved);
  }, [draft, saved, windows]);
  const patch = useMemo(
    () => (draft && saved ? patchFrom({ ...draft, schedule: windows }, saved) : {}),
    [draft, saved, windows],
  );

  const page = (children: ReactNode) => (
    <div style={{ minHeight: "100vh", backgroundColor: "var(--color-bg)", padding: "2rem" }}>
      <div style={{ maxWidth: "960px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "1rem" }}>
        <Link href="/reservations" style={{ color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
          ← Reservas
        </Link>
        <div>
          <h1 style={{ margin: 0, fontSize: "2rem", fontWeight: "bold", color: "var(--color-text-primary)" }}>
            Confirmación automática
          </h1>
          <p style={{ margin: "0.5rem 0 0", color: "var(--color-text-secondary)", fontSize: "1rem" }}>
            Qué reservas de WhatsApp confirma el agente sin esperar al equipo. Las demás llegan al tablero como
            solicitudes, como siempre. Nunca se rechaza una reserva por estas reglas.
          </p>
        </div>
        {children}
      </div>
    </div>
  );

  if (!initialSettings || !saved || !draft || !errors) {
    return page(<ErrorBanner message={loadError || "Error al cargar la configuración de reservas."} />);
  }

  const timezone = initialSettings.timezone;
  const changed = !isEmptyPatch(patch);
  const dirty = changed || JSON.stringify(draftFrom(saved)) !== JSON.stringify({
    ...draft,
    schedule: windows.map(({ day, opens, closes }) => ({ day, opens, closes })),
  });
  const disabled = !canEdit || saving;

  const change = (update: (d: EditableDraft) => EditableDraft) => {
    setDraft((d) => (d ? update(d) : d));
    setNotice("");
    setError("");
    setBackendField(null);
  };

  const setField = (key: "maxPartySize" | "minNoticeMinutes" | "dailyGuestCap", value: string) =>
    change((d) => ({ ...d, [key]: value }));

  const addWindow = (day: DayCode) =>
    change((d) => ({ ...d, schedule: [...d.schedule, { id: nextId++, day, opens: "12:00", closes: "22:00" }] }));

  const editWindow = (id: number, field: "opens" | "closes", value: string) =>
    change((d) => ({ ...d, schedule: d.schedule.map((w) => (w.id === id ? { ...w, [field]: value } : w)) }));

  const removeWindow = (id: number) => change((d) => ({ ...d, schedule: d.schedule.filter((w) => w.id !== id) }));

  const handleCancel = () => {
    setDraft(editableFrom(saved));
    setError("");
    setBackendField(null);
  };

  const handleSave = async () => {
    const sentIds = windows.map((w) => w.id);
    try {
      setSaving(true);
      setError("");
      setBackendField(null);
      const response = await organizationSettingsApi.update({ reservations: patch });
      setSaved(response.reservations);
      setDraft(editableFrom(response.reservations));
      setNotice("Configuración guardada.");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Error al guardar la configuración";
      setError(message);
      if (err instanceof OrganizationSettingsError) {
        const target = parsePolicyErrorField(err.field);
        if (target && "field" in target) setBackendField({ field: target.field, message });
        if (target && "scheduleIndex" in target && target.scheduleIndex !== null) {
          setBackendField({ windowId: sentIds[target.scheduleIndex], message });
        }
      }
    } finally {
      setSaving(false);
    }
  };

  const fieldError = (field: PolicyField) =>
    (backendField?.field === field ? backendField.message : undefined) ?? errors.fields[field];

  const scheduleErrorById: Record<number, string> = {};
  let scheduleListError = "";
  for (const e of errors.schedule) {
    if (e.index === null) scheduleListError = e.message;
    else scheduleErrorById[windows[e.index].id] = e.message;
  }
  if (backendField?.windowId !== undefined) scheduleErrorById[backendField.windowId] = backendField.message;

  const noticeHelper =
    errors.fields.min_notice_minutes === undefined
      ? `${describeNotice(Number(draft.minNoticeMinutes))} antes de la hora de la reserva.`
      : undefined;

  return page(
    <>
      {error && <ErrorBanner message={error} />}
      {notice && <NoticeBanner message={notice} />}
      {!canEdit && (
        <NoticeBanner message="Solo puedes ver estas reglas. Cambiarlas requiere el permiso settings:reservations:update." />
      )}

      <Card>
        <CardBody>
          <p style={{ margin: 0, color: "var(--color-text-primary)" }}>
            <strong>Ahora:</strong> {describePolicy(saved)}
          </p>
        </CardBody>
      </Card>

      <Card>
        <CardBody>
          <h2 style={{ margin: "0 0 1rem", fontSize: "1.25rem", color: "var(--color-text-primary)" }}>Reglas</h2>
          <label
            style={{ display: "flex", gap: "0.6rem", alignItems: "flex-start", marginBottom: "1.25rem", cursor: disabled ? "default" : "pointer" }}
          >
            <input
              type="checkbox"
              checked={draft.enabled}
              disabled={disabled}
              onChange={(e) => change((d) => ({ ...d, enabled: e.target.checked }))}
              style={{ marginTop: "0.2rem", width: "1.1rem", height: "1.1rem" }}
            />
            <span>
              <span style={{ fontWeight: 600, color: "var(--color-text-primary)" }}>
                Confirmar automáticamente las reservas de WhatsApp que cumplan todas las reglas
              </span>
              <br />
              <span style={{ fontSize: "0.875rem", color: "var(--color-text-secondary)" }}>
                El agente le dice al cliente que su mesa quedó confirmada, y la reserva aparece confirmada en el
                tablero. Las reservas que tome el equipo no cambian.
              </span>
            </span>
          </label>

          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
              gap: "1rem",
            }}
          >
            <Input
              label="Máximo de personas"
              type="number"
              inputMode="numeric"
              min={1}
              max={MAX_PARTY_SIZE}
              value={draft.maxPartySize}
              disabled={disabled}
              onChange={(e) => setField("maxPartySize", e.target.value)}
              error={fieldError("max_party_size")}
              helper="Grupos más grandes esperan al equipo."
            />
            <Input
              label="Anticipación mínima (minutos)"
              type="number"
              inputMode="numeric"
              min={0}
              max={MAX_NOTICE_MINUTES}
              step={15}
              value={draft.minNoticeMinutes}
              disabled={disabled}
              onChange={(e) => setField("minNoticeMinutes", e.target.value)}
              error={fieldError("min_notice_minutes")}
              helper={noticeHelper}
            />
            <Input
              label="Cupo diario de invitados"
              type="number"
              inputMode="numeric"
              min={1}
              value={draft.dailyGuestCap}
              disabled={disabled}
              placeholder="Ej. 120"
              onChange={(e) => setField("dailyGuestCap", e.target.value)}
              error={fieldError("daily_guest_cap")}
              helper="Suma de personas confirmadas ese día, de todos los canales. Al llenarse, las nuevas esperan al equipo."
            />
          </div>
        </CardBody>
      </Card>

      <Card>
        <CardBody>
          <h2 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>Horario</h2>
          <p style={{ margin: "0.25rem 0 1rem", color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
            Solo se confirman solas las reservas que empiezan dentro de estas franjas, en la zona horaria del
            restaurante ({timezone}). Sin franjas: a cualquier hora. Si el cierre es antes de la apertura, la franja
            termina al día siguiente.
          </p>
          {scheduleListError && (
            <p style={{ color: "var(--color-danger)", margin: "0 0 0.75rem" }}>{scheduleListError}</p>
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
                        {windows.length === 0 ? "cualquier hora" : "no se confirman solas"}
                      </span>
                    )}
                    {dayWindows.map((w) => (
                      <WindowRow
                        key={w.id}
                        window={w}
                        disabled={disabled}
                        error={scheduleErrorById[w.id]}
                        onChange={(field, value) => editWindow(w.id, field, value)}
                        onRemove={() => removeWindow(w.id)}
                      />
                    ))}
                    {canEdit && (
                      <div>
                        <Button size="sm" variant="ghost" onClick={() => addWindow(code)} disabled={saving}>
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

      {canEdit && (
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          <Button onClick={handleSave} disabled={!changed || saving || hasErrors(errors)}>
            {saving ? "Guardando…" : "Guardar"}
          </Button>
          <Button variant="secondary" onClick={handleCancel} disabled={!dirty || saving}>
            Descartar cambios
          </Button>
        </div>
      )}
    </>,
  );
}
