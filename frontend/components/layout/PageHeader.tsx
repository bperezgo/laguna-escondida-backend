"use client";

import { useRouter, usePathname } from "next/navigation";

export default function PageHeader() {
  const router = useRouter();
  const pathname = usePathname();

  // Determine page title based on current path
  const getPageTitle = () => {
    if (pathname.startsWith("/orders")) return "Órdenes";
    if (pathname.startsWith("/products")) return "Productos";
    if (pathname.startsWith("/stock")) return "Inventario";
    if (pathname.startsWith("/invoices")) return "Facturas";
    if (pathname.startsWith("/kitchen")) return "Cocina";
    if (pathname.startsWith("/suppliers")) return "Proveedores";
    if (pathname.startsWith("/purchase-entries")) return "Entradas de Compra";
    if (pathname.startsWith("/expenses")) return "Gastos";
    return "Laguna Escondida";
  };

  const title = getPageTitle();

  const handleSignOut = async () => {
    try {
      await fetch("/api/auth/signout", {
        method: "POST",
      });
      router.push("/signin");
      router.refresh();
    } catch (error) {
      console.error("Sign out error:", error);
    }
  };

  const handleBackToHome = () => {
    router.push("/home");
  };

  return (
    <header
      style={{
        backgroundColor: "var(--color-surface)",
        borderBottom: "1px solid var(--color-border)",
        padding: "1rem 1.5rem",
        position: "sticky",
        top: 0,
        zIndex: 50,
        boxShadow: "var(--shadow-md)",
      }}
    >
      <div
        style={{
          maxWidth: "80rem",
          margin: "0 auto",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
          <button
            onClick={handleBackToHome}
            style={{
              display: "flex",
              alignItems: "center",
              padding: "0.5rem",
              backgroundColor: "transparent",
              border: "none",
              borderRadius: "var(--radius-md)",
              cursor: "pointer",
              color: "var(--color-text-secondary)",
              transition: "all var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor =
                "var(--color-surface-hover)";
              e.currentTarget.style.color = "var(--color-primary)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "transparent";
              e.currentTarget.style.color = "var(--color-text-secondary)";
            }}
            title="Volver al Inicio"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              width="24"
              height="24"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M19 12H5M12 19l-7-7 7-7" />
            </svg>
          </button>
          <h1
            style={{
              fontSize: "1.5rem",
              fontWeight: "600",
              color: "var(--color-text-primary)",
              margin: 0,
            }}
          >
            {title}
          </h1>
        </div>

        <button
          onClick={handleSignOut}
          style={{
            padding: "0.5rem 1rem",
            backgroundColor: "var(--color-danger)",
            color: "white",
            border: "none",
            borderRadius: "var(--radius-md)",
            cursor: "pointer",
            fontSize: "0.875rem",
            fontWeight: "500",
            transition: "background-color var(--transition-normal)",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.backgroundColor = "var(--color-danger-hover)";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.backgroundColor = "var(--color-danger)";
          }}
        >
          Cerrar Sesión
        </button>
      </div>
    </header>
  );
}
