"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import NoticeMessage from "@/components/auth/NoticeMessage";
import CardForm from "@/components/public/CardForm";
import PeriodToggle from "@/components/public/PeriodToggle";
import PlanPicker from "@/components/public/PlanPicker";
import PlanSummary from "@/components/public/PlanSummary";
import PublicLayout from "@/components/public/PublicLayout";
import styles from "@/components/public/signup.module.css";
import { Button } from "@/components/ui";
import type { CardInput } from "@/lib/billing/card";
import {
  CHARGE_DECLINED_MESSAGE,
  CHARGE_PENDING_MESSAGE,
  payOutcome,
  RECOVERY_POLL_MS,
  recoveryView,
  STOPPED_REASONS,
  type RecoveryState,
  type RecoveryView,
} from "@/lib/billing/recovery";
import { CARD_REFUSED_MESSAGE, PERIOD_UNITS, PROVIDER_DOWN_MESSAGE } from "@/lib/billing/setup";
import { savedCardLabel } from "@/lib/billing/subscription";
import { TokenizeError, tokenizeCard } from "@/lib/billing/wompiToken";
import { formatPesos, planPrice, type Period, type Plan, type PlanPrices } from "@/lib/plans";
import { supportEmail, supportMailto } from "@/lib/support";

async function readRecovery(): Promise<RecoveryView> {
  const response = await fetch("/api/billing/recovery", { cache: "no-store" });
  return recoveryView(response.status, await response.json().catch(() => ({})));
}

type Phase = "pay" | "pending" | "declined";

