"use client";

import { useState, type FormEvent } from "react";
import { PaymentSettingsError, paymentSettingsApi } from "@/lib/api/paymentSettings";
import {
  draftFrom,
  hasPaymentErrors,
  isPaymentDraftChanged,
  MAX_INSTRUCTIONS,
  MAX_MANUAL_WINDOW,
  MIN_MANUAL_WINDOW,
  paymentRequest,
  paymentSaveErrors,
  strategyLabel,
  validatePaymentDraft,
  wompiMissingNotice,
  type PaymentSettingsCardState,
  type PaymentSettingsDraft,
  type PaymentSettingsErrors,
} from "@/lib/settings/paymentSettings";
import type { PaymentSettings, PaymentStrategy } from "@/types/paymentSettings";
import { Button, Card, CardBody, Input, Select, Textarea } from "@/components/ui";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";

interface PaymentSettingsCardProps {
  initialState: PaymentSettingsCardState;
  canManage: boolean;
  wompiConfigured: boolean;
}

const muted = { color: "var(--color-text-secondary)", fontSize: "0.9rem" } as const;

export default function PaymentSettingsCard({ initialState, canManage, wompiConfigured }: PaymentSettingsCardProps) {
  const [saved, setSaved] = useState<PaymentSettings | null>(initialState.kind === "loaded" ? initialState.settings : null);
  const [draft, setDraft] = useState<PaymentSettingsDraft | null>(
    initialState.kind === "loaded" ? draftFrom(initialState.settings) : null
  );
  const [errors, setErrors] = useState<PaymentSettingsErrors>({});
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");

  let body;
  if (initialState.kind === "error" || !saved || !draft) {
    body = <ErrorBanner message={initialState.kind === "error" ? initialState.message : "Error al cargar."} />;
  } else {
    const change = (patch: Partial<PaymentSettingsDraft>) => {
      setDraft({ ...draft, ...patch });
      setNotice("");
    };
    const handleSave = async (event: FormEvent) => {
      event.preventDefault();
      const invalid = validatePaymentDraft(draft);
      if (hasPaymentErrors(invalid)) {
        setErrors(invalid);
        return;
      }
      try {
        setSaving(true);
        setErrors({});
        const next = await paymentSettingsApi.save(paymentRequest(draft));
        setSaved(next);
        setDraft(draftFrom(next));
        setNotice("Guardado. Aplica a los pedidos nuevos; los que ya esperan pago conservan su forma de pago.");
      } catch (err) {
        if (err instanceof PaymentSettingsError) {
          setErrors(paymentSaveErrors(err.status, err.message, err.field));
        } else {
          setErrors({ form: "Error al guardar cómo se pagan los pedidos." });
        }
      } finally {
        setSaving(false);
      }
    };
    const missing = wompiMissingNotice(saved.strategy, wompiConfigured);

    body = (
      <form onSubmit={handleSave} style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
        {notice && <NoticeBanner message={notice} />}
        {errors.form && <ErrorBanner message={errors.form} />}
        {missing && <ErrorBanner message={missing} />}
        <Select
          label="Forma de pago"
          value={draft.strategy}
          disabled={!canManage || saving}
          onChange={(e) => change({ strategy: e.target.value as PaymentStrategy })}
        >
          <option value="wompi">{strategyLabel("wompi")}</option>
          <option value="manual">{strategyLabel("manual")}</option>
        </Select>
        {draft.strategy === "manual" && (
          <>
            <Textarea
              label="Instrucciones de pago"
              rows={3}
              maxLength={MAX_INSTRUCTIONS}
              value={draft.manual_instructions}
              disabled={!canManage || saving}
              onChange={(e) => change({ manual_instructions: e.target.value })}
              error={errors.manual_instructions}
              helper="Lo que el cliente recibe en WhatsApp: llave o cuenta y a nombre de quién. El personal confirma la transferencia y paga el pedido."
            />
            <Input
              label="Minutos para pagar"
              type="number"
              min={MIN_MANUAL_WINDOW}
              max={MAX_MANUAL_WINDOW}
              value={draft.manual_window_minutes}
              disabled={!canManage || saving}
              onChange={(e) => change({ manual_window_minutes: e.target.value })}
              error={errors.manual_window_minutes}
              helper="Pasado este tiempo el pedido se cancela, salvo que el personal lo esté revisando."
            />
          </>
        )}
        {canManage ? (
          <div>
            <Button type="submit" disabled={saving || !isPaymentDraftChanged(draft, saved)}>
              {saving ? "Guardando…" : "Guardar"}
            </Button>
          </div>
        ) : (
          <p style={{ ...muted, margin: 0 }}>Solo un administrador puede cambiar cómo se pagan los pedidos.</p>
        )}
      </form>
    );
  }

  return (
    <Card>
      <CardBody>
        <h2 style={{ margin: "0 0 0.75rem", fontSize: "1.25rem", color: "var(--color-text-primary)" }}>
          Pago de pedidos de WhatsApp
        </h2>
        {body}
      </CardBody>
    </Card>
  );
}
