import { serverApiRequest } from "@/lib/api/server";
import type { OrganizationIntegrationListResponse } from "@/types/organizationIntegrations";
import { wompiCardState, type IntegrationsLoad } from "@/lib/settings/integrations";
import SettingsPageClient from "@/components/settings/SettingsPageClient";

async function loadIntegrations(): Promise<IntegrationsLoad> {
  try {
    const response = await serverApiRequest<OrganizationIntegrationListResponse>("/organization/integrations");
    const body = await response.json().catch(() => ({}));
    if (!response.ok) return { status: response.status, code: body.code };
    return { status: response.status, integrations: body.integrations ?? [] };
  } catch {
    return { status: 500 };
  }
}

export default async function SettingsPage() {
  const load = await loadIntegrations();
  return <SettingsPageClient wompi={wompiCardState(load)} />;
}
