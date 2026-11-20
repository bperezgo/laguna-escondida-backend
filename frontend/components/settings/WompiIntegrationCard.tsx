"use client";

import { useState, type FormEvent } from "react";
import type { WompiIntegration } from "@/types/organizationIntegrations";
import {
  OrganizationIntegrationError,
  organizationIntegrationsApi,
} from "@/lib/api/organizationIntegrations";
import {
  environmentLabel,
  hasWompiErrors,
  maskedHint,
  validateWompiDraft,
  wompiSaveErrors,
  type WompiCardState,
  type WompiFormErrors,
} from "@/lib/settings/integrations";
import { Badge, Button, Card, CardBody, Input } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";

interface WompiIntegrationCardProps {
  initialState: WompiCardState;
  canManage: boolean;
}

const muted = { color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;

function formatDate(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? iso
    : date.toLocaleString("es-CO", { dateStyle: "medium", timeStyle: "short" });
}

export default function WompiIntegrationCard({ initialState, canManage }: WompiIntegrationCardProps) {
  const [state, setState] = useState<WompiCardState>(initialState);
  const [editing, setEditing] = useState(false);
  const [privateKey, setPrivateKey] = useState("");
  const [eventsSecret, setEventsSecret] = useState("");
  const [errors, setErrors] = useState<WompiFormErrors>({});
  const [saving, setSaving] = useState(false);
  const [confirmingRemove, setConfirmingRemove] = useState(false);
  const [notice, setNotice] = useState("");
  const [copied, setCopied] = useState(false);

  const clearInputs = () => {
    setPrivateKey("");
    setEventsSecret("");
  };

  const closeForm = () => {
    clearInputs();
    setErrors({});
    setEditing(false);
  };

  const handleSave = async (event: FormEvent) => {
    event.preventDefault();
    const draft = { private_key: privateKey, events_secret: eventsSecret };
    const invalid = validateWompiDraft(draft);
    if (hasWompiErrors(invalid)) {
      setErrors(invalid);
      return;
    }
    try {
      setSaving(true);
      setErrors({});
      setNotice("");
      const integration = await organizationIntegrationsApi.putWompi(draft);
      setState({ kind: "configured", integration });
      setEditing(false);
      setNotice("Llaves de Wompi guardadas.");
    } catch (err) {
      if (err instanceof OrganizationIntegrationError) {
        if (err.status === 503) setState({ kind: "unavailable" });
        setErrors(wompiSaveErrors(err.status, err.message, err.field));
      } else {
        setErrors({ form: "Error al guardar las llaves." });
      }
    } finally {
      clearInputs();
      setSaving(false);
    }
  };

  const handleRemove = async () => {
    try {
      setSaving(true);
      setErrors({});
      await organizationIntegrationsApi.remove("wompi");
      setState({ kind: "not_configured" });
      setNotice("Wompi quedó desconectado. El agente no tomará pedidos con pago hasta que pongas llaves de nuevo.");
    } catch (err) {
      if (err instanceof OrganizationIntegrationError && err.status === 404) {
        setState({ kind: "not_configured" });
      } else {
        setErrors({ form: err instanceof Error ? err.message : "Error al quitar Wompi." });
      }
    } finally {
      setConfirmingRemove(false);
      setSaving(false);
    }
  };

  const copyWebhookPath = async (path: string) => {
    try {
      await navigator.clipboard.writeText(path);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  const form = (
    <form onSubmit={handleSave} autoComplete="off" style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
      {errors.form && <ErrorBanner message={errors.form} />}
      <Input
        label="Llave privada"
        type="password"
        autoComplete="off"
        spellCheck={false}
        placeholder="prv_test_… o prv_prod_…"
        value={privateKey}
        disabled={saving}
        onChange={(e) => setPrivateKey(e.target.value)}
        error={errors.private_key}
        helper="En Wompi: Desarrolladores → Llaves del API."
      />
      <Input
        label="Secreto de eventos"
        type="password"
        autoComplete="off"
        spellCheck={false}
        placeholder="test_events_… o prod_events_…"
        value={eventsSecret}
        disabled={saving}
        onChange={(e) => setEventsSecret(e.target.value)}
        error={errors.events_secret}
        helper="En Wompi: Desarrolladores → Secretos para integración técnica."
      />
      <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
        <Button type="submit" disabled={saving}>
          {saving ? "Guardando…" : "Guardar llaves"}
        </Button>
        {state.kind === "configured" && (
          <Button type="button" variant="secondary" onClick={closeForm} disabled={saving}>
            Cancelar
          </Button>
        )}
      </div>
    </form>
  );

  let body;
  if (state.kind === "unavailable") {
    body = <p style={muted}>Las integraciones no están habilitadas en este servidor.</p>;
  } else if (state.kind === "error") {
    body = <ErrorBanner message={state.message} />;
  } else if (state.kind === "not_configured") {
    body = (
      <>
        <p style={{ ...muted, marginTop: 0 }}>
          Wompi no está configurado: el agente de WhatsApp no puede tomar pedidos con pago.
        </p>
        {canManage ? form : <p style={muted}>Solo un administrador puede poner las llaves.</p>}
      </>
    );
  } else {
    const integration: WompiIntegration = state.integration;
    body = (
      <>
        <dl
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(140px, max-content) minmax(0, 1fr)",
            gap: "0.5rem 1rem",
            margin: "0 0 1rem",
          }}
        >
          <dt style={muted}>Llave privada</dt>
          <dd style={{ margin: 0, fontFamily: "monospace" }}>{maskedHint(integration.private_key_hint)}</dd>
          <dt style={muted}>Secreto de eventos</dt>
          <dd style={{ margin: 0, fontFamily: "monospace" }}>{maskedHint(integration.events_secret_hint)}</dd>
          <dt style={muted}>Actualizado</dt>
          <dd style={{ margin: 0 }}>{formatDate(integration.updated_at)}</dd>
          <dt style={muted}>URL de eventos</dt>
          <dd style={{ margin: 0, display: "flex", gap: "0.5rem", alignItems: "center", flexWrap: "wrap" }}>
            <code style={{ wordBreak: "break-all" }}>{integration.webhook_path}</code>
            <Button size="sm" variant="ghost" type="button" onClick={() => copyWebhookPath(integration.webhook_path)}>
              {copied ? "Copiado" : "Copiar"}
            </Button>
          </dd>
        </dl>
        <p style={{ ...muted, marginTop: 0 }}>
          En Wompi, pon como URL de eventos el dominio de este servidor seguido de esta ruta.
        </p>
        {editing && canManage && form}
        {!editing && errors.form && <ErrorBanner message={errors.form} />}
        {canManage && !editing && !confirmingRemove && (
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setNotice("");
                setEditing(true);
              }}
            >
              Reemplazar llaves
            </Button>
            <Button type="button" variant="danger" onClick={() => setConfirmingRemove(true)}>
              Quitar
            </Button>
          </div>
        )}
        {confirmingRemove && (
          <div
            role="alertdialog"
            aria-label="Confirmar quitar Wompi"
            style={{
              padding: "1rem",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              display: "flex",
              flexDirection: "column",
              gap: "0.75rem",
            }}
          >
            <span>
              ¿Quitar Wompi? El agente dejará de tomar pedidos con pago, y los pagos de los links ya enviados no se
              confirmarán hasta que pongas llaves de nuevo.
            </span>
            <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
              <Button type="button" variant="danger" onClick={handleRemove} disabled={saving}>
                {saving ? "Quitando…" : "Sí, quitar"}
              </Button>
              <Button type="button" variant="secondary" onClick={() => setConfirmingRemove(false)} disabled={saving}>
                Cancelar
              </Button>
            </div>
          </div>
        )}
      </>
    );
  }

  return (
    <Card>
      <CardBody>
        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "0.75rem" }}>
          <h2 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>Wompi</h2>
          {state.kind === "configured" && (
            <Badge tone={state.integration.environment === "production" ? "success" : "warning"}>
              {environmentLabel(state.integration.environment)}
            </Badge>
          )}
          {state.kind === "not_configured" && <Badge tone="neutral">No configurado</Badge>}
        </div>
        {notice && (
          <div style={{ marginBottom: "1rem" }}>
            <NoticeBanner message={notice} />
          </div>
        )}
        {body}
      </CardBody>
    </Card>
  );
}
