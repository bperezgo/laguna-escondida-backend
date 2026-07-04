"use client";

import { useState, useEffect } from "react";
import { usersApi, rolesApi } from "@/lib/api/users";
import type {
  Role,
  UserWithRoles,
  CreateUserRequest,
  UpdateUserRequest,
} from "@/types/user";
import { usePermissions, PERMISSIONS } from "@/lib/permissions";
import { PermissionGate } from "@/components/permissions";
import UserList from "./UserList";
import UserForm from "./UserForm";
import ResetPasswordModal from "./ResetPasswordModal";

interface UsersPageClientProps {
  initialUsers: UserWithRoles[];
  initialRoles: Role[];
}

export default function UsersPageClient({
  initialUsers,
  initialRoles,
}: UsersPageClientProps) {
  const { user: currentUser } = usePermissions();

  const [users, setUsers] = useState<UserWithRoles[]>(initialUsers);
  const [roles, setRoles] = useState<Role[]>(initialRoles);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [success, setSuccess] = useState<string>("");

  const [showForm, setShowForm] = useState<boolean>(false);
  const [editingUser, setEditingUser] = useState<UserWithRoles | null>(null);
  const [formLoading, setFormLoading] = useState<boolean>(false);

  const [resetTarget, setResetTarget] = useState<UserWithRoles | null>(null);
  const [resetLoading, setResetLoading] = useState<boolean>(false);

  const [deleteConfirm, setDeleteConfirm] = useState<UserWithRoles | null>(null);

  const flashSuccess = (message: string) => {
    setSuccess(message);
    setTimeout(() => setSuccess(""), 3000);
  };

  const loadUsers = async () => {
    try {
      setLoading(true);
      setError("");
      const fetched = await usersApi.getAll();
      setUsers(fetched);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al cargar usuarios");
    } finally {
      setLoading(false);
    }
  };

  const loadRoles = async () => {
    try {
      const fetched = await rolesApi.getAll();
      setRoles(fetched);
    } catch (err) {
      console.error("Error loading roles:", err);
    }
  };

  // Refresh from the API on mount (SSR data may be stale / empty on 403).
  useEffect(() => {
    if (initialRoles.length === 0) {
      loadRoles();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleCreate = async () => {
    if (roles.length === 0) await loadRoles();
    setEditingUser(null);
    setError("");
    setShowForm(true);
  };

  const handleEdit = async (user: UserWithRoles) => {
    if (roles.length === 0) await loadRoles();
    setEditingUser(user);
    setError("");
    setShowForm(true);
  };

  const handleCancelForm = () => {
    setShowForm(false);
    setEditingUser(null);
    setError("");
  };

  const handleCreateSubmit = async (data: CreateUserRequest) => {
    try {
      setFormLoading(true);
      setError("");
      await usersApi.create(data);
      await loadUsers();
      setShowForm(false);
      flashSuccess("Usuario creado correctamente");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al crear usuario");
    } finally {
      setFormLoading(false);
    }
  };

  const handleUpdateSubmit = async (data: UpdateUserRequest) => {
    if (!editingUser) return;
    try {
      setFormLoading(true);
      setError("");
      await usersApi.update(editingUser.user.id, data);
      await loadUsers();
      setShowForm(false);
      setEditingUser(null);
      flashSuccess("Usuario actualizado correctamente");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al actualizar usuario");
    } finally {
      setFormLoading(false);
    }
  };

  const handleToggleActive = async (user: UserWithRoles) => {
    try {
      setError("");
      await usersApi.update(user.user.id, { active: !user.user.active });
      await loadUsers();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al actualizar usuario");
    }
  };

  const handleResetSubmit = async (password: string) => {
    if (!resetTarget) return;
    try {
      setResetLoading(true);
      setError("");
      await usersApi.resetPassword(resetTarget.user.id, password);
      setResetTarget(null);
      flashSuccess("Contraseña actualizada correctamente");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al restablecer contraseña");
    } finally {
      setResetLoading(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteConfirm) return;
    try {
      setError("");
      await usersApi.remove(deleteConfirm.user.id);
      await loadUsers();
      setDeleteConfirm(null);
      flashSuccess("Usuario eliminado correctamente");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al eliminar usuario");
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "2rem",
      }}
    >
      <div style={{ maxWidth: "1100px", margin: "0 auto" }}>
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "2rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <div>
            <h1
              style={{
                margin: 0,
                fontSize: "2rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Usuarios
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Crear y administrar usuarios del sistema
            </p>
          </div>
          <PermissionGate permission={PERMISSIONS.USERS_CREATE}>
            <button
              onClick={handleCreate}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-primary)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                boxShadow: "var(--shadow-sm)",
              }}
            >
              + Nuevo Usuario
            </button>
          </PermissionGate>
        </div>

        {/* Success Message */}
        {success && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-success-light)",
              color: "var(--color-success)",
              border: "1px solid var(--color-success)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            {success}
          </div>
        )}

        {/* Error Message */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1.5rem",
            }}
          >
            <strong>Error:</strong> {error}
          </div>
        )}

        <UserList
          users={users}
          currentUserId={currentUser?.id}
          onEdit={handleEdit}
          onResetPassword={setResetTarget}
          onToggleActive={handleToggleActive}
          onDelete={setDeleteConfirm}
          isLoading={loading}
        />
      </div>

      {/* Create / Edit Form */}
      {showForm && (
        <UserForm
          roles={roles}
          user={editingUser}
          isSelf={!!editingUser && editingUser.user.id === currentUser?.id}
          onCreate={handleCreateSubmit}
          onUpdate={handleUpdateSubmit}
          onCancel={handleCancelForm}
          isLoading={formLoading}
        />
      )}

      {/* Reset Password */}
      {resetTarget && (
        <ResetPasswordModal
          user={resetTarget}
          onSubmit={handleResetSubmit}
          onCancel={() => setResetTarget(null)}
          isLoading={resetLoading}
        />
      )}

      {/* Delete Confirmation */}
      {deleteConfirm && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            backgroundColor: "var(--color-overlay)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 1000,
            padding: "1rem",
          }}
          role="presentation"
          onClick={() => setDeleteConfirm(null)}
        >
          <div
            role="dialog"
            aria-modal="true"
            onClick={(e) => e.stopPropagation()}
            style={{
              backgroundColor: "var(--color-surface)",
              borderRadius: "var(--radius-md)",
              padding: "2rem",
              maxWidth: "450px",
              width: "100%",
              boxShadow: "var(--shadow-xl)",
              border: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                margin: "0 0 1rem 0",
                fontSize: "1.25rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Confirmar Eliminación
            </h3>
            <p style={{ margin: "0 0 1.5rem 0", color: "var(--color-text-secondary)" }}>
              ¿Estás seguro de que deseas eliminar al usuario{" "}
              <strong>{deleteConfirm.user.username}</strong> ({deleteConfirm.user.name})?
            </p>
            <div style={{ display: "flex", gap: "1rem", justifyContent: "flex-end" }}>
              <button
                onClick={() => setDeleteConfirm(null)}
                style={{
                  padding: "0.75rem 1.5rem",
                  backgroundColor: "var(--color-surface-hover)",
                  color: "var(--color-text-primary)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  fontSize: "1rem",
                  fontWeight: "500",
                }}
              >
                Cancelar
              </button>
              <button
                onClick={handleDeleteConfirm}
                style={{
                  padding: "0.75rem 1.5rem",
                  backgroundColor: "var(--color-danger)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  fontSize: "1rem",
                  fontWeight: "500",
                }}
              >
                Eliminar
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
