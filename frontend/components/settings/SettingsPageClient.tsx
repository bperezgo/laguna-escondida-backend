"use client";

import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import type { WompiCardState } from "@/lib/settings/integrations";
import { ErrorBanner, NoticeBanner } from "@/components/deliveries/Banners";
import WompiIntegrationCard from "./WompiIntegrationCard";

interface SettingsPageClientProps {
  wompi: WompiCardState;
}

export default function SettingsPageClient({ wompi }: SettingsPageClientProps) {
  const { hasPermission, isLoading } = usePermissions();
  const canRead = hasPermission(PERMISSIONS.INTEGRATIONS_READ);
  const canManage = hasPermission(PERMISSIONS.INTEGRATIONS_MANAGE);

  let content;
  if (isLoading) {
    content = null;
  } else if (!canRead) {
    // On the edge the permissions are dropped, so the page shows nothing of the cloud's state.
    content = <ErrorBanner message="No tienes acceso a los ajustes del restaurante." />;
  } else {
    content = (
      <>
        {!canManage && <NoticeBanner message="Solo puedes ver el estado. Cambiar las llaves requiere ser administrador." />}
        <section style={{ display: "flex", flexDirection: "column", gap: "1rem" }}>
          <div>
            <h2 style={{ margin: 0, fontSize: "1.5rem", color: "var(--color-text-primary)" }}>Integraciones</h2>
            <p style={{ margin: "0.25rem 0 0", color: "var(--color-text-secondary)" }}>
              Servicios externos conectados al restaurante. Las llaves se guardan cifradas y nunca se vuelven a
              mostrar: solo sus últimos 4 caracteres.
            </p>
          </div>
          <WompiIntegrationCard initialState={wompi} canManage={canManage} />
        </section>
      </>
    );
  }

  return (
    <div style={{ minHeight: "100vh", backgroundColor: "var(--color-bg)", padding: "2rem" }}>
      <div style={{ maxWidth: "960px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "1rem" }}>
        <h1 style={{ margin: 0, fontSize: "2rem", fontWeight: "bold", color: "var(--color-text-primary)" }}>Ajustes</h1>
        {content}
      </div>
    </div>
  );
}
