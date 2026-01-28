"use client";

import { ReactNode } from "react";
import { usePermissions } from "@/lib/permissions";

interface PermissionGateProps {
  /** Single permission required to render children */
  permission?: string;
  /** Multiple permissions (use with requireAll to control logic) */
  permissions?: string[];
  /** If true, require all permissions; if false (default), require any */
  requireAll?: boolean;
  /** Content to render when user has permission */
  children: ReactNode;
  /** Content to render when user lacks permission (defaults to null) */
  fallback?: ReactNode;
}

/**
 * A component that conditionally renders children based on user permissions.
 *
 * @example
 * // Single permission
 * <PermissionGate permission={PERMISSIONS.EXPENSES_CREATE}>
 *   <Button>Create Expense</Button>
 * </PermissionGate>
 *
 * @example
 * // Any of multiple permissions
 * <PermissionGate permissions={[PERMISSIONS.EXPENSES_UPDATE, PERMISSIONS.EXPENSES_DELETE]}>
 *   <ActionMenu />
 * </PermissionGate>
 *
 * @example
 * // All permissions required
 * <PermissionGate
 *   permissions={[PERMISSIONS.EXPENSES_READ, PERMISSIONS.EXPENSES_CREATE]}
 *   requireAll={true}
 * >
 *   <ExpenseManagementPanel />
 * </PermissionGate>
 */
export function PermissionGate({
  permission,
  permissions,
  requireAll = false,
  children,
  fallback = null,
}: PermissionGateProps) {
  const { hasPermission, hasAnyPermission, hasAllPermissions, isLoading } =
    usePermissions();

  // While loading, don't render anything to prevent flash of content
  if (isLoading) {
    return null;
  }

  let hasAccess = false;

  if (permission) {
    hasAccess = hasPermission(permission);
  } else if (permissions && permissions.length > 0) {
    hasAccess = requireAll
      ? hasAllPermissions(permissions)
      : hasAnyPermission(permissions);
  }

  return hasAccess ? <>{children}</> : <>{fallback}</>;
}
