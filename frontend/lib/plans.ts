import { supportMailto, type Notice } from "./support.ts";

// Mirrors the backend's plan table (backend/docs/api/organizations.md#plans).
export type Plan = "starter" | "einvoicing" | "full";
export type Feature = "core" | "einvoicing" | "customer_channels" | "payments_integrations" | "agents";

export const PLANS: readonly Plan[] = ["starter", "einvoicing", "full"];

export const PLAN_FEATURES: Record<Plan, readonly Feature[]> = {
  starter: ["core"],
  einvoicing: ["core", "einvoicing"],
  full: ["core", "einvoicing", "customer_channels", "payments_integrations", "agents"],
};

export const PLAN_LABELS: Record<Plan, string> = {
  starter: "Básico",
  einvoicing: "Facturación electrónica",
  full: "Completo",
};

export const PLAN_CARDS: { plan: Plan; title: string; includes: string[] }[] = [
  {
    plan: "starter",
    title: PLAN_LABELS.starter,
    includes: [
      "Órdenes, cocina y caja",
      "Productos, inventario, proveedores y compras",
      "Gastos, cierre de caja y reportes",
      "Usuarios y la caja del restaurante",
    ],
  },
  {
    plan: "einvoicing",
    title: PLAN_LABELS.einvoicing,
    includes: ["Todo lo del plan Básico", "Facturas electrónicas DIAN", "Documentos soporte"],
  },
  {
    plan: "full",
    title: PLAN_LABELS.full,
    includes: [
      "Todo lo del plan Facturación electrónica",
      "Reservas, domicilios y zonas de entrega",
      "Opiniones de clientes y notificaciones",
      "Pagos con Wompi e integraciones",
      "Agentes y claves API",
    ],
  },
];

// The app's pages behind a feature; a page not listed is core.
export const PAGE_FEATURES: readonly [string, Feature][] = [
  ["/invoices", "einvoicing"],
  ["/support-documents", "einvoicing"],
  ["/reservations", "customer_channels"],
  ["/feedback", "customer_channels"],
  ["/deliveries", "customer_channels"],
  ["/delivery-zones", "customer_channels"],
  ["/my-deliveries", "customer_channels"],
  ["/ajustes/notificaciones", "customer_channels"],
  ["/ajustes/integraciones", "payments_integrations"],
];

export function isPlan(value: unknown): value is Plan {
  return typeof value === "string" && (PLANS as readonly string[]).includes(value);
}

export function pageFeature(path: string): Feature {
  const match = PAGE_FEATURES.find(([prefix]) => path === prefix || path.startsWith(`${prefix}/`));
  return match ? match[1] : "core";
}

// Plans are cloud-only: the edge never hides anything. An unknown or absent plan (an older
// backend) hides nothing either; the backend still refuses what the plan excludes.
export function featureVisible(feature: Feature, plan: string | null | undefined, isEdge: boolean): boolean {
  if (isEdge || !isPlan(plan)) return true;
  return PLAN_FEATURES[plan].includes(feature);
}

export function pageVisible(path: string, plan: string | null | undefined, isEdge: boolean): boolean {
  return featureVisible(pageFeature(path), plan, isEdge);
}

export function planRequiredNotice(requiredPlan: string | undefined, email: string | null): Notice {
  const named = isPlan(requiredPlan) ? ` Está en el plan ${PLAN_LABELS[requiredPlan]}.` : "";
  return {
    message: `Tu plan no incluye esta función.${named} Escríbenos para cambiar de plan${email ? ` a ${email}` : ""}.`,
    mailto: supportMailto(email, "Cambiar de plan"),
    email,
  };
}
