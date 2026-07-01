import type { OpenBill } from "@/types/order";
import type { AuthUser } from "@/lib/permissions";

/** Which slice of open bills the Orders page is showing. */
export type OrderViewMode = "mine" | "all";

/** Stable key/label for the bucket holding bills with no `created_by`. */
export const UNASSIGNED_KEY = "__unassigned__";
export const UNASSIGNED_LABEL = "Sin asignar";
export const MINE_LABEL = "Mis órdenes";

/**
 * Role names that should land on the "Todas" view by default (the box / a
 * manager wants the whole floor at a glance). Matched case-insensitively and
 * trimmed. Roles are dynamic `{ id, name }` from the backend, so we cover both
 * the English and Spanish spellings until the exact strings are pinned down.
 */
const ALL_ORDERS_ROLE_NAMES = new Set([
  "admin",
  "administrador",
  "manager",
  "gerente",
]);

/**
 * A bill is "mine" when I created it. Matched by **id** — the bill carries
 * `created_by.user_name` while the auth user carries `username`, so a name
 * match would be a trap.
 */
export function isMyBill(bill: OpenBill, user: AuthUser | null): boolean {
  return !!user && !!bill.created_by && bill.created_by.id === user.id;
}

/**
 * Whether the user holds a managerial role (admin/manager, either spelling).
 * Single source of truth for "is this a supervisor" — used both to default to
 * the "Todas" view and to bypass the per-item edit lock in the order editor.
 */
export function isManagerialUser(user: AuthUser | null): boolean {
  if (!user) return false;
  return (user.roles ?? []).some((role) =>
    ALL_ORDERS_ROLE_NAMES.has(role.name.trim().toLowerCase()),
  );
}

/**
 * Whether this user should default to the "Todas" (all orders) view. Waitresses
 * default to "Mis órdenes"; admin/manager default to the whole floor.
 *
 * This is the single place that keys behaviour off a role. If the backend ever
 * grows a dedicated `orders:read_all` permission, that signal wins — swapping
 * fully to PBAC then is a one-line change here.
 */
export function canDefaultToAllOrders(user: AuthUser | null): boolean {
  if (!user) return false;
  if (user.permissions?.includes("orders:read_all")) return true;
  return isManagerialUser(user);
}

/** Search predicate: match by table identifier OR waitress name. */
export function billMatchesQuery(bill: OpenBill, query: string): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const identifier = bill.temporal_identifier?.toLowerCase() ?? "";
  const waitress = bill.created_by?.name?.toLowerCase() ?? "";
  return identifier.includes(q) || waitress.includes(q);
}

/**
 * Intra-view ordering: most recent first (created_at DESC), so a freshly
 * created order shows up at the top of the page instead of the bottom.
 */
export function sortBillsByRecency(bills: OpenBill[]): OpenBill[] {
  return [...bills].sort(
    (a, b) =>
      new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );
}

export interface WaitressGroup {
  /** User id, or `UNASSIGNED_KEY` for bills with no creator. */
  key: string;
  /** Section header text. */
  label: string;
  count: number;
  bills: OpenBill[];
  isMine: boolean;
}

/**
 * Group bills into per-waitress sections for the "Todas" view. Ordering:
 * my section first, then other waitresses alphabetically by name, then
 * "Sin asignar" last. Bills within a section keep the identifier sort.
 */
export function groupBillsByWaitress(
  bills: OpenBill[],
  user: AuthUser | null,
): WaitressGroup[] {
  const byKey = new Map<string, WaitressGroup>();

  for (const bill of bills) {
    const creator = bill.created_by;
    const key = creator?.id ?? UNASSIGNED_KEY;
    const mine = isMyBill(bill, user);

    let group = byKey.get(key);
    if (!group) {
      group = {
        key,
        label: mine
          ? MINE_LABEL
          : (creator?.name ?? UNASSIGNED_LABEL),
        count: 0,
        bills: [],
        isMine: mine,
      };
      byKey.set(key, group);
    }
    group.bills.push(bill);
  }

  const groups = Array.from(byKey.values());
  for (const group of groups) {
    group.bills = sortBillsByRecency(group.bills);
    group.count = group.bills.length;
  }

  return groups.sort((a, b) => {
    // My section always first.
    if (a.isMine) return -1;
    if (b.isMine) return 1;
    // "Sin asignar" always last.
    if (a.key === UNASSIGNED_KEY) return 1;
    if (b.key === UNASSIGNED_KEY) return -1;
    // Everyone else alphabetically by name.
    return a.label.localeCompare(b.label);
  });
}

/**
 * Distinct hues for per-server section colouring. Teal (~172°) is intentionally
 * absent — that's the brand/primary colour reserved for "my" section.
 */
const SERVER_HUES = [210, 265, 330, 22, 45, 130, 290, 350];

/** Stable hue for a server, derived from their id so it never shifts. */
function getServerHue(key: string): number {
  let hash = 0;
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) >>> 0;
  }
  return SERVER_HUES[hash % SERVER_HUES.length];
}

export interface GroupAccent {
  /** Solid colour for the left stripe. */
  stripe: string;
  /** Translucent panel background tint. */
  tint: string;
  /** Translucent background for the name chip. */
  chipBg: string;
  /** Chip border. */
  chipBorder: string;
  /** Theme-safe heading text colour. */
  heading: string;
}

/**
 * Presentation colours for a section. "Mine" uses the brand primary,
 * "Sin asignar" stays neutral, everyone else gets a stable hue. All tints are
 * translucent so they read correctly over both the light and dark themes.
 */
export function getGroupAccent(group: WaitressGroup): GroupAccent {
  if (group.isMine) {
    return {
      stripe: "var(--color-primary)",
      tint: "var(--color-primary-light)",
      chipBg: "var(--color-primary-light)",
      chipBorder: "var(--color-primary)",
      heading: "var(--color-primary-hover)",
    };
  }

  if (group.key === UNASSIGNED_KEY) {
    return {
      stripe: "var(--color-border-strong)",
      tint: "rgba(148, 163, 184, 0.08)",
      chipBg: "rgba(148, 163, 184, 0.16)",
      chipBorder: "var(--color-border-strong)",
      heading: "var(--color-text-secondary)",
    };
  }

  const h = getServerHue(group.key);
  return {
    stripe: `hsl(${h} 65% 50%)`,
    tint: `hsl(${h} 70% 50% / 0.08)`,
    chipBg: `hsl(${h} 70% 50% / 0.16)`,
    chipBorder: `hsl(${h} 65% 50% / 0.45)`,
    heading: "var(--color-text-primary)",
  };
}
