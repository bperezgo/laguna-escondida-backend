"use client";

import { usePermissions, PERMISSIONS } from "@/lib/permissions";
import { PermissionGate } from "@/components/permissions";
import { Card, CardBody, Button } from "@/components/ui";

// Navigation items with their required permissions
const navItems = [
  {
    href: "/orders",
    title: "Órdenes",
    description: "Administra las órdenes de los clientes",
    permission: PERMISSIONS.ORDERS_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/products",
    title: "Productos",
    description: "Ver y editar productos",
    permission: PERMISSIONS.PRODUCTS_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/stock",
    title: "Inventario",
    description: "Administrar inventario",
    permission: PERMISSIONS.STOCK_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/invoices",
    title: "Facturas",
    description: "Ver facturas",
    permission: PERMISSIONS.INVOICES_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/support-documents",
    title: "Documentos Soporte",
    description: "Crear y administrar documentos soporte",
    permission: PERMISSIONS.SUPPORT_DOCUMENTS_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/kitchen",
    title: "Cocina",
    description: "Ver comandas en tiempo real",
    permission: PERMISSIONS.COMMANDS_READ,
    accent: "var(--color-secondary)",
  },
  {
    href: "/suppliers",
    title: "Proveedores",
    description: "Administrar proveedores y catálogos",
    permission: PERMISSIONS.SUPPLIERS_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/purchase-entries",
    title: "Entradas de Compra",
    description: "Registrar recepción de productos",
    permission: PERMISSIONS.PURCHASE_ENTRIES_READ,
    accent: "var(--color-primary)",
  },
  {
    href: "/expenses",
    title: "Gastos",
    description: "Registrar gastos, inversiones y servicios",
    permission: PERMISSIONS.EXPENSES_READ,
    accent: "var(--color-warning)",
  },
  {
    href: "/financial",
    title: "Finanzas",
    description: "Resumen financiero — ingresos, gastos y utilidad",
    permission: PERMISSIONS.FINANCIAL_SUMMARY_READ,
    accent: "var(--color-success)",
  },
  {
    href: "/users",
    title: "Usuarios",
    description: "Crear y administrar usuarios del sistema",
    permission: PERMISSIONS.USERS_READ,
    accent: "var(--color-primary)",
  },
];

export default function HomePage() {
  const { user, isLoading } = usePermissions();

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
              Bienvenido al Sistema de Gestión de Laguna Escondida
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
            {navItems.map((item) => (
              <PermissionGate key={item.href} permission={item.permission}>
                <a
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
              </PermissionGate>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
