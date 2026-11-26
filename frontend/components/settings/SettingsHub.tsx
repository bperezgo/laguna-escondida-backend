"use client";

import { usePermissions } from "@/lib/permissions";
import { visibleSettingsSections } from "@/lib/settings/hub";
import { ErrorBanner } from "@/components/deliveries/Banners";
import { Card, CardBody } from "@/components/ui";

export default function SettingsHub() {
  const { hasPermission, isLoading, organization, isEdge } = usePermissions();
  const sections = visibleSettingsSections({ hasPermission, plan: organization?.plan, isEdge });

  let content;
  if (isLoading) {
    content = null;
  } else if (sections.length === 0) {
    content = <ErrorBanner message="No tienes acceso a los ajustes del restaurante." />;
  } else {
    content = (
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))", gap: "1rem" }}>
        {sections.map((section) => (
          <a key={section.href} href={section.href} style={{ textDecoration: "none", color: "inherit", display: "block" }}>
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
