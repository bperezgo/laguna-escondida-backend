"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { Button, Card, CardBody, PasswordInput } from "@/components/ui";
import GoogleSignInButton from "@/components/auth/GoogleSignInButton";
import NoticeMessage from "@/components/auth/NoticeMessage";
import PublicLayout from "@/components/public/PublicLayout";
import { usePermissions } from "@/lib/permissions";
import { googleClientId } from "@/lib/google";
import {
  acceptFailure,
  lookupState,
  MIN_PASSWORD_LENGTH,
  validateAcceptPassword,
  type LookupState,
} from "@/lib/invitations";
import { roleLabel } from "@/types/user";
import type { InvitationLookup } from "@/types/invitation";

type PageState =
  | LookupState
  | { kind: "loading" }
  | { kind: "organization_inactive" }
  | { kind: "email_taken"; email: string };

const muted = { margin: 0, color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;

function AcceptForm({
  invitation,
  token,
  onState,
}: {
  invitation: InvitationLookup;
  token: string;
  onState: (state: PageState) => void;
}) {
  const router = useRouter();
  const { fetchPermissions } = usePermissions();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [fieldError, setFieldError] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  const accept = useCallback(
    async (body: { password: string } | { id_token: string }) => {
      setError("");
      setIsLoading(true);
      try {
        const response = await fetch(`/api/invitations/${encodeURIComponent(token)}/accept`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
          const failure = acceptFailure(response.status, data, invitation.email);
          if (failure.kind === "message") setError(failure.message);
          else if (failure.kind === "email_taken") onState({ kind: "email_taken", email: invitation.email });
          else onState({ kind: failure.kind });
          return;
        }
        await fetchPermissions();
        router.push("/home");
      } catch {
        setError("No se pudo conectar. Intenta de nuevo.");
      } finally {
        setIsLoading(false);
      }
    },
    [token, invitation.email, onState, fetchPermissions, router]
  );

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    setShowPassword(false);
    const problem = validateAcceptPassword(password, confirm);
    setFieldError(problem);
    if (!problem) accept({ password });
  };

  const handleGoogle = useCallback((idToken: string) => accept({ id_token: idToken }), [accept]);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}>
      <div>
        <h2 style={{ margin: "0 0 0.5rem", fontSize: "1.25rem", color: "var(--color-text-primary)" }}>
          {invitation.organization_name}
        </h2>
        <p style={muted}>
          {invitation.name}, te invitaron como{" "}
          <strong>{invitation.roles.map((r) => roleLabel(r.name)).join(", ")}</strong>. Entrarás con{" "}
          <strong>{invitation.email}</strong>.
        </p>
      </div>

      {googleClientId() && (
        <>
          <GoogleSignInButton text="continue_with" onCredential={handleGoogle} disabled={isLoading} />
          <p style={{ ...muted, textAlign: "center" }}>o crea una contraseña</p>
        </>
      )}

      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
        <PasswordInput
          label="Contraseña"
          id="password"
          shown={showPassword}
          onShownChange={setShowPassword}
          autoComplete="new-password"
          placeholder={`Mínimo ${MIN_PASSWORD_LENGTH} caracteres`}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={isLoading}
        />
        <PasswordInput
          label="Confirmar contraseña"
          id="confirm-password"
          shown={showPassword}
          hideToggle
          autoComplete="new-password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          error={fieldError || undefined}
          disabled={isLoading}
        />
        {error && <NoticeMessage message={error} />}
        <Button type="submit" variant="primary" fullWidth disabled={isLoading}>
          {isLoading ? "Creando cuenta..." : "Aceptar invitación"}
        </Button>
      </form>
    </div>
  );
}

export default function AcceptInvitationClient() {
  const params = useParams<{ token: string }>();
  const token = params?.token ?? "";
  const [state, setState] = useState<PageState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    fetch(`/api/invitations/${encodeURIComponent(token)}`)
      .then(async (response) => {
        const body = await response.json().catch(() => ({}));
        if (!cancelled) setState(lookupState(response.status, body));
      })
      .catch(() => !cancelled && setState({ kind: "error", message: "No se pudo conectar. Intenta de nuevo." }));
    return () => {
      cancelled = true;
    };
  }, [token]);

  let content;
  switch (state.kind) {
    case "loading":
      content = <p style={{ ...muted, textAlign: "center" }}>Cargando invitación...</p>;
      break;
    case "invalid":
      content = (
        <NoticeMessage message="Este enlace no es válido o ya venció. Pide a quien te invitó un enlace nuevo." />
      );
      break;
    case "organization_inactive":
      content = (
        <NoticeMessage message="La cuenta de este restaurante no está activa, así que no se puede aceptar la invitación por ahora. Habla con quien te invitó." />
      );
      break;
    case "email_taken":
      content = (
        <NoticeMessage message={`El correo ${state.email} ya tiene una cuenta.`}>
          {" "}
          <a href="/signin" style={{ color: "inherit", fontWeight: 700, textDecoration: "underline" }}>
            Inicia sesión
          </a>
        </NoticeMessage>
      );
      break;
    case "error":
      content = <NoticeMessage message={state.message} />;
      break;
    case "loaded":
      content = <AcceptForm invitation={state.invitation} token={token} onState={setState} />;
      break;
  }

  return (
    <PublicLayout variant="single" title="Acepta tu invitación">
      <Card>
        <CardBody>{content}</CardBody>
      </Card>
    </PublicLayout>
  );
}
