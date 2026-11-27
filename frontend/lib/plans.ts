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

// The plan signup opens on when the landing named none.
export const RECOMMENDED_PLAN: Plan = "einvoicing";

export type Period = "monthly" | "annual";
export const PERIODS: readonly Period[] = ["monthly", "annual"];

export function isPeriod(value: unknown): value is Period {
  return value === "monthly" || value === "annual";
}

export function planFromQuery(value: unknown): Plan {
  const first = Array.isArray(value) ? value[0] : value;
  return isPlan(first) ? first : RECOMMENDED_PLAN;
}

// Pesos with a dot as thousands separator, built by hand so it reads the same on every runtime.
export function formatPesos(amount: number): string {
  const digits = Math.round(Math.abs(amount)).toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${amount < 0 ? "-" : ""}$${digits}`;
}

export interface PlanPrices {
  trialDays: number;
  prices: Record<Plan, Record<Period, number>>;
}

// GET /api/plans (backend/docs/api/plans.md). Null when any plan or price is missing: the page then shows no prices.
export function parsePlanPrices(body: unknown): PlanPrices | null {
  if (typeof body !== "object" || body === null) return null;
  const { trial_days, plans } = body as { trial_days?: unknown; plans?: unknown };
  if (typeof trial_days !== "number" || !Array.isArray(plans)) return null;
  const prices: Partial<Record<Plan, Record<Period, number>>> = {};
  for (const entry of plans) {
    const { id, monthly, annual } = (entry ?? {}) as { id?: unknown; monthly?: unknown; annual?: unknown };
    if (!isPlan(id) || typeof monthly !== "number" || typeof annual !== "number") return null;
    prices[id] = { monthly, annual };
  }
  if (!PLANS.every((plan) => prices[plan])) return null;
  return { trialDays: trial_days, prices: prices as Record<Plan, Record<Period, number>> };
}

export function planPrice(prices: PlanPrices | null, plan: Plan, period: Period): number | null {
  return prices ? prices.prices[plan][period] : null;
}

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