export default function RecoveryPageClient({ prices }: { prices: PlanPrices | null }) {
  const router = useRouter();
  const [state, setState] = useState<RecoveryState | null>(null);
  const [phase, setPhase] = useState<Phase>("pay");
  const [amount, setAmount] = useState<number | null>(null);
  const [choosingPlan, setChoosingPlan] = useState(false);
  const [otherCard, setOtherCard] = useState(false);
  const [busy, setBusy] = useState(false);
  const [paying, setPaying] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const leaving = useRef(false);

  const email = supportEmail();
  const mailto = supportMailto(email, "Ayuda para pagar mi suscripción");

  // Applies what the backend says; true when the page is being left.
  const show = useCallback(
    (view: RecoveryView): boolean => {
      if (view.next === "home") {
        leaving.current = true;
        // A full load: the public pages' content security policy must not follow the person into the app.
        window.location.assign("/home");
        return true;
      }
      if (view.next === "signin") {
        leaving.current = true;
        router.replace(view.expired ? "/signin?aviso=sesion" : "/signin");
        return true;
      }
      if (view.next === "retry") return false;
      setState(view.state);
      setPhase(view.next);
      if (view.next === "declined") setOtherCard(true);
      return false;
    },
    [router]
  );

  useEffect(() => {
    let cancelled = false;
    readRecovery()
      .then((view) => {
        if (cancelled) return;
        if (view.next === "retry") setLoadFailed(true);
        else show(view);
      })
      .catch(() => !cancelled && setLoadFailed(true));
    return () => {
      cancelled = true;
    };
  }, [show]);

  // While the charge is pending the page asks again by itself; the approval's answer signs the person in.
  useEffect(() => {
    if (phase !== "pending") return;
    const timer = setInterval(() => {
      if (leaving.current) return;
      readRecovery()
        .then((view) => {
          if (!leaving.current) show(view);
        })
        .catch(() => undefined);
    }, RECOVERY_POLL_MS);
    return () => clearInterval(timer);
  }, [phase, show]);

  const select = useCallback(
    async (plan: Plan, period: Period) => {
      if (!state) return;
      const previous = state;
      setState({ ...state, plan, period });
      setAmount(null);
      setNotice(null);
      setBusy(true);
      try {
        const response = await fetch("/api/billing/recovery/selection", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ plan, period }),
        });
        const body = await response.json().catch(() => ({}));
        if (response.status === 401 || (response.status === 409 && body.code === "not_stopped")) {
          show({ next: "signin", expired: response.status === 401 });
          return;
        }
        if (!response.ok) {
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
    [state, show]
  );

  const pay = useCallback(
    async (body: Record<string, string> | null): Promise<{ clearCard: boolean }> => {
      const response = await fetch("/api/billing/recovery/pay", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: body ? JSON.stringify(body) : undefined,
      });
      const outcome = payOutcome(response.status, await response.json().catch(() => ({})));
      if (outcome.next === "signin") {
        show(outcome);
        return { clearCard: false };
      }
      if (outcome.next === "retry") {
        setNotice(outcome.message);
        return { clearCard: outcome.clearCard };
      }
      setPhase("pending");
      const view = await readRecovery().catch((): RecoveryView => ({ next: "retry" }));
      show(view);
      return { clearCard: true };
    },
    [show]
  );

  const payWithSavedCard = useCallback(async () => {
    setNotice(null);
    setPaying(true);
    try {
      await pay(null);
    } catch {
      setNotice("No se pudo conectar. Intenta de nuevo.");
    } finally {
      setPaying(false);
    }
  }, [pay]);

  const payWithNewCard = useCallback(
    async (card: CardInput): Promise<{ clearCard: boolean }> => {
      setNotice(null);
      setPaying(true);
      try {
        // Acceptance tokens are single use: every attempt reads fresh ones.
        const fresh = await readRecovery();
        if (fresh.next === "home" || fresh.next === "signin") {
          show(fresh);
          return { clearCard: false };
        }
        const wompi = fresh.next === "retry" ? null : fresh.state.wompi;
        if (!wompi) {
          setNotice(PROVIDER_DOWN_MESSAGE);
          return { clearCard: false };
        }
        const token = await tokenizeCard({ publicKey: wompi.publicKey, apiUrl: wompi.apiUrl }, card);
        return await pay({
          card_token: token.token,
          acceptance_token: wompi.acceptanceToken,
          personal_data_acceptance_token: wompi.personalDataAcceptanceToken,
        });
      } catch (error) {
        const refused = error instanceof TokenizeError && error.kind === "refused";
        setNotice(refused ? CARD_REFUSED_MESSAGE : PROVIDER_DOWN_MESSAGE);
        return { clearCard: refused };
      } finally {
        setPaying(false);
      }
    },
    [pay, show]
  );

  if (!state) {
    return (
      <PublicLayout title="Tu cuenta está detenida">
        {loadFailed ? (
          <NoticeMessage message="No pudimos cargar tu cuenta. Recarga la página para intentar de nuevo." mailto={mailto} />
        ) : (
          <div className={styles.skeleton} role="status" aria-label="Cargando" />
        )}
      </PublicLayout>
    );
  }

  const shownAmount = amount ?? (state.amountDue > 0 ? state.amountDue : planPrice(prices, state.plan, state.period));
  const locked = busy || paying || phase === "pending";
  const showForm = state.cardEntry && state.wompi && (otherCard || !state.card);

  return (
    <PublicLayout
      title="Tu cuenta está detenida"
      subtitle={
        <>
          <strong>{state.organizationName}</strong>: {STOPPED_REASONS[state.reason]}
        </>
      }
    >
      <div className={styles.stack}>
        <section className={styles.card} aria-label="Plan y valor a pagar">
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
              onChange={locked || !state.cardEntry ? undefined : () => setChoosingPlan(true)}
            />
          )}
          {state.cardEntry && (
            <PeriodToggle
              plan={state.plan}
              value={state.period}
              prices={prices}
              disabled={locked}
              onChange={(period) => period !== state.period && select(state.plan, period)}
            />
          )}
          {shownAmount !== null && (
            <div className={styles.charge} aria-live="polite">
              <p className={styles.today} style={{ margin: 0 }}>
                <span>Hoy pagas</span>
                <strong>
                  {formatPesos(shownAmount)} / {PERIOD_UNITS[state.period]}
                </strong>
              </p>
              <p className={styles.muted}>Tu acceso vuelve en cuanto el banco apruebe el pago. El precio no tiene impuestos adicionales.</p>
            </div>
          )}
        </section>

        {phase === "pending" && <NoticeMessage tone="warning" message={CHARGE_PENDING_MESSAGE} />}
        {phase === "declined" && !notice && <NoticeMessage message={CHARGE_DECLINED_MESSAGE} />}
        {notice && <NoticeMessage message={notice} mailto={mailto} />}

        {!state.cardEntry ? (
          <section className={styles.card} aria-labelledby="recovery-support-title">
            <h2 id="recovery-support-title" className={styles.sectionTitle}>
              Escríbenos para reactivar tu cuenta
            </h2>
            <p className={styles.muted}>
              Por ahora los pagos se coordinan con nuestro equipo.
              {email && mailto && (
                <>
                  {" "}
                  Escríbenos a{" "}
                  <a className={styles.link} href={mailto}>
                    {email}
                  </a>
                  .
                </>
              )}
            </p>
          </section>
        ) : (
          phase !== "pending" && (
            <>
              {state.card && !otherCard && (
                <section className={styles.card} aria-label="Pagar con la tarjeta guardada">
                  <Button variant="primary" size="lg" fullWidth onClick={payWithSavedCard} disabled={locked}>
                    {paying ? "Enviando el pago…" : `Pagar con ${savedCardLabel(state.card)}`}
                  </Button>
                  <button type="button" className={styles.linkButton} onClick={() => setOtherCard(true)} disabled={locked}>
                    Usar otra tarjeta
                  </button>
                </section>
              )}
              {showForm && state.wompi && (
                <section className={styles.card} aria-labelledby="recovery-card-title">
                  <h2 id="recovery-card-title" className={styles.sectionTitle}>
                    Tarjeta de crédito o débito
                  </h2>
                  <CardForm
                    termsUrl={state.wompi.termsUrl}
                    personalDataUrl={state.wompi.personalDataUrl}
                    submitting={paying}
                    onSubmit={payWithNewCard}
                    submitLabel={shownAmount !== null ? `Pagar ${formatPesos(shownAmount)}` : "Pagar"}
                    submittingLabel="Enviando el pago…"
                  />
                  {state.card && (
                    <button type="button" className={styles.linkButton} onClick={() => setOtherCard(false)} disabled={locked}>
                      Pagar con {savedCardLabel(state.card)}
                    </button>
                  )}
                </section>
              )}
            </>
          )
        )}
      </div>
    </PublicLayout>
  );
}
