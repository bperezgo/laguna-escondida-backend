"use client";

import { useEffect, useState } from "react";
import { ApiError } from "@/lib/api/config";
import { invitationsApi } from "@/lib/api/invitations";
import {
  invitableRoles,
  invitationExpired,
  invitationIssued,
  invitationRevoked,
  inviteBody,
  inviteErrorMessage,
  validateInvite,
  type InviteErrors,
} from "@/lib/invitations";
import { usePermissions, PERMISSIONS } from "@/lib/permissions";
import { useNow } from "@/lib/kitchen/useNow";
import { roleLabel, type Role } from "@/types/user";
import type { Invitation } from "@/types/invitation";
import { Badge, Button, Card, CardBody, Input, Modal } from "@/components/ui";
import CopyField from "@/components/settings/CopyField";

function messageOf(err: unknown): string {
  if (err instanceof ApiError) return inviteErrorMessage(err.status, err.message);
  return err instanceof Error ? err.message : "Error al administrar las invitaciones.";
}

function formatDate(iso: string): string {
  const date = new Date(iso);
  return Number.isNaN(date.getTime()) ? iso : date.toLocaleString("es-CO", { dateStyle: "medium", timeStyle: "short" });
}

function InviteForm({
  roles,
  onCancel,
  onSubmit,
  isLoading,
  error,
}: {
  roles: Role[];
  onCancel: () => void;
  onSubmit: (form: { email: string; name: string; roleIds: number[] }) => void;
  isLoading: boolean;
  error: string;
}) {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [roleIds, setRoleIds] = useState<number[]>([]);
  const [errors, setErrors] = useState<InviteErrors>({});

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const form = { email, name, roleIds };
    const found = validateInvite(form);
    setErrors(found);
    if (Object.keys(found).length === 0) onSubmit(form);
  };

  return (
    <Modal
      open
      onClose={onCancel}
      title="Invitar a una persona"
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancelar
          </Button>
          <Button type="submit" form="invite-form" disabled={isLoading}>
            {isLoading ? "Invitando..." : "Crear invitación"}
          </Button>
        </>
      }
    >
      <form id="invite-form" onSubmit={submit} style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}>
        <p style={{ margin: 0, color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
          Recibirás un enlace para compartir, por ejemplo por WhatsApp. Vale por 7 días. El personal de la caja (meseros,
          cocina) se crea con “Nuevo Usuario”.
        </p>
        <Input label="Nombre *" value={name} onChange={(e) => setName(e.target.value)} error={errors.name} maxLength={255} />
        <Input
          label="Correo *"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          error={errors.email}
          maxLength={255}
          autoComplete="off"
        />
        <div>
          <span style={{ display: "block", marginBottom: "0.5rem", fontWeight: 500, fontSize: "0.875rem" }}>Roles *</span>
          <div style={{ display: "flex", flexWrap: "wrap", gap: "0.5rem" }}>
            {roles.map((role) => (
              <label key={role.id} style={{ display: "flex", alignItems: "center", gap: "0.4rem", fontSize: "0.875rem" }}>
                <input
                  type="checkbox"
                  checked={roleIds.includes(role.id)}
                  onChange={() =>
                    setRoleIds((prev) => (prev.includes(role.id) ? prev.filter((id) => id !== role.id) : [...prev, role.id]))
                  }
                />
                {roleLabel(role.name)}
              </label>
            ))}
          </div>
          {errors.roles && <p style={{ margin: "0.4rem 0 0", color: "var(--color-danger)", fontSize: "0.8rem" }}>{errors.roles}</p>}
        </div>
        {error && (
          <p role="alert" style={{ margin: 0, color: "var(--color-danger)", fontSize: "0.875rem" }}>
            {error}
          </p>
        )}
      </form>
    </Modal>
  );
}

