"use client";

import { useRouter } from "next/navigation";

export default function HomePage() {
  const router = useRouter();

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
          </p>

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
            <a
              href="/orders"
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
                e.currentTarget.style.borderColor = "var(--color-primary)";
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
                Órdenes
              </h2>
              <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
                Administra las órdenes de los clientes
              </p>
            </a>

            <a
              href="/products"
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
                e.currentTarget.style.borderColor = "var(--color-primary)";
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
                Productos
              </h2>
              <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
                Ver y editar productos
              </p>
            </a>

            <a
              href="/stock"
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
                e.currentTarget.style.borderColor = "var(--color-primary)";
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
                Inventario
              </h2>
              <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
                Administrar inventario
              </p>
            </a>

            <a
              href="/invoices"
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
                e.currentTarget.style.borderColor = "var(--color-primary)";
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
                Facturas
              </h2>
              <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
                Ver facturas
              </p>
            </a>

            <a
              href="/kitchen"
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
                e.currentTarget.style.borderColor = "var(--color-secondary)";
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
                🍳 Cocina
              </h2>
              <p style={{ color: "var(--color-text-secondary)", margin: 0 }}>
                Ver comandas en tiempo real
              </p>
            </a>
          </div>

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
