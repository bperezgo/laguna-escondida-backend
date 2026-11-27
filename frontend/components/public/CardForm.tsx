"use client";

import { useState, type FormEvent } from "react";
import { LockSimple } from "@phosphor-icons/react";
import { Button, Input } from "@/components/ui";
import { CARD_BRAND_LABELS, cardBrand, formatCardNumber, formatExpiry, validateCard, type CardErrors, type CardInput } from "@/lib/billing/card";
import styles from "./signup.module.css";

export interface CardConsent {
  terms: boolean;
  personalData: boolean;
}

export interface CardFormProps {
  termsUrl: string;
  personalDataUrl: string;
  submitting: boolean;
  /** Resolves true when the card was saved; false keeps the holder's name and clears the rest. */
  onSubmit: (card: CardInput) => Promise<{ clearCard: boolean }>;
}

const EMPTY: CardInput = { number: "", expiry: "", cvc: "", holder: "" };

// The inputs carry no `name`, and the form never posts: the card leaves this page only inside
// the encrypted request to Wompi.
export default function CardForm({ termsUrl, personalDataUrl, submitting, onSubmit }: CardFormProps) {
  const [card, setCard] = useState<CardInput>(EMPTY);
  const [consent, setConsent] = useState<CardConsent>({ terms: false, personalData: false });
  const [errors, setErrors] = useState<CardErrors & { terms?: string; personalData?: string }>({});
  const brand = cardBrand(card.number);

  const set = (field: keyof CardInput, value: string) => {
    setCard((prev) => ({ ...prev, [field]: value }));
    setErrors((prev) => ({ ...prev, [field]: undefined }));
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const found: typeof errors = validateCard(card, new Date());
    if (!consent.terms) found.terms = "Acepta los términos de Wompi para continuar.";
    if (!consent.personalData) found.personalData = "Autoriza el tratamiento de tus datos para continuar.";
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    const { clearCard } = await onSubmit(card);
    if (clearCard) setCard((prev) => ({ ...EMPTY, holder: prev.holder }));
  };

  return (
    <form className={styles.form} onSubmit={handleSubmit} noValidate autoComplete="on">
      <div className={styles.numberWrap}>
        <Input
          label="Número de la tarjeta"
          id="card-number"
          inputMode="numeric"
          autoComplete="cc-number"
          placeholder="1234 5678 9012 3456"
          value={card.number}
          onChange={(e) => set("number", formatCardNumber(e.target.value))}
          error={errors.number}
          disabled={submitting}
        />
        {brand && <span className={styles.brandTag}>{CARD_BRAND_LABELS[brand]}</span>}
      </div>
      <div className={styles.row}>
        <Input
          label="Vencimiento"
          id="card-expiry"
          inputMode="numeric"
          autoComplete="cc-exp"
          placeholder="MM/AA"
          value={card.expiry}
          onChange={(e) => set("expiry", formatExpiry(e.target.value))}
          error={errors.expiry}
          disabled={submitting}
        />
        <Input
          label="Código de seguridad"
          id="card-cvc"
          type="password"
          inputMode="numeric"
          autoComplete="cc-csc"
          placeholder="CVC"
          maxLength={4}
          value={card.cvc}
          onChange={(e) => set("cvc", e.target.value.replace(/\D/g, "").slice(0, 4))}
          error={errors.cvc}
          disabled={submitting}
        />
      </div>
      <Input
        label="Nombre en la tarjeta"
        id="card-holder"
        autoComplete="cc-name"
        placeholder="Como aparece en la tarjeta"
        value={card.holder}
        onChange={(e) => set("holder", e.target.value)}
        error={errors.holder}
        disabled={submitting}
      />

      <label className={styles.consent}>
        <input
          type="checkbox"
          checked={consent.terms}
          disabled={submitting}
          aria-invalid={!!errors.terms}
          onChange={(e) => {
            setConsent((prev) => ({ ...prev, terms: e.target.checked }));
            setErrors((prev) => ({ ...prev, terms: undefined }));
          }}
        />
        <span>
          Acepto los{" "}
          <a className={styles.link} href={termsUrl} target="_blank" rel="noopener noreferrer">
            términos y condiciones de Wompi
          </a>
          .
        </span>
      </label>
      {errors.terms && (
        <p className={styles.consentError} role="alert">
          {errors.terms}
        </p>
      )}
      <label className={styles.consent}>
        <input
          type="checkbox"
          checked={consent.personalData}
          disabled={submitting}
          aria-invalid={!!errors.personalData}
          onChange={(e) => {
            setConsent((prev) => ({ ...prev, personalData: e.target.checked }));
            setErrors((prev) => ({ ...prev, personalData: undefined }));
          }}
        />
        <span>
          Autorizo el{" "}
          <a className={styles.link} href={personalDataUrl} target="_blank" rel="noopener noreferrer">
            tratamiento de mis datos personales
          </a>{" "}
          por parte de Wompi.
        </span>
      </label>
      {errors.personalData && (
        <p className={styles.consentError} role="alert">
          {errors.personalData}
        </p>
      )}

      <Button type="submit" variant="primary" size="lg" fullWidth disabled={submitting}>
        {submitting ? "Guardando tu tarjeta…" : "Guardar tarjeta y empezar la prueba"}
      </Button>
      <p className={styles.secure}>
        <LockSimple size={15} weight="fill" aria-hidden />
        Pago procesado por Wompi. No guardamos el número de tu tarjeta.
      </p>
    </form>
  );
}
