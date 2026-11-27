"use client";

import { useCallback, useEffect, useState, FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { usePermissions } from "@/lib/permissions";
import { Input, Button } from "@/components/ui";
import {
  LEGACY_EDGE_CONTEXT,
  parseSignInContext,
  signInOutcome,
  type SignInContext,
  type SignInFailure,
} from "@/lib/signIn";
import PublicLayout, { PRODUCT_NAME } from "@/components/public/PublicLayout";
import styles from "@/components/public/signup.module.css";
import { PAYMENT_PATH, SIGNUP_PATH } from "@/lib/signup";
import { supportEmail } from "@/lib/support";
import GoogleSignInButton from "./GoogleSignInButton";
import NoticeMessage from "./NoticeMessage";

export interface SignInFormProps {
  /** Read on the server so the layout is right from the first paint; null when that failed. */
  initialContext?: SignInContext | null;
  /** Why the person was sent here, e.g. an expired setup session. */
  notice?: string | null;
}

export default function SignInForm({ initialContext = null, notice = null }: SignInFormProps) {
  const [context, setContext] = useState<SignInContext | null>(initialContext);
  const [login, setLogin] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<SignInFailure | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const router = useRouter();
  const { fetchPermissions } = usePermissions();

  useEffect(() => {
    if (initialContext) return;
    let cancelled = false;
    fetch("/api/auth/context")
      .then((response) => (response.ok ? response.json() : LEGACY_EDGE_CONTEXT))
      .then((body) => !cancelled && setContext(parseSignInContext(body)))
      .catch(() => !cancelled && setContext(LEGACY_EDGE_CONTEXT));
    return () => {
      cancelled = true;
    };
  }, [initialContext]);

  const isCloud = context?.mode === "cloud";

  const submit = useCallback(
    async (endpoint: string, body: Record<string, string>) => {
      setError(null);
      setIsLoading(true);
      try {
        const response = await fetch(endpoint, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });
        const data = await response.json().catch(() => ({}));
        const outcome = signInOutcome(response.ok, data, supportEmail());
        if (outcome.next === "payment") {
          router.push(PAYMENT_PATH);
          return;
        }
        if (outcome.next === "failure") {
          setError(outcome.failure);
          return;
        }
        await fetchPermissions();
        router.push("/home");
      } catch {
        setError({ message: "No se pudo conectar. Intenta de nuevo.", mailto: null });
      } finally {
        setIsLoading(false);
      }
    },
    [fetchPermissions, router]
  );

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    submit(
      "/api/auth/signin",
      isCloud ? { email: login, password } : { username: login, password }
    );
  };

  const handleGoogle = useCallback(
    (idToken: string) => submit("/api/auth/google", { id_token: idToken }),
    [submit]
  );

  return (
    <PublicLayout
      variant={isCloud ? "split" : "single"}
      title={context?.organization?.name ?? PRODUCT_NAME}
      subtitle="Inicia sesión para continuar"
    >
      <div className={styles.stack}>
        {notice && !error && <NoticeMessage tone="warning" message={notice} />}
        <form onSubmit={handleSubmit} className={styles.stack}>
          {isCloud ? (
            <Input
              key="email"
              label="Correo"
              id="email"
              name="email"
              type="email"
              required
              placeholder="Ingresa tu correo"
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              disabled={isLoading}
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              autoComplete="email"
            />
          ) : (
            <Input
              key="username"
              label="Usuario"
              id="username"
              name="username"
              type="text"
              required
              placeholder="Ingresa tu usuario"
              value={login}
              onChange={(e) => setLogin(e.target.value)}
              disabled={isLoading || !context}
              autoCapitalize="none"
              autoCorrect="off"
              spellCheck={false}
              autoComplete="username"
            />
          )}

          <Input
            label="Contraseña"
            id="password"
            name="password"
            type="password"
            required
            placeholder="Ingresa tu contraseña"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={isLoading}
          />

          {error && <NoticeMessage message={error.message} mailto={error.mailto} />}

          <Button
            type="submit"
            variant="primary"
            fullWidth
            disabled={isLoading || !context}
          >
            {isLoading ? "Iniciando sesión..." : "Iniciar sesión"}
          </Button>
        </form>
        {isCloud && (
          <>
            <div className={styles.divider}>o</div>
            <GoogleSignInButton onCredential={handleGoogle} disabled={isLoading} />
            <p className={`${styles.muted} ${styles.center}`}>
              ¿Tu restaurante aún no tiene cuenta?{" "}
              <Link className={styles.link} href={SIGNUP_PATH}>
                Empieza tu prueba gratis
              </Link>
            </p>
          </>
        )}
      </div>
    </PublicLayout>
  );
}
