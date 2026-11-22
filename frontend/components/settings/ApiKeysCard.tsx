"use client";

import { useState, type FormEvent } from "react";
import { OrganizationApiKeyError, organizationApiKeysApi } from "@/lib/api/organizationApiKeys";
import {
  API_KEY_MAX_NAME_LENGTH,
  API_KEY_REVOKE_WARNING,
  API_KEY_SHOWN_ONCE_NOTE,
  apiKeyErrorMessage,
  apiKeyIssued,
  apiKeyRevoked,
  canRevokeApiKey,
  validateApiKeyName,
  type ApiKeysCardState,
  type ShownApiKey,
} from "@/lib/settings/apiKeys";
import type { OrganizationApiKey } from "@/types/organizationApiKeys";
import { Badge, Button, Card, CardBody, Input, Modal } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";

interface ApiKeysCardProps {
  initialState: ApiKeysCardState;
  mcpUrl: string | null;
}

const muted = { color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;
const mono = { fontFamily: "monospace", flex: "1 1 20rem", minWidth: 0, padding: "0.5rem" } as const;
const row = { display: "flex", gap: "0.5rem", alignItems: "center", flexWrap: "wrap" } as const;

function formatDate(iso: string | null): string {
  if (!iso) return "Nunca";
  const date = new Date(iso);
  return Number.isNaN(date.getTime())
    ? iso
    : date.toLocaleString("es-CO", { dateStyle: "medium", timeStyle: "short" });
}

function CopyField({ id, label, value }: { id: string; label: string; value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "0.25rem" }}>
      <label style={muted} htmlFor={id}>
        {label}
      </label>
      <div style={row}>
        <input
          id={id}
          readOnly
          value={value}
          spellCheck={false}
          autoComplete="off"
          onFocus={(e) => e.currentTarget.select()}
          style={mono}
        />
        <Button size="sm" variant="secondary" type="button" onClick={copy}>
          {copied ? "Copiado" : "Copiar"}
        </Button>
      </div>
    </div>
  );
}

