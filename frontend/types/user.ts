export interface Role {
  id: number;
  name: string;
  created_at?: string;
}

export interface User {
  id: string;
  username: string;
  name: string;
  email: string | null;
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
  email?: string;
  password: string;
  role_ids: number[];
}

export interface UpdateUserRequest {
  name?: string;
  /** Omitted: unchanged. "": removed. */
  email?: string;
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
  agent: "Agente",
  courier: "Repartidor",
};

/** A user holding one of these must have an email. A manager may go without one: a box-only manager. */
export const EMAIL_REQUIRED_ROLE_NAMES = ["admin", "accountant", "agent", "courier"];

export const EMAIL_REQUIRED_MESSAGE =
  "El correo es obligatorio para Administrador, Contador, Agente y Repartidor";

export function requiresEmail(roleNames: string[]): boolean {
  return roleNames.some((name) => EMAIL_REQUIRED_ROLE_NAMES.includes(name));
}

const USER_ERROR_MESSAGES: Record<string, string> = {
  "Email is already in use": "Ese correo ya lo usa otro usuario",
  "email is required for an admin, accountant, agent or courier": EMAIL_REQUIRED_MESSAGE,
  "email is not a valid address": "El correo no es válido",
};

export function userErrorMessage(message: string): string {
  return USER_ERROR_MESSAGES[message] ?? message;
}

export function roleLabel(name: string): string {
  return ROLE_LABELS[name] ?? name;
}
