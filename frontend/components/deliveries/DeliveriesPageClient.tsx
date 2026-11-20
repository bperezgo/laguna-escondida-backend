"use client";

import { useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import type { DeliveryZone } from "@/types/deliveryZone";
import type { OrganizationSettingsResponse } from "@/types/organizationSettings";
import { usePermissions } from "@/lib/permissions";
import { deliveriesTabs, type DeliveriesTab } from "@/lib/deliveries/board";
import DeliveryZonesPageClient from "@/components/delivery-zones/DeliveryZonesPageClient";
import OrderingStatusTab from "./OrderingStatusTab";
import OrderingHoursTab from "./OrderingHoursTab";
import DeliveryOrdersTab from "./DeliveryOrdersTab";

export type { DeliveriesTab };

interface DeliveriesPageClientProps {
  requestedTab: DeliveriesTab | null;
  initialSettings: OrganizationSettingsResponse | null;
  settingsError: string;
  initialZones: DeliveryZone[];
  zonesError: string;
}

const TAB_LABELS: Record<DeliveriesTab, string> = {
  pedidos: "Pedidos",
  estado: "Estado",
  horarios: "Horarios",
  zonas: "Zonas",
};

export default function DeliveriesPageClient({
  requestedTab,
  initialSettings,
  settingsError,
  initialZones,
  zonesError,
}: DeliveriesPageClientProps) {
  const router = useRouter();
  const pathname = usePathname();
  const { hasPermission, isLoading } = usePermissions();
  const [settings, setSettings] = useState(initialSettings);

  const tabs = deliveriesTabs(hasPermission);
  const [selected, setSelected] = useState<DeliveriesTab | null>(requestedTab);
  const current = selected && tabs.includes(selected) ? selected : (tabs[0] ?? null);

  const selectTab = (tab: DeliveriesTab) => {
    setSelected(tab);
    router.replace(`${pathname}?tab=${tab}`, { scroll: false });
  };

  return (
    <div style={{ minHeight: "100vh", backgroundColor: "var(--color-bg)", padding: "2rem" }}>
      <div style={{ maxWidth: "1400px", margin: "0 auto" }}>
        <h1
          style={{
            margin: 0,
            fontSize: "2rem",
            fontWeight: "bold",
            color: "var(--color-text-primary)",
          }}
        >
          Domicilios
        </h1>
        <p
          style={{
            margin: "0.5rem 0 1.5rem 0",
            color: "var(--color-text-secondary)",
            fontSize: "1rem",
          }}
        >
          Los pedidos para llevar y domicilios en curso, cuándo se reciben y dónde se entregan.
        </p>

        <div
          role="tablist"
          aria-label="Secciones de domicilios"
          style={{
            display: "flex",
            gap: "0.5rem",
            marginBottom: "1.5rem",
            borderBottom: "2px solid var(--color-border)",
            flexWrap: "wrap",
          }}
        >
          {tabs.map((tab) => {
            const active = tab === current;
            return (
              <button
                key={tab}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => selectTab(tab)}
                style={{
                  padding: "0.75rem 1.5rem",
                  fontSize: "1rem",
                  fontWeight: 600,
                  backgroundColor: "transparent",
                  color: active ? "var(--color-primary)" : "var(--color-text-secondary)",
                  border: "none",
                  borderBottom: active
                    ? "3px solid var(--color-primary)"
                    : "3px solid transparent",
                  cursor: "pointer",
                  marginBottom: "-2px",
                  whiteSpace: "nowrap",
                }}
              >
                {TAB_LABELS[tab]}
              </button>
            );
          })}
        </div>

        {!isLoading && current === null && (
          <p style={{ color: "var(--color-text-secondary)" }}>
            No tienes permiso para ver esta sección.
          </p>
        )}
        {current === "pedidos" && <DeliveryOrdersTab />}
        {current === "estado" && (
          <OrderingStatusTab settings={settings} loadError={settingsError} onSaved={setSettings} />
        )}
        {current === "horarios" && (
          <OrderingHoursTab settings={settings} loadError={settingsError} onSaved={setSettings} />
        )}
        {current === "zonas" && (
          <DeliveryZonesPageClient initialZones={initialZones} loadError={zonesError} embedded />
        )}
      </div>
    </div>
  );
}
