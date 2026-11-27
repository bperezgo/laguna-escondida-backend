"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import NoticeMessage from "@/components/auth/NoticeMessage";
import CardForm from "@/components/public/CardForm";
import ChargeSummary from "@/components/public/ChargeSummary";
import PeriodToggle from "@/components/public/PeriodToggle";
import PlanPicker from "@/components/public/PlanPicker";
import PlanSummary from "@/components/public/PlanSummary";
import PublicLayout from "@/components/public/PublicLayout";
import RequestStatus from "@/components/public/RequestStatus";
import styles from "@/components/public/signup.module.css";
import type { CardInput } from "@/lib/billing/card";
import {
  CARD_REFUSED_MESSAGE,
  firstChargeDate,
  formatLongDate,
  parseSetupState,
  PROVIDER_DOWN_MESSAGE,
  setupOutcome,
  SUMMARY_KEY,
  type SetupOutcome,
  type SetupState,
  type SignupSummary,
} from "@/lib/billing/setup";
import { TokenizeError, tokenizeCard } from "@/lib/billing/wompiToken";
import { usePermissions } from "@/lib/permissions";
import { planPrice, type Period, type Plan, type PlanPrices } from "@/lib/plans";
import { DONE_PATH } from "@/lib/signup";
import { supportEmail, supportMailto } from "@/lib/support";

const DEFAULT_TRIAL_DAYS = 14;

async function readSetupState(): Promise<{ state: SetupState | null; outcome: SetupOutcome }> {
  const response = await fetch("/api/billing/setup", { cache: "no-store" });
  const body = await response.json().catch(() => ({}));
  const outcome = setupOutcome(response.status, body);
  return { state: outcome.next === "done" ? parseSetupState(body) : null, outcome };
}

