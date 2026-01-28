/**
 * Permission system types
 */

export interface UserRole {
  id: number;
  name: string;
  created_at?: string;
}

export interface AuthUser {
  id: string;
  username: string;
  name?: string;
  roles: UserRole[];
  permissions: string[];
}

export interface PermissionsContextValue {
  user: AuthUser | null;
  permissions: string[];
  isLoading: boolean;
  error: string | null;
  hasPermission: (permission: string) => boolean;
  hasAnyPermission: (permissions: string[]) => boolean;
  hasAllPermissions: (permissions: string[]) => boolean;
  fetchPermissions: () => Promise<void>;
}
