"use client";

import { ReactNode, useEffect, useRef, useState } from "react";
import Modal from "./Modal";
import Button from "./Button";
import { Input } from "./Field";
import type { ButtonVariant } from "./Button";
import { ORDER_ACTION_PIN } from "@/lib/orders/actionPin";

const PIN_LENGTH = 4;

export interface PinConfirmModalProps {
  open: boolean;
  onClose: () => void;
  /** Called only once the correct PIN has been entered and confirmed. */
  onConfirm: () => void;
  title?: string;
  message?: ReactNode;
  confirmLabel?: string;
  confirmVariant?: ButtonVariant;
  /** Disables input/close while the confirmed action is in flight. */
  isProcessing?: boolean;
}

export default function PinConfirmModal({
  open,
  onClose,
  onConfirm,
  title = "Confirmar acción",
  message,
  confirmLabel = "Confirmar",
  confirmVariant = "primary",
  isProcessing = false,
}: PinConfirmModalProps) {
  const [entered, setEntered] = useState("");
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Focus the field when the modal opens (deferred so the portal is mounted).
  useEffect(() => {
    if (!open) return;
    const id = setTimeout(() => inputRef.current?.focus(), 0);
    return () => clearTimeout(id);
  }, [open]);

  const complete = entered.length === PIN_LENGTH;

  const reset = () => {
    setEntered("");
    setError(null);
  };

  // Clear any typed digits on every close path (Cancel / overlay / Escape) so a
  // stale PIN never lingers into the next time the modal opens.
  const handleClose = () => {
    reset();
    onClose();
  };

  const handleChange = (value: string) => {
    if (isProcessing) return;
    setError(null);
    // Digits only, capped at the PIN length.
    setEntered(value.replace(/\D/g, "").slice(0, PIN_LENGTH));
  };

  const confirm = () => {
    if (!complete || isProcessing) return;
    if (entered !== ORDER_ACTION_PIN) {
      setError("PIN incorrecto. Intenta de nuevo.");
      setEntered("");
      return;
    }
    reset();
    onConfirm();
  };

  return (
    <Modal
      open={open}
      onClose={handleClose}
      title={title}
      size="sm"
      closeOnOverlay={!isProcessing}
      footer={
        <>
          <Button
            variant="secondary"
            onClick={handleClose}
            disabled={isProcessing}
          >
            Cancelar
          </Button>
          <Button
            variant={confirmVariant}
            onClick={confirm}
            disabled={!complete || isProcessing}
          >
            {isProcessing ? "Procesando..." : confirmLabel}
          </Button>
        </>
      }
    >
      {message && (
        <div
          style={{
            marginBottom: "1rem",
            color: "var(--color-text-secondary)",
            fontSize: "1rem",
          }}
        >
          {message}
        </div>
      )}

      <Input
        ref={inputRef}
        label="PIN"
        type="password"
        inputMode="numeric"
        autoComplete="off"
        maxLength={PIN_LENGTH}
        placeholder="••••"
        value={entered}
        error={error ?? undefined}
        disabled={isProcessing}
        onChange={(e) => handleChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            confirm();
          }
        }}
      />
    </Modal>
  );
}
