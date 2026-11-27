"use client";

import { useCallback, useEffect, useState } from "react";
import NoticeMessage from "@/components/auth/NoticeMessage";
import { ErrorBanner } from "@/components/deliveries/Banners";
import CardForm from "@/components/public/CardForm";
import SettingsSectionShell from "@/components/settings/SettingsSectionShell";
import { Badge, Button, Card, CardBody, Modal, Table, type BadgeTone } from "@/components/ui";
import type { CardInput } from "@/lib/billing/card";
import { CARD_REFUSED_MESSAGE, parseSetupState, PERIOD_LABELS, PERIOD_UNITS, PROVIDER_DOWN_MESSAGE, setupOutcome } from "@/lib/billing/setup";
import {
  CHARGE_STATUS_LABELS,
  longDate,
  parseSubscription,
  savedCardLabel,
  subscriptionState,
  type ChargeStatus,
  type Subscription,
  type SubscriptionStateKind,
} from "@/lib/billing/subscription";
import { TokenizeError, tokenizeCard } from "@/lib/billing/wompiToken";
import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { PLAN_LABELS, formatPesos } from "@/lib/plans";
import { supportEmail, supportMailto } from "@/lib/support";

const heading = { margin: "0 0 0.75rem", fontSize: "1.125rem", color: "var(--color-text-primary)" } as const;
const muted = { margin: 0, color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;
const facts = { display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: "1rem", margin: 0 } as const;
const factLabel = { margin: 0, fontSize: "0.8rem", color: "var(--color-text-secondary)" } as const;
const factValue = { margin: "0.15rem 0 0", fontSize: "1rem", fontWeight: 600, color: "var(--color-text-primary)" } as const;

const STATE_TONES: Record<SubscriptionStateKind, BadgeTone> = {
  trial: "info",
  paid: "success",
  cancelled: "warning",
  failed: "danger",
  "no-card": "neutral",
};

const CHARGE_TONES: Record<ChargeStatus, BadgeTone> = {
  pending: "info",
  approved: "success",
  declined: "danger",
  voided: "neutral",
  error: "danger",
};

interface CardSetup {
  publicKey: string;
  apiUrl: string;
  termsUrl: string;
  personalDataUrl: string;
  acceptanceToken: string;
  personalDataAcceptanceToken: string;
}

// The card setup has the setup state's `wompi`, without an organization, plan or period.
async function readCardSetup(): Promise<CardSetup | null> {
  const response = await fetch("/api/billing/card-setup", { cache: "no-store" });
  if (!response.ok) return null;
  const body = await response.json().catch(() => ({}));
  return parseSetupState({ organization_name: "", plan: "starter", period: "monthly", ...body })?.wompi ?? null;
}

function Fact({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt style={factLabel}>{label}</dt>
      <dd style={factValue}>{children}</dd>
    </div>
  );
}

export default function SubscriptionPageClient() {
  const { hasPermission, isEdge, isLoading: permissionsLoading, fetchPermissions } = usePermissions();
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [loadError, setLoadError] = useState("");
  const [actionError, setActionError] = useState("");
  const [working, setWorking] = useState(false);
  const [confirmingCancel, setConfirmingCancel] = useState(false);
  const [cardSetup, setCardSetup] = useState<CardSetup | null>(null);
  // The notice's "Cambiar la tarjeta" link arrives with #tarjeta: the form then starts open.
  const [cardToggled, setCardToggled] = useState<boolean | null>(null);
  const [openFromLink] = useState(() => typeof window !== "undefined" && window.location.hash === "#tarjeta");
  const [savingCard, setSavingCard] = useState(false);
  const [cardNotice, setCardNotice] = useState<string | null>(null);

  const allowed = !isEdge && hasPermission(PERMISSIONS.BILLING_MANAGE);
  const email = supportEmail();

  const apply = useCallback(
    async (response: Response): Promise<boolean> => {
      const body = await response.json().catch(() => ({}));
      const parsed = response.ok ? parseSubscription(body) : null;
      if (!parsed) return false;
      setSubscription(parsed);
      // The notice at the top of the page reads the same facts from /auth/me.
      await fetchPermissions();
      return true;
    },
    [fetchPermissions]
  );

  useEffect(() => {
    if (permissionsLoading || !allowed) return;
    let cancelled = false;
    fetch("/api/billing", { cache: "no-store" })
      .then(async (response) => {
        const parsed = response.ok ? parseSubscription(await response.json().catch(() => ({}))) : null;
        if (cancelled) return;
        if (parsed) setSubscription(parsed);
        else setLoadError("No pudimos cargar tu suscripción. Recarga la página para intentar de nuevo.");
      })
      .catch(() => !cancelled && setLoadError("No se pudo conectar. Recarga la página para intentar de nuevo."));
    return () => {
      cancelled = true;
    };
  }, [permissionsLoading, allowed]);

  const changingCard = cardToggled ?? (openFromLink && subscription?.cardEntry === true);

  useEffect(() => {
    if (!changingCard || cardSetup) return;
    let cancelled = false;
    readCardSetup()
      .then((setup) => {
        if (cancelled) return;
        if (setup) setCardSetup(setup);
        else setCardNotice(PROVIDER_DOWN_MESSAGE);
      })
      .catch(() => !cancelled && setCardNotice(PROVIDER_DOWN_MESSAGE));
    return () => {
      cancelled = true;
    };
  }, [changingCard, cardSetup]);

  const act = useCallback(
    async (action: "cancel" | "resume") => {
      setWorking(true);
      setActionError("");
      try {
        const response = await fetch(`/api/billing/${action}`, { method: "POST" });
        if (response.status === 409) {
          const body = await response.clone().json().catch(() => ({}));
          if (body.code === "subscription_lapsed") {
            setActionError("La fecha ya pasó y la suscripción no se puede reanudar. Cierra sesión y vuelve a entrar para pagar.");
            return;
          }
        }
        if (!(await apply(response))) {
          setActionError(action === "cancel" ? "No pudimos cancelar la suscripción. Intenta de nuevo." : "No pudimos reanudar la suscripción. Intenta de nuevo.");
        }
      } catch {
        setActionError("No se pudo conectar. Intenta de nuevo.");
      } finally {
        setWorking(false);
        setConfirmingCancel(false);
      }
    },
    [apply]
  );

  const saveCard = useCallback(
    async (card: CardInput): Promise<{ clearCard: boolean }> => {
      setCardNotice(null);
      setSavingCard(true);
      try {
        // Acceptance tokens are single use: every attempt reads fresh ones.
        const fresh = await readCardSetup();
        if (!fresh) {
          setCardNotice(PROVIDER_DOWN_MESSAGE);
          return { clearCard: false };
        }
        const token = await tokenizeCard({ publicKey: fresh.publicKey, apiUrl: fresh.apiUrl }, card);
        const response = await fetch("/api/billing/card", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            card_token: token.token,
            acceptance_token: fresh.acceptanceToken,
            personal_data_acceptance_token: fresh.personalDataAcceptanceToken,
          }),
        });
        if (!response.ok) {
          const outcome = setupOutcome(response.status, await response.json().catch(() => ({})));
          const message = outcome.next === "retry" ? outcome.message : "No pudimos guardar la tarjeta. Intenta de nuevo.";
          setCardNotice(message);
          return { clearCard: outcome.next === "retry" && outcome.clearCard };
        }
        if (!(await apply(response))) {
          setCardNotice("Guardamos la tarjeta, pero no pudimos leer tu suscripción. Recarga la página.");
          return { clearCard: true };
        }
        setCardToggled(false);
        setCardSetup(null);
        return { clearCard: true };
      } catch (error) {
        const refused = error instanceof TokenizeError && error.kind === "refused";
        setCardNotice(refused ? CARD_REFUSED_MESSAGE : PROVIDER_DOWN_MESSAGE);
        return { clearCard: refused };
      } finally {
        setSavingCard(false);
      }
    },
    [apply]
  );

  if (permissionsLoading) return <SettingsSectionShell title="Suscripción">{null}</SettingsSectionShell>;
  if (!allowed) {
    return (
      <SettingsSectionShell title="Suscripción">
        <ErrorBanner message="Solo quien administra el restaurante puede ver la suscripción." />
      </SettingsSectionShell>
    );
  }
  if (!subscription) {
    return <SettingsSectionShell title="Suscripción">{loadError ? <ErrorBanner message={loadError} /> : <p style={muted}>Cargando…</p>}</SettingsSectionShell>;
  }

  const state = subscriptionState(subscription);
  const due = longDate(subscription.dueDate);
  const lastDay = longDate(subscription.accessEndsAt ?? subscription.dueDate);
  const planMailto = supportMailto(email, "Cambiar de plan o de forma de pago");
  const canCancel = subscription.card !== null && !subscription.cancelled && subscription.dueDate !== null;

  return (
    <SettingsSectionShell title="Suscripción">
      <Card>
        <CardBody>
          <div style={{ display: "flex", flexWrap: "wrap", alignItems: "center", gap: "0.75rem", marginBottom: "1rem" }}>
            <h2 style={{ ...heading, margin: 0 }}>Plan {PLAN_LABELS[subscription.plan]}</h2>
            <Badge tone={STATE_TONES[state.kind]}>{state.label}</Badge>
          </div>
          {state.detail && <p style={{ ...muted, marginBottom: "1rem" }}>{state.detail}</p>}
          <dl style={facts}>
            <Fact label="Forma de pago">{PERIOD_LABELS[subscription.period]}</Fact>
            {subscription.nextChargeAmount !== null && due && (
              <Fact label="Próximo cobro">
                {formatPesos(subscription.nextChargeAmount)} / {PERIOD_UNITS[subscription.period]}, el {due}
              </Fact>
            )}
            {subscription.card && <Fact label="Tarjeta">{savedCardLabel(subscription.card)}</Fact>}
          </dl>
          <p style={{ ...muted, marginTop: "1rem" }}>
            Los precios no incluyen impuestos adicionales. Para cambiar de plan o de forma de pago,{" "}
            {planMailto ? (
              <a href={planMailto} style={{ color: "var(--color-primary)", fontWeight: 600 }}>
                escríbenos{email ? ` a ${email}` : ""}
              </a>
            ) : (
              "escríbenos"
            )}
            .
          </p>
          {actionError && (
            <div style={{ marginTop: "1rem" }}>
              <ErrorBanner message={actionError} />
            </div>
          )}
          {subscription.card && (
            <div style={{ display: "flex", flexWrap: "wrap", gap: "0.75rem", marginTop: "1.25rem" }}>
              {subscription.cardEntry && (
                <Button variant="secondary" onClick={() => setCardToggled(!changingCard)} disabled={working || savingCard}>
                  {changingCard ? "Cerrar" : "Cambiar la tarjeta"}
                </Button>
              )}
              {subscription.cancelled ? (
                <Button onClick={() => act("resume")} disabled={working}>
                  {working ? "Reanudando…" : "Reanudar la suscripción"}
                </Button>
              ) : (
                canCancel && (
                  <Button variant="danger" onClick={() => setConfirmingCancel(true)} disabled={working}>
                    Cancelar la suscripción
                  </Button>
                )
              )}
            </div>
          )}
        </CardBody>
      </Card>

      {!subscription.card && subscription.cardEntry && subscription.status === "trial" && !changingCard && (
        <Card>
          <CardBody>
            <h2 style={heading}>Guarda una tarjeta</h2>
            <p style={{ ...muted, marginBottom: "1rem" }}>
              Con una tarjeta guardada tu plan se cobra solo cuando termina la prueba y no pierdes el acceso.
            </p>
            <Button onClick={() => setCardToggled(true)}>Agregar una tarjeta</Button>
          </CardBody>
        </Card>
      )}

      {changingCard && subscription.cardEntry && (
        <Card id="tarjeta">
          <CardBody>
            <h2 style={heading}>{subscription.card ? "Cambiar la tarjeta" : "Agregar una tarjeta"}</h2>
            {subscription.paymentFailed && (
              <p style={{ ...muted, marginBottom: "1rem" }}>Al guardarla intentamos de inmediato el cobro que falló.</p>
            )}
            {cardNotice && (
              <div style={{ marginBottom: "1rem" }}>
                <NoticeMessage message={cardNotice} />
              </div>
            )}
            {cardSetup ? (
              <CardForm
                termsUrl={cardSetup.termsUrl}
                personalDataUrl={cardSetup.personalDataUrl}
                submitting={savingCard}
                onSubmit={saveCard}
                submitLabel="Guardar tarjeta"
              />
            ) : (
              !cardNotice && <p style={muted}>Cargando…</p>
            )}
          </CardBody>
        </Card>
      )}

      {subscription.card && (
        <Card>
          <CardBody>
            <h2 style={heading}>Cobros</h2>
            {subscription.charges.length === 0 ? (
              <p style={muted}>Todavía no se ha hecho ningún cobro.</p>
            ) : (
              <Table>
                <thead>
                  <tr>
                    <th>Fecha</th>
                    <th data-numeric>Valor</th>
                    <th>Estado</th>
                    <th>Periodo que cubre</th>
                  </tr>
                </thead>
                <tbody>
                  {subscription.charges.map((charge) => {
                    const from = longDate(charge.coversFrom);
                    const until = longDate(charge.coversUntil);
                    return (
                      <tr key={charge.id}>
                        <td>{longDate(charge.date) ?? "—"}</td>
                        <td data-numeric>{formatPesos(charge.amount)}</td>
                        <td>
                          <Badge tone={CHARGE_TONES[charge.status]}>{CHARGE_STATUS_LABELS[charge.status]}</Badge>
                        </td>
                        <td>{from && until ? `${from} al ${until}` : "—"}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </Table>
            )}
          </CardBody>
        </Card>
      )}

      <Modal
        open={confirmingCancel}
        onClose={() => !working && setConfirmingCancel(false)}
        title="¿Cancelar la suscripción?"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirmingCancel(false)} disabled={working}>
              Volver
            </Button>
            <Button variant="danger" onClick={() => act("cancel")} disabled={working}>
              {working ? "Cancelando…" : "Sí, cancelar"}
            </Button>
          </>
        }
      >
        <p style={{ margin: 0, color: "var(--color-text-primary)" }}>
          {lastDay ? (
            <>
              Tu restaurante puede seguir usando Kuenta hasta el <strong>{lastDay}</strong>. Después de ese día el acceso se detiene y no se
              hace ningún cobro más.
            </>
          ) : (
            "No se hará ningún cobro más."
          )}
        </p>
        <p style={{ ...muted, marginTop: "0.75rem" }}>Tu tarjeta queda guardada y puedes reanudar la suscripción antes de esa fecha.</p>
      </Modal>
    </SettingsSectionShell>
  );
}
