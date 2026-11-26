"use client";

import { useCallback, useState } from "react";
import { useRouter } from "next/navigation";
import { Card, CardBody, Input } from "@/components/ui";
import GoogleSignInButton from "@/components/auth/GoogleSignInButton";
import NoticeMessage from "@/components/auth/NoticeMessage";
import PublicShell from "@/components/auth/PublicShell";
import { usePermissions } from "@/lib/permissions";
import { googleClientId } from "@/lib/google";
import { PLAN_CARDS, type Plan } from "@/lib/plans";
import { signupBody, signupFailure, validateSignup, type SignupErrors, type SignupFailure } from "@/lib/signup";
import { supportEmail } from "@/lib/support";

export default function SignupPageClient() {
  const router = useRouter();
  const { fetchPermissions } = usePermissions();
  const [restaurantName, setRestaurantName] = useState("");
  const [plan, setPlan] = useState<Plan | null>(null);
  const [errors, setErrors] = useState<SignupErrors>({});
  const [failure, setFailure] = useState<SignupFailure | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const ready = Object.keys(validateSignup({ restaurantName, plan })).length === 0;

  const handleCredential = useCallback(
    async (idToken: string) => {
      const current = { restaurantName, plan };
      const found = validateSignup(current);
      setErrors(found);
      setFailure(null);
      if (Object.keys(found).length > 0) return;
      setIsLoading(true);
      try {
        const response = await fetch("/api/signup/google", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(signupBody(idToken, current)),
        });
        const data = await response.json().catch(() => ({}));
        if (!response.ok) {
          const f = signupFailure(response.status, data, supportEmail());
          if (f.field) setErrors({ [f.field]: f.message });
          setFailure(f);
          return;
        }
        await fetchPermissions();
        router.push("/home");
      } catch {
        setFailure({ message: "No se pudo conectar. Intenta de nuevo.", mailto: null });
      } finally {
        setIsLoading(false);
      }
    },
    [restaurantName, plan, fetchPermissions, router]
  );

  return (
    <PublicShell subtitle="Crea la cuenta de tu restaurante: 14 días de prueba gratis" maxWidth="56rem">
      <Card>
        <CardBody>
          <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem" }}>
            <Input
              label="Nombre del restaurante"
              id="restaurant_name"
              value={restaurantName}
              onChange={(e) => {
                setRestaurantName(e.target.value);
                setErrors((prev) => ({ ...prev, restaurant_name: undefined }));
              }}
              error={errors.restaurant_name}
              placeholder="Ej. Restaurante Doña Rosa"
              maxLength={255}
              disabled={isLoading}
            />

            <fieldset style={{ border: "none", margin: 0, padding: 0 }}>
              <legend style={{ fontWeight: 600, fontSize: "0.875rem", marginBottom: "0.75rem", color: "var(--color-text-primary)" }}>
                Elige tu plan
              </legend>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: "1rem" }}>
                {PLAN_CARDS.map((card) => {
                  const selected = plan === card.plan;
                  return (
                    <label
                      key={card.plan}
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        gap: "0.5rem",
                        padding: "1rem",
                        borderRadius: "var(--radius-md)",
                        border: `2px solid ${selected ? "var(--color-primary)" : "var(--color-border)"}`,
                        backgroundColor: selected ? "var(--color-primary-light)" : "var(--color-surface)",
                        cursor: isLoading ? "default" : "pointer",
                      }}
                    >
                      <span style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
                        <input
                          type="radio"
                          name="plan"
                          value={card.plan}
                          checked={selected}
                          disabled={isLoading}
                          onChange={() => {
                            setPlan(card.plan);
                            setErrors((prev) => ({ ...prev, plan: undefined }));
                          }}
                        />
                        <strong style={{ color: "var(--color-text-primary)" }}>{card.title}</strong>
                      </span>
                      <ul style={{ margin: 0, paddingLeft: "1.25rem", color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
                        {card.includes.map((item) => (
                          <li key={item}>{item}</li>
                        ))}
                      </ul>
                    </label>
                  );
                })}
              </div>
              {errors.plan && (
                <p style={{ margin: "0.5rem 0 0", color: "var(--color-danger)", fontSize: "0.8rem" }}>{errors.plan}</p>
              )}
            </fieldset>

            {failure && <NoticeMessage message={failure.message} mailto={failure.mailto} />}

            {!googleClientId() ? (
              <NoticeMessage tone="warning" message="El registro no está disponible por ahora." />
            ) : ready ? (
              <div style={{ maxWidth: "26rem", width: "100%", margin: "0 auto" }}>
                <GoogleSignInButton text="signup_with" onCredential={handleCredential} disabled={isLoading} />
              </div>
            ) : (
              <p style={{ margin: 0, textAlign: "center", color: "var(--color-text-secondary)", fontSize: "0.875rem" }}>
                Escribe el nombre del restaurante y elige un plan para continuar con Google.
              </p>
            )}

            <p style={{ margin: 0, textAlign: "center", fontSize: "0.875rem", color: "var(--color-text-secondary)" }}>
              ¿Ya tienes cuenta? <a href="/signin" style={{ color: "var(--color-primary)" }}>Inicia sesión</a>
            </p>
          </div>
        </CardBody>
      </Card>
    </PublicShell>
  );
}
