"use client";

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  ReactNode,
} from "react";
import { AuthUser, PermissionsContextValue } from "./types";

const PermissionsContext = createContext<PermissionsContextValue | undefined>(
  undefined,
);

interface PermissionsProviderProps {
  children: ReactNode;
}

export function PermissionsProvider({ children }: PermissionsProviderProps) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchPermissions = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch("/api/auth/me");

      if (!response.ok) {
        if (response.status === 401) {
          // Not authenticated - this is expected for unauthenticated users
          setUser(null);
          setPermissions([]);
          return;
        }
        throw new Error("Failed to fetch permissions");
      }

      const data: AuthUser = await response.json();
      setUser(data);
      setPermissions(data.permissions || []);
    } catch (err) {
      console.error("Error fetching permissions:", err);
      setError(err instanceof Error ? err.message : "Unknown error");
      setUser(null);
      setPermissions([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const hasPermission = useCallback(
    (permission: string): boolean => {
      return permissions.includes(permission);
    },
    [permissions],
  );

  const hasAnyPermission = useCallback(
    (perms: string[]): boolean => {
      return perms.some((p) => permissions.includes(p));
    },
    [permissions],
  );

  const hasAllPermissions = useCallback(
    (perms: string[]): boolean => {
      return perms.every((p) => permissions.includes(p));
    },
    [permissions],
  );

  // Fetch permissions on mount
  useEffect(() => {
    fetchPermissions();
  }, [fetchPermissions]);

  const value: PermissionsContextValue = {
    user,
    permissions,
    isLoading,
    error,
    hasPermission,
    hasAnyPermission,
    hasAllPermissions,
    fetchPermissions,
  };

  return (
    <PermissionsContext.Provider value={value}>
      {children}
    </PermissionsContext.Provider>
  );
}

export function usePermissions(): PermissionsContextValue {
  const context = useContext(PermissionsContext);
  if (context === undefined) {
    throw new Error("usePermissions must be used within a PermissionsProvider");
  }
  return context;
}

/**
 * Hook to check if the current user has a specific permission
 */
export function useHasPermission(permission: string): boolean {
  const { hasPermission, isLoading } = usePermissions();
  // Return false while loading to prevent flash of content
  if (isLoading) return false;
  return hasPermission(permission);
}

/**
 * Hook to check if the current user has any of the specified permissions
 */
export function useHasAnyPermission(permissions: string[]): boolean {
  const { hasAnyPermission, isLoading } = usePermissions();
  if (isLoading) return false;
  return hasAnyPermission(permissions);
}
