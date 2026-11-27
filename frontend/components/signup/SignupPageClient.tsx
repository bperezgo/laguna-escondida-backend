"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Input } from "@/components/ui";
import GoogleSignInButton from "@/components/auth/GoogleSignInButton";
import NoticeMessage from "@/components/auth/NoticeMessage";
import PublicLayout from "@/components/public/PublicLayout";
import PlanPicker from "@/components/public/PlanPicker";
import PlanSummary from "@/components/public/PlanSummary";
import styles from "@/components/public/signup.module.css";
import { usePermissions } from "@/lib/permissions";
import { googleClientId } from "@/lib/google";
import { planPrice, type Plan, type PlanPrices } from "@/lib/plans";
import { PAYMENT_PATH, signupBody, signupOutcome, validateSignup, type SignupErrors, type SignupFailure } from "@/lib/signup";
import { supportEmail } from "@/lib/support";

export interface SignupPageClientProps {
  initialPlan: Plan;
  /** Null when prices could not be read: the plans show without them and signup still works. */
  prices: PlanPrices | null;
}

export default function SignupPageClient({ initialPlan, prices }: SignupPageClientProps) {
  const router = useRouter();
  const { fetchPermissions } = usePermissions();
  const [restaurantName, setRestaurantName] = useState("");
  const [plan, setPlan] = useState<Plan>(initialPlan);
  const [choosingPlan, setChoosingPlan] = useState(false);
  const [errors, setErrors] = useState<SignupErrors>({});
  const [failure, setFailure] = useState<SignupFailure | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const handleCredential = useCallback(
    async (idToken: string) => {
      const current = { restaurantName, plan };
      const found = validateSignup(current);
      setErrors(found);
      setFailure(null);
      if (Object.keys(found).length > 0) {
        document.getElementById("restaurant_name")?.focus();
        return;
      }
      setIsLoading(true);
      try {
        const response = await fetch("/api/signup/google", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(signupBody(idToken, current)),
        });
        const data = await response.json().catch(() => ({}));
        const outcome = signupOutcome(response.status, data, supportEmail());
        if (outcome.next === "payment") {
          router.push(PAYMENT_PATH);
          return;
        }
        if (outcome.next === "failure") {
          const f = outcome.failure;
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
    <PublicLayout step={1} title="Crea la cuenta de tu restaurante" subtitle="Empieza con 14 días de prueba gratis. Te toma un par de minutos.">
      <div className={styles.stack}>
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
          autoComplete="organization"
        />

        <section className={styles.card} aria-label="Plan">
          {choosingPlan ? (
            <PlanPicker
              value={plan}
              prices={prices}
              disabled={isLoading}
              onChange={(next) => {
                setPlan(next);
                setChoosingPlan(false);
              }}
            />
          ) : (
            <PlanSummary plan={plan} monthlyPrice={planPrice(prices, plan, "monthly")} onChange={() => setChoosingPlan(true)} />
          )}
        </section>

        {failure && <NoticeMessage message={failure.message} mailto={failure.mailto} />}

        {googleClientId() ? (
          <GoogleSignInButton text="signup_with" onCredential={handleCredential} disabled={isLoading} />
        ) : (
          <NoticeMessage tone="warning" message="El registro no está disponible por ahora." />
        )}

        <p className={`${styles.muted} ${styles.center}`}>
          En el siguiente paso eliges cómo pagar. Hoy no se te cobra nada.
        </p>
        <p className={`${styles.muted} ${styles.center}`}>
          ¿Ya tienes cuenta?{" "}
          <Link className={styles.link} href="/signin">
            Inicia sesión
          </Link>
        </p>
      </div>
    </PublicLayout>
  );
}
