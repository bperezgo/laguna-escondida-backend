"use client";

import { useEffect, useReducer, useState } from "react";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { ApiError } from "@/lib/api/config";
import { installBoxApi, type EnrollmentCode } from "@/lib/api/installBox";
import { usersApi } from "@/lib/api/users";
import { useNow } from "@/lib/kitchen/useNow";
import {
  boxPasswordReducer,
  codeExpired,
  findBoxUser,
  formatCountdown,
  INSTALLER_DOWNLOAD_PATH,
  installBoxErrorMessage,
  secondsLeft,
} from "@/lib/settings/installBox";
import type { UserWithRoles } from "@/types/user";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Button, Card, CardBody } from "@/components/ui";
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

function PasswordStep() {
  const [boxUser, setBoxUser] = useState<UserWithRoles | null>(null);
  const [usersLoaded, setUsersLoaded] = useState(false);
  const [shown, dispatch] = useReducer(boxPasswordReducer, null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    usersApi
      .getAll()
      .then((users) => !cancelled && setBoxUser(findBoxUser(users)))
      .catch((err) => !cancelled && setError(errorMessage(err)))
      .finally(() => !cancelled && setUsersLoaded(true));
    const leave = () => dispatch({ type: "left" });
    window.addEventListener("pagehide", leave);
    return () => {
      cancelled = true;
      window.removeEventListener("pagehide", leave);
      leave();
    };
  }, []);

  const generate = async () => {
    if (!boxUser) return;
    setLoading(true);
    setError("");
    try {
      const generated = await installBoxApi.generatePassword(boxUser.user.id);
      dispatch({ type: "generated", shown: generated });
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card>
      <CardBody>
        <h2 style={step}>3. Usuario de la caja</h2>
        <p style={muted}>
          Con este usuario se inicia sesión en la caja. La contraseña se muestra una sola vez; generar otra reemplaza la
          anterior y la caja la recibe en su próxima sincronización.
        </p>
        <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem", marginTop: "1rem" }}>
          {usersLoaded && !boxUser && !error && (
            <p style={muted}>No hay un usuario de caja (un Gerente sin correo). Créalo en Usuarios.</p>
          )}
          {shown && (
            <>
              <CopyField id="box-username" label="Usuario" value={shown.username} />
              <CopyField id="box-password" label="Contraseña" value={shown.password} />
              <p style={muted}>Cópiala ahora: no se volverá a mostrar.</p>
            </>
          )}
          {error && <ErrorBanner message={error} />}
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <Button onClick={generate} disabled={loading || !boxUser}>
              {loading ? "Generando..." : "Generar contraseña"}
            </Button>
            {shown && (
              <Button variant="secondary" onClick={() => dispatch({ type: "hidden" })}>
                Ocultar
              </Button>
            )}
          </div>
        </div>
      </CardBody>
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
        <PasswordStep />
      </>
    );
  }

  return <SettingsSectionShell title="Instalar caja">{content}</SettingsSectionShell>;
}