export default function InvitationsSection({ roles }: { roles: Role[] }) {
  const { hasPermission, isAdmin, isEdge, isLoading } = usePermissions();
  const allowed = !isLoading && !isEdge && hasPermission(PERMISSIONS.USERS_CREATE);

  const [pending, setPending] = useState<Invitation[]>([]);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);
  const [formError, setFormError] = useState("");
  const [busy, setBusy] = useState(false);
  const [shownLink, setShownLink] = useState<{ name: string; link: string } | null>(null);
  const [revokeTarget, setRevokeTarget] = useState<Invitation | null>(null);
  const now = useNow(60_000);

  useEffect(() => {
    if (!allowed) return;
    let cancelled = false;
    invitationsApi
      .list()
      .then((list) => !cancelled && setPending(list))
      .catch((err) => !cancelled && setError(messageOf(err)));
    return () => {
      cancelled = true;
    };
  }, [allowed]);

  if (!allowed) return null;

  const offered = invitableRoles(roles, isAdmin);

  const create = async (form: { email: string; name: string; roleIds: number[] }) => {
    setBusy(true);
    setFormError("");
    try {
      const issued = await invitationsApi.create(inviteBody(form));
      setPending((prev) => invitationIssued(prev, issued));
      setShowForm(false);
      setShownLink({ name: issued.invitation.name, link: issued.link });
    } catch (err) {
      setFormError(messageOf(err));
    } finally {
      setBusy(false);
    }
  };

  const newLink = async (invitation: Invitation) => {
    setBusy(true);
    setError("");
    try {
      const issued = await invitationsApi.newLink(invitation.id);
      setPending((prev) => invitationIssued(prev, issued));
      setShownLink({ name: issued.invitation.name, link: issued.link });
    } catch (err) {
      setError(messageOf(err));
      if (err instanceof ApiError && err.status === 404) setPending((prev) => invitationRevoked(prev, invitation.id));
    } finally {
      setBusy(false);
    }
  };

  const revoke = async () => {
    if (!revokeTarget) return;
    setBusy(true);
    setError("");
    try {
      await invitationsApi.revoke(revokeTarget.id);
      setPending((prev) => invitationRevoked(prev, revokeTarget.id));
      setRevokeTarget(null);
    } catch (err) {
      setError(messageOf(err));
      if (err instanceof ApiError && err.status === 404) {
        setPending((prev) => invitationRevoked(prev, revokeTarget.id));
        setRevokeTarget(null);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <section style={{ marginTop: "2rem" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: "1rem", flexWrap: "wrap", marginBottom: "1rem" }}>
        <div>
          <h2 style={{ margin: 0, fontSize: "1.375rem", color: "var(--color-text-primary)" }}>Invitaciones pendientes</h2>
          <p style={{ margin: "0.25rem 0 0", color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
            Para quienes usan la app en la nube: administradores, gerentes, contadores y repartidores.
          </p>
        </div>
        <Button
          onClick={() => {
            setFormError("");
            setShowForm(true);
          }}
        >
          + Invitar
        </Button>
      </div>

      {error && (
        <p role="alert" style={{ margin: "0 0 1rem", color: "var(--color-danger)", fontSize: "0.875rem" }}>
          {error}
        </p>
      )}

      <Card>
        <CardBody>
          {pending.length === 0 ? (
            <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>No hay invitaciones pendientes.</p>
          ) : (
            <ul style={{ listStyle: "none", margin: 0, padding: 0, display: "flex", flexDirection: "column", gap: "0.75rem" }}>
              {pending.map((invitation) => {
                const expired = invitationExpired(invitation, now);
                return (
                  <li
                    key={invitation.id}
                    style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: "1rem", flexWrap: "wrap" }}
                  >
                    <div>
                      <strong style={{ color: "var(--color-text-primary)" }}>{invitation.name}</strong>{" "}
                      <span style={{ color: "var(--color-text-secondary)" }}>{invitation.email}</span>
                      <div style={{ fontSize: "0.8rem", color: "var(--color-text-secondary)", marginTop: "0.25rem" }}>
                        {invitation.roles.map((r) => roleLabel(r.name)).join(", ")}
                        {invitation.invited_by_name && ` · invitado por ${invitation.invited_by_name}`}
                        {" · "}
                        {expired ? "venció" : `vence ${formatDate(invitation.expires_at)}`}
                      </div>
                    </div>
                    <div style={{ display: "flex", gap: "0.5rem", alignItems: "center" }}>
                      {expired && <Badge tone="warning">Vencida</Badge>}
                      <Button size="sm" variant="secondary" onClick={() => newLink(invitation)} disabled={busy}>
                        Nuevo enlace
                      </Button>
                      <Button size="sm" variant="danger" onClick={() => setRevokeTarget(invitation)} disabled={busy}>
                        Revocar
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </CardBody>
      </Card>

      {showForm && (
        <InviteForm roles={offered} onCancel={() => setShowForm(false)} onSubmit={create} isLoading={busy} error={formError} />
      )}

      <Modal
        open={shownLink !== null}
        onClose={() => setShownLink(null)}
        title="Enlace de invitación"
        footer={<Button onClick={() => setShownLink(null)}>Listo</Button>}
      >
        {shownLink && (
          <div style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
            <p style={{ margin: 0 }}>
              Comparte este enlace con <strong>{shownLink.name}</strong>. Vale por 7 días y no se volverá a mostrar; si se
              pierde, genera uno nuevo.
            </p>
            <CopyField id="invitation-link" label="Enlace" value={shownLink.link} />
          </div>
        )}
      </Modal>

      <Modal
        open={revokeTarget !== null}
        onClose={() => setRevokeTarget(null)}
        title="Revocar invitación"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setRevokeTarget(null)} disabled={busy}>
              Cancelar
            </Button>
            <Button variant="danger" onClick={revoke} disabled={busy}>
              Revocar
            </Button>
          </>
        }
      >
        {revokeTarget && (
          <p style={{ margin: 0 }}>
            El enlace de <strong>{revokeTarget.name}</strong> ({revokeTarget.email}) dejará de funcionar.
          </p>
        )}
      </Modal>
    </section>
  );
}