export default function PaymentPageClient({ prices }: { prices: PlanPrices | null }) {
  const router = useRouter();
  const { fetchPermissions } = usePermissions();
  const [state, setState] = useState<SetupState | null>(null);
  const [amount, setAmount] = useState<number | null>(null);
  const [choosingPlan, setChoosingPlan] = useState(false);
  const [busy, setBusy] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);

  const email = supportEmail();
  const mailto = supportMailto(email, "Ayuda con mi registro");

  // Where an answer that ends this step leads; true when it did.
  const leave = useCallback(
    async (outcome: SetupOutcome): Promise<boolean> => {
      if (outcome.next === "signin") {
        router.replace("/signin?aviso=sesion");
        return true;
      }
      if (outcome.next === "home") {
        router.replace("/signin");
        return true;
      }
      return false;
    },
    [router]
  );

  useEffect(() => {
    let cancelled = false;
    readSetupState()
      .then(async ({ state: read, outcome }) => {
        if (cancelled || (await leave(outcome))) return;
        if (read) setState(read);
        else if (outcome.next === "retry") {
          setLoadFailed(true);
          setNotice(outcome.message === CARD_REFUSED_MESSAGE ? PROVIDER_DOWN_MESSAGE : outcome.message);
        } else setLoadFailed(true);
      })
      .catch(() => !cancelled && setLoadFailed(true));
    return () => {
      cancelled = true;
    };
  }, [leave]);

  const select = useCallback(
    async (plan: Plan, period: Period) => {
      if (!state) return;
      const previous = state;
      setState({ ...state, plan, period });
      setAmount(null);
      setNotice(null);
      setBusy(true);
      try {
        const response = await fetch("/api/billing/setup/selection", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ plan, period }),
        });
        const body = await response.json().catch(() => ({}));
        const outcome = setupOutcome(response.status, body);
        if (await leave(outcome)) return;
        if (outcome.next === "retry") {
          setState(previous);
          setNotice("No pudimos guardar el cambio. Intenta de nuevo.");
          return;
        }
        if (typeof body.amount === "number") setAmount(body.amount);
      } catch {
        setState(previous);
        setNotice("No se pudo conectar. Intenta de nuevo.");
      } finally {
        setBusy(false);
      }
    },
    [state, leave]
  );

  const saveCard = useCallback(
    async (card: CardInput): Promise<{ clearCard: boolean }> => {
      if (!state) return { clearCard: false };
      setNotice(null);
      setSaving(true);
      try {
        // Acceptance tokens are single use: every attempt reads fresh ones.
        const fresh = await readSetupState();
        if (await leave(fresh.outcome)) return { clearCard: false };
        const wompi = fresh.state?.wompi;
        if (!wompi) {
          setNotice(PROVIDER_DOWN_MESSAGE);
          return { clearCard: false };
        }
        const token = await tokenizeCard({ publicKey: wompi.publicKey, apiUrl: wompi.apiUrl }, card);
        const response = await fetch("/api/billing/setup/card", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            card_token: token.token,
            acceptance_token: wompi.acceptanceToken,
            personal_data_acceptance_token: wompi.personalDataAcceptanceToken,
          }),
        });
        const outcome = setupOutcome(response.status, await response.json().catch(() => ({})));
        if (await leave(outcome)) return { clearCard: false };
        if (outcome.next === "retry") {
          setNotice(outcome.message);
          return { clearCard: outcome.clearCard };
        }
        const summary: SignupSummary = {
          plan: state.plan,
          period: state.period,
          amount: amount ?? planPrice(prices, state.plan, state.period),
          cardBrand: token.brand,
          cardLastFour: token.lastFour,
        };
        try {
          sessionStorage.setItem(SUMMARY_KEY, JSON.stringify(summary));
        } catch {
          // Step 3 then shows the trial without the card line.
        }
        await fetchPermissions();
        router.push(DONE_PATH);
        return { clearCard: true };
      } catch (error) {
        const refused = error instanceof TokenizeError && error.kind === "refused";
        setNotice(refused ? CARD_REFUSED_MESSAGE : PROVIDER_DOWN_MESSAGE);
        return { clearCard: refused };
      } finally {
        setSaving(false);
      }
    },
    [state, amount, prices, leave, fetchPermissions, router]
  );

  const trialDays = prices?.trialDays ?? DEFAULT_TRIAL_DAYS;
  const firstChargeOn = useMemo(() => formatLongDate(firstChargeDate(new Date(), trialDays)), [trialDays]);

  if (!state) {
    return (
      <PublicLayout step={2} title="Elige cómo pagar" subtitle="Hoy no se te cobra nada.">
        {loadFailed ? (
          <NoticeMessage message={notice ?? "No pudimos cargar tu registro. Recarga la página para intentar de nuevo."} mailto={mailto} />
        ) : (
          <div className={styles.skeleton} role="status" aria-label="Cargando" />
        )}
      </PublicLayout>
    );
  }

  const shownAmount = amount ?? planPrice(prices, state.plan, state.period);
  const locked = busy || saving;

  return (
    <PublicLayout
      step={2}
      title={state.cardEntry ? "Elige cómo pagar" : "Ya casi está"}
      subtitle={
        state.cardEntry ? (
          <>
            Guarda una tarjeta para empezar la prueba de <strong>{state.organizationName}</strong>. Hoy no se te cobra nada.
          </>
        ) : (
          <>
            Recibimos la solicitud de <strong>{state.organizationName}</strong>.
          </>
        )
      }
    >
      <div className={styles.stack}>
        <section className={styles.card} aria-label="Plan y forma de pago">
          {choosingPlan ? (
            <PlanPicker
              value={state.plan}
              prices={prices}
              disabled={locked}
              onChange={(plan) => {
                setChoosingPlan(false);
                if (plan !== state.plan) select(plan, state.period);
              }}
            />
          ) : (
            <PlanSummary
              plan={state.plan}
              monthlyPrice={planPrice(prices, state.plan, "monthly")}
              showIncludes={false}
              onChange={locked ? undefined : () => setChoosingPlan(true)}
            />
          )}
          <PeriodToggle
            plan={state.plan}
            value={state.period}
            prices={prices}
            disabled={locked}
            onChange={(period) => period !== state.period && select(state.plan, period)}
          />
          <ChargeSummary period={state.period} amount={shownAmount} firstChargeOn={firstChargeOn} trialDays={trialDays} />
        </section>

        {notice && <NoticeMessage message={notice} mailto={mailto} />}

        {state.cardEntry && state.wompi ? (
          <section className={styles.card} aria-labelledby="card-title">
            <h2 id="card-title" className={styles.sectionTitle}>
              Tarjeta de crédito o débito
            </h2>
            <CardForm termsUrl={state.wompi.termsUrl} personalDataUrl={state.wompi.personalDataUrl} submitting={saving} onSubmit={saveCard} />
          </section>
        ) : (
          <RequestStatus supportEmail={email} supportMailto={mailto} />
        )}
      </div>
    </PublicLayout>
  );
}
