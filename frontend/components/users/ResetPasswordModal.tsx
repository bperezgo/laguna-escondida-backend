"use client";

import { useState } from "react";
import { Modal, Input, Button } from "@/components/ui";
import type { UserWithRoles } from "@/types/user";

interface ResetPasswordModalProps {
  user: UserWithRoles;
  onSubmit: (password: string) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

export default function ResetPasswordModal({
  user,
  onSubmit,
  onCancel,
  isLoading = false,
}: ResetPasswordModalProps) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = (): boolean => {
    const next: Record<string, string> = {};
    if (!password) {
      next.password = "La contraseña es requerida";
    } else if (password.length < 6) {
      next.password = "La contraseña debe tener al menos 6 caracteres";
    }
    if (password !== confirmPassword) {
      next.confirmPassword = "Las contraseñas no coinciden";
    }
    setErrors(next);
    return Object.keys(next).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    await onSubmit(password);
  };

  return (
    <Modal
      open
      onClose={onCancel}
      title="Restablecer contraseña"
      size="sm"
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancelar
          </Button>
          <Button type="submit" form="reset-password-form" disabled={isLoading}>
            {isLoading ? "Guardando..." : "Restablecer"}
          </Button>
        </>
      }
    >
      <form
        id="reset-password-form"
        onSubmit={handleSubmit}
        style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}
      >
        <p style={{ margin: 0, color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
          Nueva contraseña para <strong>{user.user.username}</strong>.
        </p>

        <div>
          <Input
            label="Nueva contraseña *"
            type={showPassword ? "text" : "password"}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            error={errors.password}
            placeholder="Mínimo 6 caracteres"
            autoComplete="new-password"
          />
          <button
            type="button"
            onClick={() => setShowPassword((s) => !s)}
            style={{
              marginTop: "0.4rem",
              background: "none",
              border: "none",
              padding: 0,
              color: "var(--color-primary)",
              cursor: "pointer",
              fontSize: "0.8rem",
            }}
          >
            {showPassword ? "Ocultar" : "Mostrar"} contraseña
          </button>
        </div>

        <Input
          label="Confirmar contraseña *"
          type={showPassword ? "text" : "password"}
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          error={errors.confirmPassword}
          placeholder="Repite la contraseña"
          autoComplete="new-password"
        />
      </form>
    </Modal>
  );
}