export default function ApiKeysCard({ initialState, mcpUrl }: ApiKeysCardProps) {
  const [keys, setKeys] = useState<OrganizationApiKey[]>(initialState.kind === "loaded" ? initialState.keys : []);
  const [issuing, setIssuing] = useState(false);
  const [name, setName] = useState("");
  const [nameError, setNameError] = useState("");
  const [shown, setShown] = useState<ShownApiKey | null>(null);
  const [revoking, setRevoking] = useState<OrganizationApiKey | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const closeIssue = () => {
    setIssuing(false);
    setName("");
    setNameError("");
  };

  const handleIssue = async (e: FormEvent) => {
    e.preventDefault();
    const invalid = validateApiKeyName(name);
    if (invalid) {
      setNameError(invalid);
      return;
    }
    try {
      setBusy(true);
      setError("");
      setNotice("");
      const result = apiKeyIssued(keys, await organizationApiKeysApi.issue(name.trim()), mcpUrl);
      setKeys(result.keys);
      setShown(result.shown);
      closeIssue();
    } catch (err) {
      if (err instanceof OrganizationApiKeyError && err.field === "name") {
        setNameError(err.message);
      } else if (err instanceof OrganizationApiKeyError) {
        setError(apiKeyErrorMessage(err.status, err.message));
        closeIssue();
      } else {
        setError("Error al emitir la clave.");
        closeIssue();
      }
    } finally {
      setBusy(false);
    }
  };

  const handleRevoke = async (key: OrganizationApiKey) => {
    try {
      setBusy(true);
      setError("");
      setNotice("");
      await organizationApiKeysApi.revoke(key.id);
      setKeys((current) => apiKeyRevoked(current, key.id, new Date().toISOString()));
      setNotice(`Clave «${key.name}» revocada.`);
    } catch (err) {
      setError(
        err instanceof OrganizationApiKeyError ? apiKeyErrorMessage(err.status, err.message) : "Error al revocar la clave."
      );
    } finally {
      setRevoking(null);
      setBusy(false);
    }
  };

  let body;
  if (initialState.kind === "error") {
    body = <ErrorBanner message={initialState.message} />;
  } else if (keys.length === 0) {
    body = <p style={{ ...muted, marginTop: 0 }}>Todavía no hay claves.</p>;
  } else {
    body = (
      <ul style={{ listStyle: "none", padding: 0, margin: "0 0 1rem", display: "flex", flexDirection: "column", gap: "0.75rem" }}>
        {keys.map((key) => (
          <li
            key={key.id}
            style={{
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              padding: "0.75rem",
              display: "flex",
              justifyContent: "space-between",
              gap: "0.75rem",
              flexWrap: "wrap",
            }}
          >
            <div style={{ display: "flex", flexDirection: "column", gap: "0.25rem", minWidth: 0 }}>
              <div style={row}>
                <strong style={{ overflowWrap: "anywhere" }}>{key.name}</strong>
                {key.revoked_at ? <Badge tone="neutral">Revocada</Badge> : <Badge tone="success">Activa</Badge>}
              </div>
              <span style={{ ...muted, fontFamily: "monospace" }}>lgk_{key.prefix}…</span>
              <span style={muted}>
                Creada {formatDate(key.created_at)} · Último uso {formatDate(key.last_used_at)}
                {key.revoked_at && ` · Revocada ${formatDate(key.revoked_at)}`}
              </span>
            </div>
            {canRevokeApiKey(key) && (
              <div>
                <Button size="sm" variant="danger" type="button" onClick={() => setRevoking(key)} disabled={busy}>
                  Revocar
                </Button>
              </div>
            )}
          </li>
        ))}
      </ul>
    );
  }

  return (
    <Card>
      <CardBody>
        <h2 style={{ margin: "0 0 0.75rem", fontSize: "1.25rem", color: "var(--color-text-primary)" }}>Claves API</h2>
        <p style={{ ...muted, marginTop: 0 }}>
          Dan acceso a una consola o agente externo (por ejemplo, el servidor MCP) para actuar como el agente del
          restaurante. Cada clave tiene su propio usuario; al revocarla, ese usuario queda inactivo.
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
        {initialState.kind === "loaded" && (
          <Button type="button" onClick={() => setIssuing(true)} disabled={busy}>
            Emitir clave
          </Button>
        )}
      </CardBody>

      <Modal
        open={issuing}
        onClose={closeIssue}
        title="Emitir clave API"
        size="sm"
        footer={
          <>
            <Button type="button" variant="secondary" onClick={closeIssue} disabled={busy}>
              Cancelar
            </Button>
            <Button type="submit" form="issue-api-key" disabled={busy}>
              {busy ? "Emitiendo…" : "Emitir"}
            </Button>
          </>
        }
      >
        <form id="issue-api-key" onSubmit={handleIssue}>
          <Input
            label="Nombre"
            helper="Para reconocerla luego, por ejemplo «consola-local» o «agente-whatsapp»."
            value={name}
            maxLength={API_KEY_MAX_NAME_LENGTH}
            onChange={(e) => {
              setName(e.target.value);
              setNameError("");
            }}
            error={nameError || undefined}
            autoFocus
          />
        </form>
      </Modal>

      <Modal
        open={shown !== null}
        onClose={() => setShown(null)}
        title={shown ? `Clave «${shown.name}»` : undefined}
        closeOnOverlay={false}
        footer={
          <Button type="button" onClick={() => setShown(null)}>
            Listo
          </Button>
        }
      >
        {shown && (
          <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
            <CopyField id="issued-api-key" label="Clave" value={shown.key} />
            {shown.mcpUrl && <CopyField id="mcp-url" label="URL del servidor MCP" value={shown.mcpUrl} />}
            <NoticeBanner message={API_KEY_SHOWN_ONCE_NOTE} />
          </div>
        )}
      </Modal>

      <Modal
        open={revoking !== null}
        onClose={() => setRevoking(null)}
        title={revoking ? `¿Revocar «${revoking.name}»?` : undefined}
        size="sm"
        footer={
          <>
            <Button type="button" variant="secondary" onClick={() => setRevoking(null)} disabled={busy}>
              Cancelar
            </Button>
            <Button type="button" variant="danger" onClick={() => revoking && handleRevoke(revoking)} disabled={busy}>
              {busy ? "Revocando…" : "Sí, revocar"}
            </Button>
          </>
        }
      >
        <p style={{ margin: 0 }}>{API_KEY_REVOKE_WARNING}</p>
      </Modal>
    </Card>
  );
}
