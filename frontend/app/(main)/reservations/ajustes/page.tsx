import { serverApiRequest } from "@/lib/api/server";
import type { OrganizationSettingsResponse } from "@/types/organizationSettings";
import ReservationSettingsPageClient from "@/components/reservations/ReservationSettingsPageClient";

function loadErrorFor(status: number): string {
  if (status === 404) return "La confirmación automática solo se administra en la nube.";
  if (status === 403) return "No tienes permiso para ver la configuración de reservas.";
  return "Error al cargar la configuración de reservas.";
}

async function loadSettings(): Promise<{ settings: OrganizationSettingsResponse | null; error: string }> {
  try {
    const response = await serverApiRequest<OrganizationSettingsResponse>("/organization/settings");
    if (!response.ok) return { settings: null, error: loadErrorFor(response.status) };
    return { settings: await response.json(), error: "" };
  } catch (error) {
    console.error("Error fetching organization settings:", error);
    return { settings: null, error: "Error al cargar la configuración de reservas." };
  }
}

export default async function ReservationSettingsPage() {
  const { settings, error } = await loadSettings();
  return <ReservationSettingsPageClient initialSettings={settings} loadError={error} />;
}
