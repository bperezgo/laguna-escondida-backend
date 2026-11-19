"use client";

import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import type { WompiCardState } from "@/lib/settings/integrations";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";
import SettingsSectionShell from "./SettingsSectionShell";
import WompiIntegrationCard from "./WompiIntegrationCard";

interface IntegrationsPageClientProps {
  wompi: WompiCardState;
}

export default function IntegrationsPageClient({ wompi }: IntegrationsPageClientProps) {
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
        {!canManage && <NoticeBanner message="Solo puedes ver el estado. Cambiar las llaves requiere ser administrador." />}
        <p style={{ margin: 0, color: "var(--color-text-secondary)" }}>
          Servicios externos conectados al restaurante. Las llaves se guardan cifradas y nunca se vuelven a mostrar:
          solo sus últimos 4 caracteres.
        </p>
        <WompiIntegrationCard initialState={wompi} canManage={canManage} />
      </>
    );
  }

  return <SettingsSectionShell title="Integraciones">{content}</SettingsSectionShell>;
}
