"use client";

import { Table, Badge, Button } from "@/components/ui";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";
import { roleLabel } from "@/types/user";
import type { UserWithRoles } from "@/types/user";

interface UserListProps {
  users: UserWithRoles[];
  currentUserId?: string;
  onEdit: (user: UserWithRoles) => void;
  onResetPassword: (user: UserWithRoles) => void;
  onToggleActive: (user: UserWithRoles) => void;
  onDelete: (user: UserWithRoles) => void;
  isLoading?: boolean;
}

export default function UserList({
  users,
  currentUserId,
  onEdit,
  onResetPassword,
  onToggleActive,
  onDelete,
  isLoading = false,
}: UserListProps) {
  if (isLoading) {
    return (
      <div
        style={{
          padding: "3rem",
          textAlign: "center",
          color: "var(--color-text-secondary)",
        }}
      >
        Cargando usuarios...
      </div>
    );
  }

  if (users.length === 0) {
    return (
      <div
        style={{
          padding: "3rem",
          textAlign: "center",
          color: "var(--color-text-secondary)",
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
        }}
      >
        No hay usuarios para mostrar.
      </div>
    );
  }

  return (
    <Table>
      <thead>
        <tr>
          <th>Usuario</th>
          <th>Nombre</th>
          <th>Roles</th>
          <th>Estado</th>
          <th style={{ textAlign: "right" }}>Acciones</th>
        </tr>
      </thead>
      <tbody>
        {users.map(({ user, roles }) => {
          const isSelf = user.id === currentUserId;
          return (
            <tr key={user.id}>
              <td style={{ fontWeight: 600 }}>{user.username}</td>
              <td>{user.name}</td>
              <td>
                <div style={{ display: "flex", flexWrap: "wrap", gap: "0.35rem" }}>
                  {roles.map((role) => (
                    <Badge key={role.id} tone="info" dot={false}>
                      {roleLabel(role.name)}
                    </Badge>
                  ))}
                </div>
              </td>
              <td>
                <Badge tone={user.active ? "success" : "neutral"}>
                  {user.active ? "Activo" : "Inactivo"}
                </Badge>
              </td>
              <td>
                <div
                  style={{
                    display: "flex",
                    gap: "0.4rem",
                    justifyContent: "flex-end",
                    flexWrap: "wrap",
                  }}
                >
                  <PermissionGate permission={PERMISSIONS.USERS_UPDATE}>
                    <Button variant="secondary" size="sm" onClick={() => onEdit({ user, roles })}>
                      Editar
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onResetPassword({ user, roles })}
                    >
                      Contraseña
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={isSelf}
                      title={isSelf ? "No puedes desactivar tu propia cuenta" : undefined}
                      onClick={() => onToggleActive({ user, roles })}
                    >
                      {user.active ? "Desactivar" : "Activar"}
                    </Button>
                  </PermissionGate>
                  <PermissionGate permission={PERMISSIONS.USERS_DELETE}>
                    <Button
                      variant="danger"
                      size="sm"
                      disabled={isSelf}
                      title={isSelf ? "No puedes eliminar tu propia cuenta" : undefined}
                      onClick={() => onDelete({ user, roles })}
                    >
                      Eliminar
                    </Button>
                  </PermissionGate>
                </div>
              </td>
            </tr>
          );
        })}
      </tbody>
    </Table>
  );
}
