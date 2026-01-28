"use client";

import { usePermissions, PERMISSIONS } from "@/lib/permissions";
import { PermissionGate } from "@/components/permissions";

// Navigation items with their required permissions
const navItems = [
  {
    href: "/orders",
    title: "Órdenes",
    description: "Administra las órdenes de los clientes",
    permission: PERMISSIONS.ORDERS_READ,
    hoverColor: "var(--color-primary)",
  },
  {
    href: "/products",
    title: "Productos",
    description: "Ver y editar productos",
    permission: PERMISSIONS.PRODUCTS_READ,
    hoverColor: "var(--color-primary)",
  },
  {
    href: "/stock",
    title: "Inventario",
    description: "Administrar inventario",
    permission: PERMISSIONS.STOCK_READ,
    hoverColor: "var(--color-primary)",
  },
  {
    href: "/invoices",
    title: "Facturas",
    description: "Ver facturas",
    permission: PERMISSIONS.INVOICES_READ,
    hoverColor: "var(--color-primary)",
  },
  {
    href: "/kitchen",
    title: "Cocina",
    description: "Ver comandas en tiempo real",
    permission: PERMISSIONS.COMMANDS_READ,
    hoverColor: "var(--color-secondary)",
  },
  {
    href: "/suppliers",
    title: "Proveedores",
    description: "Administrar proveedores y catálogos",
    permission: PERMISSIONS.SUPPLIERS_READ,
    hoverColor: "var(--color-primary)",
  },
  {
    href: "/purchase-entries",
    title: "Entradas de Compra",
    description: "Registrar recepción de productos",
    permission: PERMISSIONS.PURCHASE_ENTRIES_READ,
    hoverColor: "var(--color-primary)",
  },
  {
    href: "/expenses",
    title: "Gastos",
    description: "Registrar gastos, inversiones y servicios",
    permission: PERMISSIONS.EXPENSES_READ,
    hoverColor: "var(--color-warning)",
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
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
      }}
    >
      <div
        style={{
          maxWidth: "80rem",
          margin: "0 auto",
          padding: "3rem 1rem",
        }}
      >
        <div style={{ textAlign: "center" }}>
          <h1
            style={{
              fontSize: "2.25rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
              marginBottom: "2rem",
            }}
          >
            Inicio
          </h1>
          <p
            style={{
              fontSize: "1.125rem",
              color: "var(--color-text-secondary)",
              marginBottom: "2rem",
            }}
          >
            Bienvenido al Sistema de Gestión de Laguna Escondida
            {user && ` - ${user.username}`}
          </p>

          {isLoading ? (
            <div
              style={{
                padding: "2rem",
                color: "var(--color-text-secondary)",
              }}
            >
              Cargando...
            </div>
          ) : (
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fit, minmax(250px, 1fr))",
                gap: "1rem",
                marginTop: "3rem",
                maxWidth: "1200px",
                margin: "3rem auto 0",
              }}
            >
              {navItems.map((item) => (
                <PermissionGate key={item.href} permission={item.permission}>
                  <a
                    href={item.href}
                    style={{
                      padding: "1.5rem",
                      backgroundColor: "var(--color-surface)",
                      borderRadius: "var(--radius-md)",
                      boxShadow: "var(--shadow-md)",
                      textDecoration: "none",
                      display: "block",
                      transition: "all var(--transition-normal)",
                      border: "1px solid var(--color-border)",
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.boxShadow = "var(--shadow-lg)";
                      e.currentTarget.style.borderColor = item.hoverColor;
                      e.currentTarget.style.transform = "translateY(-2px)";
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.boxShadow = "var(--shadow-md)";
                      e.currentTarget.style.borderColor = "var(--color-border)";
                      e.currentTarget.style.transform = "translateY(0)";
                    }}
                  >
                    <h2
                      style={{
                        fontSize: "1.25rem",
                        fontWeight: "600",
                        color: "var(--color-text-primary)",
                        marginBottom: "0.5rem",
                      }}
                    >
                      {item.title}
                    </h2>
                    <p
                      style={{ color: "var(--color-text-secondary)", margin: 0 }}
                    >
                      {item.description}
                    </p>
                  </a>
                </PermissionGate>
              ))}
            </div>
          )}

          <div style={{ marginTop: "3rem" }}>
            <button
              onClick={handleSignOut}
              style={{
                padding: "0.625rem 1.5rem",
                backgroundColor: "var(--color-danger)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-md)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                transition: "background-color var(--transition-normal)",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor =
                  "var(--color-danger-hover)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "var(--color-danger)";
              }}
            >
              Cerrar Sesión
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
