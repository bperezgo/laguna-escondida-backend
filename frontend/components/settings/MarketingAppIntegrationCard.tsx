"use client";

import { useState } from "react";
import {
  OrganizationIntegrationError,
  organizationIntegrationsApi,
} from "@/lib/api/organizationIntegrations";
import {
  MARKETING_APP_PASTE_NOTE,
  MARKETING_APP_REGENERATE_WARNING,
  marketingAppActions,
  marketingAppDismissed,
  marketingAppErrorMessage,
  marketingAppGenerated,
  maskedHint,
  type MarketingAppCardState,
} from "@/lib/settings/integrations";
import { Badge, Button, Card, CardBody } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";

interface MarketingAppIntegrationCardProps {
  initialState: MarketingAppCardState;
  canManage: boolean;
}

type Confirming = "regenerate" | "remove" | null;

const muted = { color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;

const confirmBox = {
  padding: "1rem",
  border: "1px solid var(--color-danger)",
  borderRadius: "var(--radius-sm)",
  display: "flex",
  flexDirection: "column",
  gap: "0.75rem",
} as const;

function formatDate(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? iso
    : date.toLocaleString("es-CO", { dateStyle: "medium", timeStyle: "short" });
}

export default function MarketingAppIntegrationCard({ initialState, canManage }: MarketingAppIntegrationCardProps) {
  const [state, setState] = useState<MarketingAppCardState>(initialState);
  const [confirming, setConfirming] = useState<Confirming>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [copied, setCopied] = useState(false);

  const fail = (err: unknown, fallback: string) => {
    if (err instanceof OrganizationIntegrationError) {
      if (err.status === 503) setState({ kind: "unavailable" });
      setError(marketingAppErrorMessage(err.status, err.message));
    } else {
      setError(fallback);
    }
  };

  const handleGenerate = async () => {
    try {
      setBusy(true);
      setError("");
      setNotice("");
      setCopied(false);
      setState(marketingAppGenerated(await organizationIntegrationsApi.generateMarketingAppSecret()));
    } catch (err) {
      fail(err, "Error al generar el secreto.");
    } finally {
      setConfirming(null);
      setBusy(false);
    }
  };

  const handleRemove = async () => {
    try {
      setBusy(true);
      setError("");
      await organizationIntegrationsApi.remove("marketing_app");
      setState({ kind: "not_configured" });
      setNotice("Secreto quitado. Los pedidos y reservas del agente dejan de enviar eventos a marketing-app.");
    } catch (err) {
      if (err instanceof OrganizationIntegrationError && err.status === 404) {
        setState({ kind: "not_configured" });
      } else {
        fail(err, "Error al quitar el secreto.");
      }
    } finally {
      setConfirming(null);
      setBusy(false);
    }
  };

  const copySecret = async (secret: string) => {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };

  const actions = marketingAppActions(state, canManage);

  let body;
  if (state.kind === "unavailable") {
    body = <p style={muted}>Las integraciones no están habilitadas en este servidor.</p>;
  } else if (state.kind === "error") {
    body = <ErrorBanner message={state.message} />;
  } else if (state.kind === "not_configured") {
    body = (
      <p style={{ ...muted, marginTop: 0 }}>
        Sin secreto: los pedidos y reservas del agente de WhatsApp no envían eventos a marketing-app.
        {!canManage && " Solo un administrador puede generarlo."}
      </p>
    );
  } else if (state.kind === "generated") {
    body = (
      <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
        <label style={muted} htmlFor="marketing-app-secret">
          Nuevo secreto
        </label>
        <div style={{ display: "flex", gap: "0.5rem", alignItems: "center", flexWrap: "wrap" }}>
          <input
            id="marketing-app-secret"
            readOnly
            value={state.secret}
            spellCheck={false}
            autoComplete="off"
            onFocus={(e) => e.currentTarget.select()}
            style={{ fontFamily: "monospace", flex: "1 1 20rem", minWidth: 0, padding: "0.5rem" }}
          />
          <Button size="sm" variant="secondary" type="button" onClick={() => copySecret(state.secret)}>
            {copied ? "Copiado" : "Copiar"}
          </Button>
        </div>
        <NoticeBanner message={MARKETING_APP_PASTE_NOTE} />
        <div>
          <Button type="button" onClick={() => setState(marketingAppDismissed(state))}>
            Listo
          </Button>
        </div>
      </div>
    );
  } else {
    body = (
      <dl
        style={{
          display: "grid",
          gridTemplateColumns: "minmax(140px, max-content) minmax(0, 1fr)",
          gap: "0.5rem 1rem",
          margin: "0 0 1rem",
        }}
      >
        <dt style={muted}>Secreto de firma</dt>
        <dd style={{ margin: 0, fontFamily: "monospace" }}>{maskedHint(state.integration.signing_secret_hint)}</dd>
        <dt style={muted}>Actualizado</dt>
        <dd style={{ margin: 0 }}>{formatDate(state.integration.updated_at)}</dd>
      </dl>
    );
  }

  return (
    <Card>
      <CardBody>
        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "0.75rem" }}>
          <h2 style={{ margin: 0, fontSize: "1.25rem", color: "var(--color-text-primary)" }}>marketing-app</h2>
          {(state.kind === "configured" || state.kind === "generated") && <Badge tone="success">Configurado</Badge>}
          {state.kind === "not_configured" && <Badge tone="neutral">No configurado</Badge>}
        </div>
        <p style={{ ...muted, marginTop: 0 }}>
          Firma los eventos de pedidos y reservas que este sistema envía a marketing-app.
        </p>
        {notice && (
          <div style={{ marginBottom: "1rem" }}>
            <NoticeBanner message={notice} />
          </div>
        )}
        {error && (
          <div style={{ marginBottom: "1rem" }}>
            <ErrorBanner message={error} />
          </div>
        )}
        {body}
        {confirming === null && actions.length > 0 && (
          <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            {actions.includes("generate") && (
              <Button type="button" onClick={handleGenerate} disabled={busy}>
                {busy ? "Generando…" : "Generar secreto"}
              </Button>
            )}
            {actions.includes("regenerate") && (
              <Button type="button" variant="secondary" onClick={() => setConfirming("regenerate")}>
                Regenerar
              </Button>
            )}
            {actions.includes("remove") && (
              <Button type="button" variant="danger" onClick={() => setConfirming("remove")}>
                Quitar
              </Button>
            )}
          </div>
        )}
        {confirming === "regenerate" && (
          <div role="alertdialog" aria-label="Confirmar regenerar el secreto de marketing-app" style={confirmBox}>
            <span>{MARKETING_APP_REGENERATE_WARNING}</span>
            <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
              <Button type="button" variant="danger" onClick={handleGenerate} disabled={busy}>
                {busy ? "Generando…" : "Sí, generar uno nuevo"}
              </Button>
              <Button type="button" variant="secondary" onClick={() => setConfirming(null)} disabled={busy}>
                Cancelar
              </Button>
            </div>
          </div>
        )}
        {confirming === "remove" && (
          <div role="alertdialog" aria-label="Confirmar quitar el secreto de marketing-app" style={confirmBox}>
            <span>
              ¿Quitar el secreto? Los pedidos y reservas del agente dejarán de enviar eventos a marketing-app hasta que
              generes uno nuevo.
            </span>
            <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
              <Button type="button" variant="danger" onClick={handleRemove} disabled={busy}>
                {busy ? "Quitando…" : "Sí, quitar"}
              </Button>
              <Button type="button" variant="secondary" onClick={() => setConfirming(null)} disabled={busy}>
                Cancelar
              </Button>
            </div>
          </div>
        )}
      </CardBody>
    </Card>
  );
}
