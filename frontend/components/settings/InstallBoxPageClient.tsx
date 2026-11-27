"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { ApiError } from "@/lib/api/config";
import { installBoxApi, type EnrollmentCode } from "@/lib/api/installBox";
import { usersApi } from "@/lib/api/users";
import { useNow } from "@/lib/kitchen/useNow";
import {
  boxUsers,
  codeExpired,
  formatCountdown,
  INSTALLER_DOWNLOAD_PATH,
  installBoxErrorMessage,
  secondsLeft,
} from "@/lib/settings/installBox";
import type { UserWithRoles } from "@/types/user";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Button, Card, CardBody } from "@/components/ui";
import ResetPasswordModal from "@/components/users/ResetPasswordModal";
import { roleLabel } from "@/types/user";
import SettingsSectionShell from "./SettingsSectionShell";
import CopyField from "./CopyField";

function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return installBoxErrorMessage(err.status, err.message);
  return err instanceof Error ? err.message : "No se pudo completar la acción.";
}

const step = { margin: "0 0 0.5rem", fontSize: "1.125rem", color: "var(--color-text-primary)" } as const;
const muted = { margin: 0, color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;

function CodeStep() {
  const now = useNow(1000);
  const [code, setCode] = useState<EnrollmentCode | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const issue = async () => {
    setLoading(true);
    setError("");
    try {
      setCode(await installBoxApi.issueCode());
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const expired = code ? codeExpired(code.expires_at, now) : false;

  return (
    <Card>
      <CardBody>
        <h2 style={step}>2. Código de instalación</h2>
        <p style={muted}>
          El instalador te pide este código. Sirve una sola vez y por 30 minutos; un código nuevo reemplaza al anterior.
        </p>
        <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem", marginTop: "1rem" }}>
          {code && !expired && (
            <>
              <CopyField id="enrollment-code" label="Código" value={code.code} />
              <p style={muted} aria-live="polite">
                Vence en {formatCountdown(secondsLeft(code.expires_at, now))}
              </p>
            </>
          )}
          {code && expired && <p style={muted}>El código venció. Genera uno nuevo.</p>}
          {error && <ErrorBanner message={error} />}
          <div>
            <Button onClick={issue} disabled={loading}>
              {loading ? "Generando..." : code ? "Nuevo código" : "Generar código"}
            </Button>
          </div>
        </div>
      </CardBody>
    </Card>
  );
}

const CREATE_BOX_USER_HREF = "/users?nuevo=caja";

function BoxUsersStep() {
  const [users, setUsers] = useState<UserWithRoles[]>([]);
  const [usersLoaded, setUsersLoaded] = useState(false);
  const [target, setTarget] = useState<UserWithRoles | null>(null);
  const [saving, setSaving] = useState(false);
  const [changed, setChanged] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    usersApi
      .getAll()
      .then((all) => !cancelled && setUsers(boxUsers(all)))
      .catch((err) => !cancelled && setError(errorMessage(err)))
      .finally(() => !cancelled && setUsersLoaded(true));
    return () => {
      cancelled = true;
    };
  }, []);

  const submit = async (password: string) => {
    if (!target) return;
    setSaving(true);
    setError("");
    try {
      await usersApi.resetPassword(target.user.id, password);
      setTarget(null);
      setChanged(true);
    } catch (err) {
      setTarget(null);
      setChanged(false);
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardBody>
        <h2 style={step}>3. Usuarios de la caja</h2>
        <p style={muted}>
          En la caja se inicia sesión con el <strong>usuario</strong> (no con el correo). Tu cuenta de Google no sirve
          para la caja: crea un usuario de caja y ponle la contraseña que quieras.
        </p>
        <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem", marginTop: "1rem" }}>
          {users.map((u) => (
            <div
              key={u.user.id}
              style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: "1rem", flexWrap: "wrap" }}
            >
              <div>
                <strong style={{ color: "var(--color-text-primary)" }}>{u.user.username}</strong>
                <p style={muted}>
                  {u.user.name} · {u.roles.map((r) => roleLabel(r.name)).join(", ")}
                </p>
              </div>
              <Button variant="secondary" onClick={() => setTarget(u)}>
                Cambiar contraseña
              </Button>
            </div>
          ))}
          {usersLoaded && users.length === 0 && !error && (
            <div>
              <p style={{ ...muted, marginBottom: "0.75rem" }}>Todavía no hay un usuario de caja.</p>
              <Link
                href={CREATE_BOX_USER_HREF}
                style={{ color: "var(--color-primary)", fontWeight: 600, textDecoration: "none" }}
              >
                Crear usuario de caja
              </Link>
            </div>
          )}
          {changed && (
            <p style={muted} role="status">
              Contraseña cambiada. La caja la recibe en su próxima sincronización (cerca de un minuto).
            </p>
          )}
          {error && <ErrorBanner message={error} />}
        </div>
      </CardBody>
      {target && (
        <ResetPasswordModal user={target} onSubmit={submit} onCancel={() => setTarget(null)} isLoading={saving} />
      )}
    </Card>
  );
}

export default function InstallBoxPageClient() {
  const { hasPermission, isEdge, isLoading } = usePermissions();

  let content;
  if (isLoading) {
    content = null;
  } else if (isEdge || !hasPermission(PERMISSIONS.EDGE_ENROLL)) {
    content = <ErrorBanner message="Solo un administrador puede instalar la caja, desde la nube." />;
  } else {
    content = (
      <>
        <Card>
          <CardBody>
            <h2 style={step}>1. Descarga el instalador</h2>
            <p style={muted}>Instálalo en el computador de la caja (Windows).</p>
            <div style={{ marginTop: "1rem" }}>
              <a
                href={INSTALLER_DOWNLOAD_PATH}
                style={{
                  display: "inline-block",
                  padding: "0.6rem 1.25rem",
                  borderRadius: "var(--radius-sm)",
                  backgroundColor: "var(--color-primary)",
                  color: "white",
                  fontWeight: 600,
                  textDecoration: "none",
                }}
              >
                Descargar instalador
              </a>
            </div>
          </CardBody>
        </Card>
        <CodeStep />
        <BoxUsersStep />
      </>
    );
  }

  return <SettingsSectionShell title="Instalar caja">{content}</SettingsSectionShell>;
}
