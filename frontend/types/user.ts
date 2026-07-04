export interface Role {
  id: number;
  name: string;
  created_at?: string;
}

export interface User {
  id: string;
  username: string;
  name: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface UserWithRoles {
  user: User;
  roles: Role[];
}

export interface CreateUserRequest {
  username: string;
  name: string;
  password: string;
  role_ids: number[];
}

export interface UpdateUserRequest {
  name?: string;
  role_ids?: number[];
  active?: boolean;
}

export interface ResetPasswordRequest {
  password: string;
}

export interface UsersListResponse {
  users: UserWithRoles[];
}

export interface RolesListResponse {
  roles: Role[];
}

/** Friendly Spanish labels for the backend role names. */
export const ROLE_LABELS: Record<string, string> = {
  admin: "Administrador",
  manager: "Gerente",
  server: "Mesero",
  cooker: "Cocina",
  accountant: "Contador",
};

export function roleLabel(name: string): string {
  return ROLE_LABELS[name] ?? name;
}
