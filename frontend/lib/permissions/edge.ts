import { PERMISSIONS } from "./constants.ts";

// The box (edge) only pulls the catalog and users from the cloud: it serves their reads, and
// a write there returns 404, so on the edge the user never holds these permissions.
// Integrations, API keys, notifications and delivery tracking aren't served on the box at all.
export const CLOUD_ONLY_PERMISSIONS: readonly string[] = [
  PERMISSIONS.PRODUCTS_CREATE,
  PERMISSIONS.PRODUCTS_UPDATE,
  PERMISSIONS.PRODUCTS_DELETE,
  PERMISSIONS.USERS_CREATE,
  PERMISSIONS.USERS_UPDATE,
  PERMISSIONS.USERS_DELETE,
  PERMISSIONS.INTEGRATIONS_READ,
  PERMISSIONS.INTEGRATIONS_MANAGE,
  PERMISSIONS.API_KEYS_MANAGE,
  PERMISSIONS.NOTIFICATION_CHANNELS_READ,
  PERMISSIONS.NOTIFICATION_CHANNELS_MANAGE,
  PERMISSIONS.DELIVERIES_READ,
  PERMISSIONS.DELIVERIES_MANAGE,
  PERMISSIONS.DELIVERIES_CARRY,
  PERMISSIONS.EDGE_ENROLL,
];

export function effectivePermissions(permissions: string[], isEdge: boolean): string[] {
  if (!isEdge) return permissions;
  return permissions.filter((p) => !CLOUD_ONLY_PERMISSIONS.includes(p));
}
