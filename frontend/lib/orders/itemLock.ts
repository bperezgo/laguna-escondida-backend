import type { AuthUser } from "@/lib/permissions";
import { isManagerialUser } from "./grouping";

/**
 * How long after a line item is first created a waitress may still remove or
 * reduce it. Past this window the line is locked and only a manager/admin can
 * touch it — a guardrail against quietly dropping an old item off a bill.
 *
 * IMPORTANT: this is a FRONTEND-ONLY deterrent. The order update endpoint
 * replaces the whole product list, so it will still accept any payload; a
 * determined user (devtools, direct API call, offline cache) can bypass this.
 * If real loss-prevention is ever needed, the check has to move to the backend.
 */
export const ITEM_EDIT_GRACE_MS = 5 * 60 * 1000;

/**
 * Managers and admins bypass the per-item edit lock entirely. Keyed off the
 * same managerial roles that default to the "Todas" view, so there's a single
 * notion of "supervisor" across the orders screens.
 */
export function canOverrideItemLock(user: AuthUser | null): boolean {
  return isManagerialUser(user);
}

/**
 * Whether a line item is past its edit window. `createdAt` is the item's own
 * creation time (ISO 8601, from the backend). A missing/`null`/unparseable
 * value means we can't age the item — a brand-new session item, or a response
 * that didn't include the timestamp — so we treat it as NOT locked.
 */
export function isItemPastGrace(
  createdAt: string | null | undefined,
  now: number,
): boolean {
  if (!createdAt) return false;
  const created = new Date(createdAt).getTime();
  if (Number.isNaN(created)) return false;
  return now - created > ITEM_EDIT_GRACE_MS;
}
