import { serverApiRequest } from "@/lib/api/server";
import type { DeliveryZone, DeliveryZoneListResponse } from "@/types/deliveryZone";
import type { OrganizationSettingsResponse } from "@/types/organizationSettings";
import DeliveriesPageClient, {
  type DeliveriesTab,
} from "@/components/deliveries/DeliveriesPageClient";

interface DeliveriesPageProps {
  searchParams: Promise<{ tab?: string }>;
}

function loadErrorFor(status: number, what: string): string {
  if (status === 404) return `${what} solo se administran en la nube.`;
  if (status === 403) return `No tienes permiso para ver ${what.toLowerCase()}.`;
  return `Error al cargar ${what.toLowerCase()}.`;
}

async function loadSettings(): Promise<{
  settings: OrganizationSettingsResponse | null;
  error: string;
}> {
  try {
    const response =
      await serverApiRequest<OrganizationSettingsResponse>("/organization/settings");
    if (!response.ok) {
      return { settings: null, error: loadErrorFor(response.status, "Los horarios y pausas") };
    }
    return { settings: await response.json(), error: "" };
  } catch (error) {
    console.error("Error fetching organization settings:", error);
    return { settings: null, error: "Error al cargar los horarios y pausas." };
  }
}

async function loadZones(): Promise<{ zones: DeliveryZone[]; error: string }> {
  try {
    const response = await serverApiRequest<DeliveryZoneListResponse>("/delivery-zones");
    if (!response.ok) {
      return { zones: [], error: loadErrorFor(response.status, "Las zonas de domicilio") };
    }
    const data = await response.json();
    return { zones: data.delivery_zones || [], error: "" };
  } catch (error) {
    console.error("Error fetching delivery zones:", error);
    return { zones: [], error: "Error al cargar las zonas de domicilio." };
  }
}

function parseTab(tab: string | undefined): DeliveriesTab | null {
  return tab === "estado" || tab === "horarios" || tab === "zonas" ? tab : null;
}

export default async function DeliveriesPage({ searchParams }: DeliveriesPageProps) {
  const [{ tab }, settings, zones] = await Promise.all([
    searchParams,
    loadSettings(),
    loadZones(),
  ]);

  return (
    <DeliveriesPageClient
      requestedTab={parseTab(tab)}
      initialSettings={settings.settings}
      settingsError={settings.error}
      initialZones={zones.zones}
      zonesError={zones.error}
    />
  );
}
