import type { PlanId } from "./actions.ts";

export interface Plan {
  id: PlanId;
  name: string;
  /** Colombian pesos per month. */
  monthly: number;
  includes: string[];
  /** The plan the pricing section points a visitor to. At most one. */
  recommended?: boolean;
}

// Prices come from the owner. The cloud charges from its own table (backend/docs/api/plans.md);
// scripts/check-prices.ts fails a release when the two differ.
export const plans: Plan[] = [
  {
    id: "starter",
    name: "Básico",
    monthly: 25000,
    includes: [
      "Órdenes, cocina y caja",
      "La caja del restaurante, que sigue vendiendo sin internet",
      "Productos, inventario, proveedores y compras",
      "Gastos, cierre del día y reportes",
      "Usuarios para tu equipo",
    ],
  },
  {
    id: "einvoicing",
    name: "Facturación electrónica",
    monthly: 40000,
    recommended: true,
    includes: ["Todo lo del plan Básico", "Factura electrónica para la DIAN", "Documentos soporte de tus compras"],
  },
  {
    id: "full",
    name: "Completo",
    monthly: 100000,
    includes: [
      "Todo lo del plan Facturación electrónica",
      "Reservas, domicilios y zonas de entrega",
      "Opiniones de clientes y notificaciones",
      "Pagos con Wompi e integraciones",
      "Conexión con el agente de WhatsApp (se contrata aparte)",
    ],
  },
];

/** A year costs ten months: two are free. */
export function annualPrice(plan: Plan): number {
  return plan.monthly * 10;
}

export function formatCop(amount: number): string {
  return `$${String(amount).replace(/\B(?=(\d{3})+(?!\d))/g, ".")}`;
}
