import { PERMISSIONS } from "./constants.ts";

// The box (edge) only pulls the catalog and users from the cloud: it serves their reads, and
// a write there returns 404, so on the edge the user never holds these permissions.
export const CLOUD_ONLY_PERMISSIONS: readonly string[] = [
  PERMISSIONS.PRODUCTS_CREATE,
  PERMISSIONS.PRODUCTS_UPDATE,
  PERMISSIONS.PRODUCTS_DELETE,
  PERMISSIONS.USERS_CREATE,
  PERMISSIONS.USERS_UPDATE,
  PERMISSIONS.USERS_DELETE,
];

export function effectivePermissions(permissions: string[], isEdge: boolean): string[] {
  if (!isEdge) return permissions;
  return permissions.filter((p) => !CLOUD_ONLY_PERMISSIONS.includes(p));
}
