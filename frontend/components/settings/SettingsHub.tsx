"use client";

import { PERMISSIONS, usePermissions } from "@/lib/permissions";
import { PermissionGate } from "@/components/permissions";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Card, CardBody } from "@/components/ui";

const sections = [
  {
    href: "/ajustes/integraciones",
    title: "Integraciones",
    description: "Servicios externos del restaurante, como las llaves de Wompi.",
    permission: PERMISSIONS.INTEGRATIONS_READ,
  },
  {
    href: "/ajustes/notificaciones",
    title: "Notificaciones",
    description: "Avisos al personal en Discord: pedidos pagados, reservas por confirmar y más.",
    permission: PERMISSIONS.NOTIFICATION_CHANNELS_READ,
  },
];

export const SETTINGS_SECTION_PERMISSIONS = sections.map((s) => s.permission);

export default function SettingsHub() {
  const { hasAnyPermission, isLoading } = usePermissions();

  let content;
  if (isLoading) {
    content = null;
  } else if (!hasAnyPermission(SETTINGS_SECTION_PERMISSIONS)) {
    content = <ErrorBanner message="No tienes acceso a los ajustes del restaurante." />;
  } else {
    content = (
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))", gap: "1rem" }}>
        {sections.map((section) => (
          <PermissionGate key={section.href} permission={section.permission}>
            <a href={section.href} style={{ textDecoration: "none", color: "inherit", display: "block" }}>
              <Card interactive>
                <CardBody>
                  <h2 style={{ margin: "0 0 0.5rem", fontSize: "1.125rem", color: "var(--color-text-primary)" }}>
                    {section.title}
                  </h2>
                  <p style={{ margin: 0, fontSize: "0.9rem", color: "var(--color-text-secondary)" }}>
                    {section.description}
                  </p>
                </CardBody>
              </Card>
            </a>
          </PermissionGate>
        ))}
      </div>
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
