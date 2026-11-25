"use client";

import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import type { MarketingAppCardState, WompiCardState } from "@/lib/settings/integrations";
import { showApiKeysCard, type ApiKeysCardState } from "@/lib/settings/apiKeys";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";
import type { PaymentSettingsCardState } from "@/lib/settings/paymentSettings";
import PaymentSettingsCard from "./PaymentSettingsCard";
import SettingsSectionShell from "./SettingsSectionShell";
import MarketingAppIntegrationCard from "./MarketingAppIntegrationCard";
import WompiIntegrationCard from "./WompiIntegrationCard";
import ApiKeysCard from "./ApiKeysCard";

interface IntegrationsPageClientProps {
  wompi: WompiCardState;
  marketingApp: MarketingAppCardState;
  payment: PaymentSettingsCardState;
  apiKeys: ApiKeysCardState;
  mcpUrl: string | null;
}

export default function IntegrationsPageClient({ wompi, marketingApp, payment, apiKeys, mcpUrl }: IntegrationsPageClientProps) {
  const { hasPermission, isLoading } = usePermissions();
  const canRead = hasPermission(PERMISSIONS.INTEGRATIONS_READ);
  const canManage = hasPermission(PERMISSIONS.INTEGRATIONS_MANAGE);

  let content;
  if (isLoading) {
    content = null;
  } else if (!canRead) {
    // On the edge the permissions are dropped, so the page shows nothing of the cloud's state.
    content = <ErrorBanner message="No tienes acceso a las integraciones del restaurante." />;
  } else {
    content = (
      <>
        {!canManage && <NoticeBanner message="Solo puedes ver el estado. Cambiar las llaves o secretos requiere ser administrador." />}
        <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>
          Servicios externos conectados al restaurante. Las llaves se guardan cifradas y nunca se vuelven a mostrar:
          solo sus últimos 4 caracteres.
        </p>
        <PaymentSettingsCard initialState={payment} canManage={canManage} wompiConfigured={wompi.kind === "configured"} />
        <WompiIntegrationCard initialState={wompi} canManage={canManage} />
        <MarketingAppIntegrationCard initialState={marketingApp} canManage={canManage} />
        {showApiKeysCard(hasPermission) && <ApiKeysCard initialState={apiKeys} mcpUrl={mcpUrl} />}
      </>
    );
  }

  return <SettingsSectionShell title="Integraciones">{content}</SettingsSectionShell>;
}
