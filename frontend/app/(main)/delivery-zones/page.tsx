import { serverApiRequest } from "@/lib/api/server";
import type { DeliveryZone, DeliveryZoneListResponse } from "@/types/deliveryZone";
import DeliveryZonesPageClient from "@/components/delivery-zones/DeliveryZonesPageClient";

export default async function DeliveryZonesPage() {
  let zones: DeliveryZone[] = [];
  let loadError = "";

  try {
    const response =
      await serverApiRequest<DeliveryZoneListResponse>("/delivery-zones");
    if (response.status === 404) {
      loadError = "Las zonas de domicilio solo se administran en la nube.";
    } else if (response.status === 403) {
      loadError = "No tienes permiso para ver las zonas de domicilio.";
    } else if (!response.ok) {
      loadError = "Error al cargar las zonas de domicilio.";
    } else {
      const data = await response.json();
      zones = data.delivery_zones || [];
    }
  } catch (error) {
    console.error("Error fetching delivery zones:", error);
    loadError = "Error al cargar las zonas de domicilio.";
  }

  return (
    <DeliveryZonesPageClient initialZones={zones} loadError={loadError} />
  );
}
