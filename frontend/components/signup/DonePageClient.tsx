"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import PublicLayout from "@/components/public/PublicLayout";
import styles from "@/components/public/signup.module.css";
import buttonStyles from "@/components/ui/Button.module.css";
import { cardLabel } from "@/lib/billing/card";
import { formatLongDate, parseSignupSummary, PERIOD_UNITS, SUMMARY_KEY, type SignupSummary } from "@/lib/billing/setup";
import { usePermissions } from "@/lib/permissions";
import { PLAN_LABELS, formatPesos, isPlan, planPrice, type PlanPrices } from "@/lib/plans";

const INSTALL_BOX_PATH = "/ajustes/instalar-caja";

export default function DonePageClient({ prices }: { prices: PlanPrices | null }) {
  const router = useRouter();
  const { organization, isLoading } = usePermissions();
  // Read once on the client; the first paint is the loading state either way, so hydration agrees.
  const [summary] = useState<SignupSummary | null>(() => {
    try {
      return typeof window === "undefined" ? null : parseSignupSummary(sessionStorage.getItem(SUMMARY_KEY));
    } catch {
      return null;
    }
  });

  const onTrial = organization?.status === "trial" && !!organization.trial_ends_at;

  useEffect(() => {
    if (!isLoading && !onTrial) router.replace("/home");
  }, [isLoading, onTrial, router]);

  if (isLoading || !onTrial || !organization?.trial_ends_at) {
    return (
      <PublicLayout step={3} title="Tu prueba empezó">
        <div className={styles.skeleton} role="status" aria-label="Cargando" />
      </PublicLayout>
    );
  }

  const trialEnds = formatLongDate(new Date(organization.trial_ends_at));
  const plan = isPlan(organization.plan) ? organization.plan : summary?.plan ?? null;
  const amount = summary ? summary.amount ?? (plan ? planPrice(prices, plan, summary.period) : null) : null;
  const card = cardLabel(summary?.cardBrand, summary?.cardLastFour);

  return (
    <PublicLayout
      step={3}
      title="Tu prueba empezó"
      subtitle={
        <>
          <strong>{organization.display_name || organization.name}</strong> ya puede usar Kuenta. Instala la caja en el computador del
          restaurante para empezar a tomar órdenes.
        </>
      }
    >
      <div className={styles.stack}>
        <section className={styles.card} aria-label="Resumen">
          <dl className={styles.facts}>
            <div className={styles.fact}>
              <dt>Tu prueba termina</dt>
              <dd>{trialEnds}</dd>
            </div>
            {plan && (
              <div className={styles.fact}>
                <dt>Plan</dt>
                <dd>{PLAN_LABELS[plan]}</dd>
              </div>
            )}
            {summary && amount !== null && (
              <div className={styles.fact}>
                <dt>Primer cobro</dt>
                <dd>
                  {formatPesos(amount)} / {PERIOD_UNITS[summary.period]}, el {trialEnds}
                </dd>
              </div>
            )}
            {card && (
              <div className={styles.fact}>
                <dt>Tarjeta</dt>
                <dd>{card}</dd>
              </div>
            )}
          </dl>
        </section>
        {/* Plain links: a full load leaves the public pages' content security policy behind. */}
        <div className={styles.actions}>
          <a className={`${buttonStyles.btn} ${buttonStyles.primary} ${buttonStyles.lg} ${buttonStyles.fullWidth}`} href={INSTALL_BOX_PATH}>
            Instalar la caja
          </a>
          <a className={`${buttonStyles.btn} ${buttonStyles.secondary} ${buttonStyles.lg} ${buttonStyles.fullWidth}`} href="/home">
            Ir a la aplicación
          </a>
        </div>
      </div>
    </PublicLayout>
  );
}
