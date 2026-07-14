/**
 * Permission system types
 */

import type { EdgeMode } from "@/types/edge";

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
  /** Deployment mode injected by the Next.js /api/auth/me proxy. */
  deployment_mode?: EdgeMode;
}

export interface PermissionsContextValue {
  user: AuthUser | null;
  permissions: string[];
  isLoading: boolean;
  error: string | null;
  /** True when the current user has the "admin" role. */
  isAdmin: boolean;
  /** True when the frontend is served from the edge node (not cloud). */
  isEdge: boolean;
  hasPermission: (permission: string) => boolean;
  hasAnyPermission: (permissions: string[]) => boolean;
  hasAllPermissions: (permissions: string[]) => boolean;
  fetchPermissions: () => Promise<void>;
}
