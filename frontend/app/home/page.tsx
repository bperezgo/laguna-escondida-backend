"use client";

import { usePermissions } from "@/lib/permissions";
import { visibleNavItems } from "@/lib/home/navItems";
import { organizationDisplayName, organizationLogoUrl } from "@/lib/organization";
import { Card, CardBody, Button } from "@/components/ui";

export default function HomePage() {
  const { user, organization, isLoading, isAdmin, isEdge, hasPermission } = usePermissions();
  const businessName = organization
    ? organizationDisplayName(organization)
    : "Laguna Escondida";
  const logoUrl = organization ? organizationLogoUrl(organization) : null;

  const handleSignOut = async () => {
    try {
      await fetch("/api/auth/signout", {
        method: "POST",
      });
      window.location.href = "/signin";
    } catch (error) {
      console.error("Sign out error:", error);
    }
  };

  return (
    <div style={{ minHeight: "100vh", backgroundColor: "var(--color-bg)" }}>
      <div
        style={{
          maxWidth: "80rem",
          margin: "0 auto",
          padding: "3rem 1.5rem",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: "1rem",
            marginBottom: "2.5rem",
          }}
        >
          <div>
            {logoUrl && (
              <img
                src={logoUrl}
                alt={businessName}
                style={{
                  height: "3rem",
                  width: "auto",
                  objectFit: "contain",
                  marginBottom: "0.75rem",
                }}
              />
            )}
            <h1
              style={{
                fontSize: "1.875rem",
                fontWeight: 800,
                letterSpacing: "-0.02em",
                color: "var(--color-text-primary)",
                margin: 0,
              }}
            >
              Inicio
            </h1>
            <p
              style={{
                fontSize: "1rem",
                color: "var(--color-text-secondary)",
                marginTop: "0.5rem",
              }}
            >
              Bienvenido al Sistema de Gestión de {businessName}
              {user && ` — ${user.username}`}
            </p>
          </div>

          <Button variant="danger" onClick={handleSignOut}>
            Cerrar Sesión
          </Button>
        </div>

        {/* Module navigation */}
        {isLoading ? (
          <div
            style={{
              padding: "3rem",
              textAlign: "center",
              color: "var(--color-text-secondary)",
            }}
          >
            Cargando...
          </div>
        ) : (
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))",
              gap: "1rem",
            }}
          >
            {visibleNavItems({ hasPermission, plan: organization?.plan, isEdge, isAdmin }).map((item) => (
                <a
                  key={item.href}
                  href={item.href}
                  style={{
                    textDecoration: "none",
                    color: "inherit",
                    display: "block",
                  }}
                >
                  <Card interactive>
                    <CardBody>
                      <div
                        style={{
                          display: "flex",
                          alignItems: "center",
                          gap: "0.6rem",
                          marginBottom: "0.5rem",
                        }}
                      >
                        <span
                          style={{
                            width: "10px",
                            height: "10px",
                            borderRadius: "50%",
                            backgroundColor: item.accent,
                            flex: "none",
                          }}
                        />
                        <h2
                          style={{
                            fontSize: "1.125rem",
                            fontWeight: 700,
                            color: "var(--color-text-primary)",
                            margin: 0,
                          }}
                        >
                          {item.title}
                        </h2>
                      </div>
                      <p
                        style={{
                          color: "var(--color-text-secondary)",
                          fontSize: "0.9rem",
                          margin: 0,
                        }}
                      >
                        {item.description}
                      </p>
                    </CardBody>
                  </Card>
                </a>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
