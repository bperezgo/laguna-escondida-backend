import { visibleNavItems, type NavContext, type NavGroup, type NavIcon } from "../home/navItems.ts";
import { PERMISSIONS } from "../permissions/constants.ts";
import { visibleSettingsSections } from "../settings/hub.ts";

export interface NavEntry {
  href: string;
  title: string;
  icon: NavIcon;
  /** Other path prefixes that belong to this entry. */
  match?: string[];
}

export interface NavGroupEntries {
  id: NavGroup;
  title: string;
  entries: NavEntry[];
}

export interface CloudNavigation {
  home: NavEntry | null;
  groups: NavGroupEntries[];
}

export interface BoxNavigation {
  tabs: NavEntry[];
  more: NavEntry[];
}

export const HOME_ENTRY: NavEntry = { href: "/home", title: "Inicio", icon: "house" };

const GROUPS: readonly { id: NavGroup; title: string }[] = [
  { id: "vender", title: "Vender" },
  { id: "menu", title: "Menú" },
  { id: "inventario", title: "Inventario" },
  { id: "finanzas", title: "Finanzas" },
  { id: "clientes", title: "Clientes" },
  { id: "ajustes", title: "Ajustes" },
];

const EXTRA_PREFIXES: Record<string, string[]> = {
  "/deliveries": ["/delivery-zones"],
};

const SETTINGS_ICONS: Record<string, NavIcon> = {
  "/ajustes/integraciones": "plugs",
  "/ajustes/notificaciones": "bell",
  "/ajustes/instalar-caja": "desktop-tower",
};

const BOX_TABS = ["/orders", "/kitchen"];

function entriesOf(ctx: NavContext): (NavEntry & { group: NavGroup })[] {
  return visibleNavItems(ctx).flatMap((item) => {
    // The settings hub is listed as its sections.
    if (item.href === "/ajustes") {
      return visibleSettingsSections(ctx).map((s) => ({
        href: s.href,
        title: s.title,
        icon: SETTINGS_ICONS[s.href] ?? "gear",
        group: item.group,
      }));
    }
    return [{ href: item.href, title: item.title, icon: item.icon, group: item.group, match: EXTRA_PREFIXES[item.href] }];
  });
}

function strip({ href, title, icon, match }: NavEntry): NavEntry {
  return match ? { href, title, icon, match } : { href, title, icon };
}

export function canSeeDashboard(ctx: NavContext): boolean {
  return !ctx.isEdge && ctx.hasPermission(PERMISSIONS.FINANCIAL_SUMMARY_READ);
}

export function cloudNavigation(ctx: NavContext): CloudNavigation {
  const entries = entriesOf(ctx);
  return {
    home: canSeeDashboard(ctx) ? HOME_ENTRY : null,
    groups: GROUPS.map((g) => ({
      ...g,
      entries: entries.filter((e) => e.group === g.id).map(strip),
    })).filter((g) => g.entries.length > 0),
  };
}

export function boxNavigation(ctx: NavContext): BoxNavigation {
  const entries = entriesOf(ctx).map(strip);
  return {
    tabs: entries.filter((e) => BOX_TABS.includes(e.href)),
    more: entries.filter((e) => !BOX_TABS.includes(e.href)),
  };
}

function prefixLength(pathname: string, prefix: string): number {
  return pathname === prefix || pathname.startsWith(`${prefix}/`) ? prefix.length : -1;
}

export function currentEntry(pathname: string, entries: readonly NavEntry[]): NavEntry | null {
  let best: NavEntry | null = null;
  let bestLength = -1;
  for (const entry of entries) {
    for (const prefix of [entry.href, ...(entry.match ?? [])]) {
      const length = prefixLength(pathname, prefix);
      if (length > bestLength) {
        best = entry;
        bestLength = length;
      }
    }
  }
  return best;
}

export function cloudEntries(nav: CloudNavigation): NavEntry[] {
  return [...(nav.home ? [nav.home] : []), ...nav.groups.flatMap((g) => g.entries)];
}

/** The tab to mark on the box: a tab's href, "more", or null. */
export function currentBoxTab(pathname: string, nav: BoxNavigation): string | null {
  const entry = currentEntry(pathname, [...nav.tabs, ...nav.more]);
  if (!entry) return null;
  return nav.tabs.includes(entry) ? entry.href : "more";
}

/** Where a user starts, or null when they can open nothing. */
export function firstScreen(ctx: NavContext): string | null {
  if (!ctx.isEdge) {
    if (canSeeDashboard(ctx)) return HOME_ENTRY.href;
    return cloudNavigation(ctx).groups[0]?.entries[0]?.href ?? null;
  }
  const { tabs, more } = boxNavigation(ctx);
  const has = (href: string) => tabs.some((t) => t.href === href);
  if (has("/orders") && ctx.hasPermission(PERMISSIONS.ORDERS_CREATE)) return "/orders";
  if (has("/kitchen")) return "/kitchen";
  return [...tabs, ...more][0]?.href ?? null;
}
