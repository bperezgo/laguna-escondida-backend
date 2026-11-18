"use client";

import { useEdgeStatus } from "@/lib/edge/context";
import { Badge } from "@/components/ui";

export default function ConnectivityBadge() {
  const { isOffline, isLoading, status, pendingSyncCount } = useEdgeStatus();

  // Avoid flashing a misleading state before the first poll resolves.
  if (isLoading && !status) {
    return (
      <span title="Verificando conexión…">
        <Badge tone="neutral">Conectando…</Badge>
      </span>
    );
  }

  const connectivityTooltip = isOffline
    ? "Sin conexión a internet. Puedes seguir tomando órdenes y cobrando; todo se sincronizará cuando vuelva la conexión."
    : "Conectado. Todo se sincroniza en tiempo real.";

  const unsyncedOrders = status?.unsynced_orders ?? 0;
  const pendingInvoices = status?.pending_invoices ?? 0;
  const pendingTooltip = `${unsyncedOrders} ${
    unsyncedOrders === 1 ? "orden" : "órdenes"
  } y ${pendingInvoices} ${
    pendingInvoices === 1 ? "factura" : "facturas"
  } pendientes de sincronizar.`;

  return (
    <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
      <span title={connectivityTooltip}>
        {isOffline ? (
          <Badge tone="danger">Sin conexión — trabajando local</Badge>
        ) : (
          <Badge tone="success">En línea</Badge>
        )}
      </span>

      {pendingSyncCount > 0 && (
        <span title={pendingTooltip}>
          <Badge tone="warning">
            {pendingSyncCount}{" "}
            {pendingSyncCount === 1 ? "pendiente" : "pendientes"} de sincronizar
          </Badge>
        </span>
      )}
    </div>
  );
}
