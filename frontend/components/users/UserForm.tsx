"use client";

import { useState } from "react";
import { Modal, Input, Button } from "@/components/ui";
import { roleLabel } from "@/types/user";
import type {
  Role,
  UserWithRoles,
  CreateUserRequest,
  UpdateUserRequest,
} from "@/types/user";

interface UserFormProps {
  roles: Role[];
  /** When present the form is in edit mode (username is fixed, no password field). */
  user?: UserWithRoles | null;
  /** True when editing your own account — disables the active toggle. */
  isSelf?: boolean;
  onCreate: (data: CreateUserRequest) => Promise<void>;
  onUpdate: (data: UpdateUserRequest) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
}

function generatePassword(): string {
  const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  let out = "";
  const values = new Uint32Array(12);
  crypto.getRandomValues(values);
  for (let i = 0; i < values.length; i++) {
    out += chars[values[i] % chars.length];
  }
  return out;
}

export default function UserForm({
  roles,
  user,
  isSelf = false,
  onCreate,
  onUpdate,
  onCancel,
  isLoading = false,
}: UserFormProps) {
  const isEdit = !!user;

  const [username, setUsername] = useState(user?.user.username ?? "");
  const [name, setName] = useState(user?.user.name ?? "");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [active, setActive] = useState<boolean>(user?.user.active ?? true);
  const [selectedRoleIds, setSelectedRoleIds] = useState<number[]>(
    user?.roles.map((r) => r.id) ?? []
  );
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const toggleRole = (id: number) => {
    setSelectedRoleIds((prev) =>
      prev.includes(id) ? prev.filter((r) => r !== id) : [...prev, id]
    );
    setErrors((prev) => {
      const next = { ...prev };
      delete next.roles;
      return next;
    });
  };

  const handleGenerate = () => {
    const pwd = generatePassword();
    setPassword(pwd);
    setConfirmPassword(pwd);
    setShowPassword(true);
    setErrors((prev) => {
      const next = { ...prev };
      delete next.password;
      delete next.confirmPassword;
      return next;
    });
  };

  const validate = (): boolean => {
    const next: Record<string, string> = {};

    if (!isEdit) {
      if (!username.trim()) {
        next.username = "El usuario es requerido";
      } else if (username.trim().length < 3) {
        next.username = "El usuario debe tener al menos 3 caracteres";
      }
      if (!password) {
        next.password = "La contraseña es requerida";
      } else if (password.length < 6) {
        next.password = "La contraseña debe tener al menos 6 caracteres";
      }
      if (password !== confirmPassword) {
        next.confirmPassword = "Las contraseñas no coinciden";
      }
    }

    if (!name.trim()) {
      next.name = "El nombre es requerido";
    }

    if (selectedRoleIds.length === 0) {
      next.roles = "Selecciona al menos un rol";
    }

    setErrors(next);
    return Object.keys(next).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;

    if (isEdit) {
      await onUpdate({
        name: name.trim(),
        role_ids: selectedRoleIds,
        active,
      });
    } else {
      await onCreate({
        username: username.trim(),
        name: name.trim(),
        password,
        role_ids: selectedRoleIds,
      });
    }
  };

  return (
    <Modal
      open
      onClose={onCancel}
      title={isEdit ? "Editar Usuario" : "Nuevo Usuario"}
      footer={
        <>
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            Cancelar
          </Button>
          <Button type="submit" form="user-form" disabled={isLoading}>
            {isLoading
              ? "Guardando..."
              : isEdit
              ? "Actualizar Usuario"
              : "Crear Usuario"}
          </Button>
        </>
      }
    >
      <form
        id="user-form"
        onSubmit={handleSubmit}
        style={{ display: "flex", flexDirection: "column", gap: "1.25rem" }}
      >
        <Input
          label="Usuario *"
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          error={errors.username}
          placeholder="Nombre de usuario para iniciar sesión"
          autoComplete="off"
          disabled={isEdit}
          helper={
            isEdit ? "El nombre de usuario no se puede cambiar" : undefined
          }
        />

        <Input
          label="Nombre *"
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={errors.name}
          placeholder="Nombre completo"
          maxLength={255}
        />

        {!isEdit && (
          <>
            <div>
              <Input
                label="Contraseña *"
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                error={errors.password}
                placeholder="Mínimo 6 caracteres"
                autoComplete="new-password"
              />
              <div
                style={{
                  display: "flex",
                  gap: "1rem",
                  marginTop: "0.4rem",
                }}
              >
                <button
                  type="button"
                  onClick={() => setShowPassword((s) => !s)}
                  style={{
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
                <button
                  type="button"
                  onClick={handleGenerate}
                  style={{
                    background: "none",
                    border: "none",
                    padding: 0,
                    color: "var(--color-primary)",
                    cursor: "pointer",
                    fontSize: "0.8rem",
                  }}
                >
                  Generar contraseña
                </button>
              </div>
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
          </>
        )}

        {/* Roles */}
        <div>
          <label
            style={{
              display: "block",
              marginBottom: "0.5rem",
              fontWeight: 500,
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
            }}
          >
            Roles *
          </label>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(150px, 1fr))",
              gap: "0.5rem",
            }}
          >
            {roles.map((role) => {
              const checked = selectedRoleIds.includes(role.id);
              return (
                <label
                  key={role.id}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.5rem",
                    padding: "0.6rem 0.75rem",
                    borderRadius: "var(--radius-sm)",
                    border: `1px solid ${
                      checked ? "var(--color-primary)" : "var(--color-border)"
                    }`,
                    backgroundColor: checked
                      ? "var(--color-primary-light)"
                      : "var(--color-surface)",
                    cursor: "pointer",
                    fontSize: "0.875rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleRole(role.id)}
                  />
                  {roleLabel(role.name)}
                </label>
              );
            })}
          </div>
          {errors.roles && (
            <p
              style={{
                margin: "0.4rem 0 0 0",
                color: "var(--color-danger)",
                fontSize: "0.8rem",
              }}
            >
              {errors.roles}
            </p>
          )}
        </div>

        {/* Active toggle (edit only) */}
        {isEdit && (
          <label
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
              opacity: isSelf ? 0.6 : 1,
            }}
          >
            <input
              type="checkbox"
              checked={active}
              disabled={isSelf}
              onChange={(e) => setActive(e.target.checked)}
            />
            Usuario activo
            {isSelf && (
              <span
                style={{ color: "var(--color-text-muted)", fontSize: "0.75rem" }}
              >
                (no puedes desactivar tu propia cuenta)
              </span>
            )}
          </label>
        )}
      </form>
    </Modal>
  );
}
