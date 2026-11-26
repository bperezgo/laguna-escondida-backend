import { PERMISSIONS } from "../permissions/constants.ts";
import { pageVisible } from "../plans.ts";
import { visibleSettingsSections, type VisibilityContext } from "../settings/hub.ts";

export interface NavItem {
  href: string;
  title: string;
  description: string;
  /** Any of them grants the card. */
  permissions: string[];
  accent: string;
  /** Hidden on the cloud for non-admin roles. */
  cloudAdminOnly?: boolean;
  /** Hidden on the edge, where the routes don't exist. */
  cloudOnly?: boolean;
}

export const NAV_ITEMS: readonly NavItem[] = [
  {
    href: "/orders",
    title: "Órdenes",
    description: "Administra las órdenes de los clientes",
    permissions: [PERMISSIONS.ORDERS_READ],
    accent: "var(--color-primary)",
    cloudAdminOnly: true,
  },
  {
    href: "/products",
    title: "Productos",
    description: "Ver y editar productos",
    permissions: [PERMISSIONS.PRODUCTS_READ],
    accent: "var(--color-primary)",
  },
  {
    href: "/stock",
    title: "Inventario",
    description: "Administrar inventario",
    permissions: [PERMISSIONS.STOCK_READ],
    accent: "var(--color-primary)",
  },
  {
    href: "/invoices",
    title: "Facturas",
    description: "Ver facturas",
    permissions: [PERMISSIONS.INVOICES_READ],
    accent: "var(--color-primary)",
    cloudOnly: true,
  },
  {
    href: "/support-documents",
    title: "Documentos Soporte",
    description: "Crear y administrar documentos soporte",
    permissions: [PERMISSIONS.SUPPORT_DOCUMENTS_READ],
    accent: "var(--color-primary)",
    cloudOnly: true,
  },
  {
    href: "/kitchen",
    title: "Cocina",
    description: "Ver comandas en tiempo real",
    permissions: [PERMISSIONS.COMMANDS_READ],
    accent: "var(--color-secondary)",
    cloudAdminOnly: true,
  },
  {
    href: "/suppliers",
    title: "Proveedores",
    description: "Administrar proveedores y catálogos",
    permissions: [PERMISSIONS.SUPPLIERS_READ],
    accent: "var(--color-primary)",
    cloudOnly: true,
  },
  {
    href: "/purchase-entries",
    title: "Entradas de Compra",
    description: "Registrar recepción de productos",
    permissions: [PERMISSIONS.PURCHASE_ENTRIES_READ],
    accent: "var(--color-primary)",
    cloudOnly: true,
  },
  {
    href: "/expenses",
    title: "Gastos",
    description: "Registrar gastos, inversiones y servicios",
    permissions: [PERMISSIONS.EXPENSES_READ],
    accent: "var(--color-warning)",
    cloudOnly: true,
  },
  {
    href: "/financial",
    title: "Finanzas",
    description: "Resumen financiero — ingresos, gastos y utilidad",
    permissions: [PERMISSIONS.FINANCIAL_SUMMARY_READ],
    accent: "var(--color-success)",
    cloudOnly: true,
  },
  {
    href: "/close",
    title: "Cierre de Caja",
    description: "Cuadre diario del dinero por método de pago",
    permissions: [PERMISSIONS.FINANCIAL_SUMMARY_READ],
    accent: "var(--color-success)",
    cloudAdminOnly: true,
    cloudOnly: true,
  },
  {
    href: "/deliveries",
    title: "Domicilios",
    description: "Pedidos en camino, pausas, horarios y zonas de domicilios y pedidos para llevar",
    // An accountant sees the hours, a zone-only role the zones.
    permissions: [PERMISSIONS.DELIVERIES_READ, PERMISSIONS.ORDERING_READ, PERMISSIONS.DELIVERY_ZONES_READ],
    accent: "var(--color-secondary)",
    cloudOnly: true,
  },
  {
    href: "/my-deliveries",
    title: "Mis entregas",
    description: "Pedidos listos para llevar y los que tienes en camino",
    permissions: [PERMISSIONS.DELIVERIES_CARRY],
    accent: "var(--color-primary)",
    cloudOnly: true,
  },
  {
    href: "/reservations",
    title: "Reservas",
    description: "Reservas del día en tiempo real: confirmar, rechazar y marcar quién no llegó",
    permissions: [PERMISSIONS.RESERVATIONS_READ],
    accent: "var(--color-secondary)",
    cloudOnly: true,
  },
  {
    href: "/feedback",
    title: "Opiniones",
    description: "Lo que calificaron los clientes después de su visita",
    permissions: [PERMISSIONS.FEEDBACK_READ],
    accent: "var(--color-success)",
    cloudOnly: true,
  },
  {
    href: "/users",
    title: "Usuarios",
    description: "Crear y administrar usuarios del sistema",
    permissions: [PERMISSIONS.USERS_READ],
    accent: "var(--color-primary)",
  },
  {
    href: "/ajustes",
    title: "Ajustes",
    description: "Integraciones, notificaciones e instalación de la caja",
    permissions: [],
    accent: "var(--color-secondary)",
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
