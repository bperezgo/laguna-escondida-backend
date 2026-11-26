export interface Plan {
  id: "starter" | "einvoicing" | "full";
  name: string;
  /** Colombian pesos per month, before IVA. */
  monthly: number;
  includes: string[];
}

// Prices come from the owner. The same plans live in backend/docs/api/organizations.md#plans
// and frontend/lib/plans.ts; nothing checks the three against each other.
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
