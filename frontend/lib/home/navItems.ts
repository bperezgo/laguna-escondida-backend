import { PERMISSIONS } from "../permissions/constants.ts";
import { pageVisible } from "../plans.ts";
import { visibleSettingsSections, type VisibilityContext } from "../settings/hub.ts";

export type NavGroup = "vender" | "menu" | "inventario" | "finanzas" | "clientes" | "ajustes";

export type NavIcon =
  | "house"
  | "receipt"
  | "cooking-pot"
  | "moped"
  | "package"
  | "calendar-check"
  | "fork-knife"
  | "stack"
  | "truck"
  | "shopping-cart"
  | "chart-line-up"
  | "cash-register"
  | "wallet"
  | "invoice"
  | "file-text"
  | "star"
  | "users"
  | "gear"
  | "plugs"
  | "bell"
  | "desktop-tower"
  | "dots-three";

export interface NavItem {
  href: string;
  title: string;
  group: NavGroup;
  icon: NavIcon;
  /** Any of them grants the entry. */
  permissions: string[];
  /** Hidden on the cloud for non-admin roles. */
  cloudAdminOnly?: boolean;
  /** Hidden on the edge, where the routes don't exist. */
  cloudOnly?: boolean;
}

export const NAV_ITEMS: readonly NavItem[] = [
  {
    href: "/orders",
    title: "Órdenes",
    group: "vender",
    icon: "receipt",
    permissions: [PERMISSIONS.ORDERS_READ],
    cloudAdminOnly: true,
  },
  {
    href: "/kitchen",
    title: "Cocina",
    group: "vender",
    icon: "cooking-pot",
    permissions: [PERMISSIONS.COMMANDS_READ],
    cloudAdminOnly: true,
  },
  {
    href: "/deliveries",
    title: "Domicilios",
    group: "vender",
    icon: "moped",
    // An accountant sees the hours, a zone-only role the zones.
    permissions: [PERMISSIONS.DELIVERIES_READ, PERMISSIONS.ORDERING_READ, PERMISSIONS.DELIVERY_ZONES_READ],
    cloudOnly: true,
  },
  {
    href: "/my-deliveries",
    title: "Mis entregas",
    group: "vender",
    icon: "package",
    permissions: [PERMISSIONS.DELIVERIES_CARRY],
    cloudOnly: true,
  },
  {
    href: "/reservations",
    title: "Reservas",
    group: "vender",
    icon: "calendar-check",
    permissions: [PERMISSIONS.RESERVATIONS_READ],
    cloudOnly: true,
  },
  {
    href: "/products",
    title: "Productos",
    group: "menu",
    icon: "fork-knife",
    permissions: [PERMISSIONS.PRODUCTS_READ],
  },
  {
    href: "/stock",
    title: "Inventario",
    group: "inventario",
    icon: "stack",
    permissions: [PERMISSIONS.STOCK_READ],
  },
  {
    href: "/suppliers",
    title: "Proveedores",
    group: "inventario",
    icon: "truck",
    permissions: [PERMISSIONS.SUPPLIERS_READ],
    cloudOnly: true,
  },
  {
    href: "/purchase-entries",
    title: "Entradas de Compra",
    group: "inventario",
    icon: "shopping-cart",
    permissions: [PERMISSIONS.PURCHASE_ENTRIES_READ],
    cloudOnly: true,
  },
  {
    href: "/financial",
    title: "Finanzas",
    group: "finanzas",
    icon: "chart-line-up",
    permissions: [PERMISSIONS.FINANCIAL_SUMMARY_READ],
    cloudOnly: true,
  },
  {
    href: "/close",
    title: "Cierre de Caja",
    group: "finanzas",
    icon: "cash-register",
    permissions: [PERMISSIONS.FINANCIAL_SUMMARY_READ],
    cloudAdminOnly: true,
    cloudOnly: true,
  },
  {
    href: "/expenses",
    title: "Gastos",
    group: "finanzas",
    icon: "wallet",
    permissions: [PERMISSIONS.EXPENSES_READ],
    cloudOnly: true,
  },
  {
    href: "/invoices",
    title: "Facturas",
    group: "finanzas",
    icon: "invoice",
    permissions: [PERMISSIONS.INVOICES_READ],
    cloudOnly: true,
  },
  {
    href: "/support-documents",
    title: "Documentos Soporte",
    group: "finanzas",
    icon: "file-text",
    permissions: [PERMISSIONS.SUPPORT_DOCUMENTS_READ],
    cloudOnly: true,
  },
  {
    href: "/feedback",
    title: "Opiniones",
    group: "clientes",
    icon: "star",
    permissions: [PERMISSIONS.FEEDBACK_READ],
    cloudOnly: true,
  },
  {
    href: "/users",
    title: "Usuarios",
    group: "ajustes",
    icon: "users",
    permissions: [PERMISSIONS.USERS_READ],
  },
  {
    href: "/ajustes",
    title: "Ajustes",
    group: "ajustes",
    icon: "gear",
    permissions: [],
    cloudOnly: true,
  },
];

export interface NavContext extends VisibilityContext {
  isAdmin: boolean;
}

export function visibleNavItems(ctx: NavContext): NavItem[] {
  const { hasPermission, plan, isEdge, isAdmin } = ctx;
  return NAV_ITEMS.filter((item) => {
    if (item.cloudAdminOnly && !isEdge && !isAdmin) return false;
    if (item.cloudOnly && isEdge) return false;
    if (!pageVisible(item.href, plan, isEdge)) return false;
    // Settings shows when any of its sections does.
    if (item.href === "/ajustes") return visibleSettingsSections(ctx).length > 0;
    return item.permissions.some(hasPermission);
  });
}
