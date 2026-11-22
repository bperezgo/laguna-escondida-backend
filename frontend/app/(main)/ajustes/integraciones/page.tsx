import { serverApiRequest } from "@/lib/api/server";
import type { OrganizationIntegrationListResponse } from "@/types/organizationIntegrations";
import type { OrganizationApiKeyListResponse } from "@/types/organizationApiKeys";
import { marketingAppCardState, wompiCardState, type IntegrationsLoad } from "@/lib/settings/integrations";
import { apiKeysCardState, mcpPublicUrl, type ApiKeysLoad } from "@/lib/settings/apiKeys";
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

async function loadApiKeys(): Promise<ApiKeysLoad> {
  try {
    const response = await serverApiRequest<OrganizationApiKeyListResponse>("/organization/api-keys");
    const body = await response.json().catch(() => ({}));
    if (!response.ok) return { status: response.status };
    return { status: response.status, keys: body.api_keys ?? [] };
  } catch {
    return { status: 500 };
  }
}

export default async function IntegrationsPage() {
  const [integrations, apiKeys] = await Promise.all([loadIntegrations(), loadApiKeys()]);
  return (
    <IntegrationsPageClient
      wompi={wompiCardState(integrations)}
      marketingApp={marketingAppCardState(integrations)}
      apiKeys={apiKeysCardState(apiKeys)}
      mcpUrl={mcpPublicUrl(process.env.MCP_PUBLIC_URL)}
    />
  );
}
