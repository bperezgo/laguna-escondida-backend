import { serverApiRequest } from "@/lib/api/server";
import type { OrganizationIntegrationListResponse } from "@/types/organizationIntegrations";
import { marketingAppCardState, wompiCardState, type IntegrationsLoad } from "@/lib/settings/integrations";
import IntegrationsPageClient from "@/components/settings/IntegrationsPageClient";

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

export default async function IntegrationsPage() {
  const load = await loadIntegrations();
  return <IntegrationsPageClient wompi={wompiCardState(load)} marketingApp={marketingAppCardState(load)} />;
}
