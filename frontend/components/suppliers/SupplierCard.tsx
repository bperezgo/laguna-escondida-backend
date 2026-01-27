"use client";

import type { Supplier } from "@/types/supplier";

interface SupplierCardProps {
  supplier: Supplier;
  onEdit: (supplier: Supplier) => void;
  onDelete: (id: string) => void;
  onViewCatalog: (supplier: Supplier) => void;
}

export default function SupplierCard({
  supplier,
  onEdit,
  onDelete,
  onViewCatalog,
}: SupplierCardProps) {
  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("es-CO", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  return (
    <div
      style={{
        border: "1px solid var(--color-border)",
        borderRadius: "var(--radius-md)",
        padding: "1.5rem",
        marginBottom: "1rem",
        backgroundColor: "var(--color-surface)",
        boxShadow: "var(--shadow-sm)",
        transition: "box-shadow var(--transition-normal)",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-md)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-sm)";
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
          marginBottom: "1rem",
        }}
      >
        <div style={{ flex: 1 }}>
          <h3
            style={{
              margin: "0 0 0.5rem 0",
              fontSize: "1.25rem",
              fontWeight: "bold",
              color: "var(--color-text-primary)",
            }}
          >
            {supplier.name}
          </h3>
          
          {supplier.contact_name && (
            <p
              style={{
                margin: "0 0 0.25rem 0",
                color: "var(--color-text-secondary)",
                fontSize: "0.9rem",
              }}
            >
              <strong>Contacto:</strong> {supplier.contact_name}
            </p>
          )}
          
          <div style={{ display: "flex", gap: "1.5rem", flexWrap: "wrap", marginTop: "0.5rem" }}>
            {supplier.phone && (
              <p
                style={{
                  margin: 0,
                  color: "var(--color-text-secondary)",
                  fontSize: "0.9rem",
                }}
              >
                <strong>Tel:</strong> {supplier.phone}
              </p>
            )}
            {supplier.email && (
              <p
                style={{
                  margin: 0,
                  color: "var(--color-text-secondary)",
                  fontSize: "0.9rem",
                }}
              >
                <strong>Email:</strong>{" "}
                <a
                  href={`mailto:${supplier.email}`}
                  style={{ color: "var(--color-primary)" }}
                >
                  {supplier.email}
                </a>
              </p>
            )}
          </div>
        </div>
        
        <div style={{ display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
          <button
            onClick={() => onViewCatalog(supplier)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "var(--color-secondary)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "background-color var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor =
                "var(--color-secondary-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-secondary)";
            }}
          >
            Catálogo
          </button>
          <button
            onClick={() => onEdit(supplier)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "var(--color-primary)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
              transition: "background-color var(--transition-normal)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor =
                "var(--color-primary-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary)";
            }}
          >
            Editar
          </button>
          <button
            onClick={() => onDelete(supplier.id)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "var(--color-danger)",
              color: "white",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "0.875rem",
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
            Eliminar
          </button>
        </div>
      </div>

      {/* Notes */}
      {supplier.notes && (
        <div
          style={{
            padding: "0.75rem",
            backgroundColor: "var(--color-bg)",
            borderRadius: "var(--radius-sm)",
            border: "1px solid var(--color-border)",
            marginBottom: "1rem",
          }}
        >
          <p
            style={{
              margin: 0,
              color: "var(--color-text-secondary)",
              fontSize: "0.875rem",
            }}
          >
            {supplier.notes}
          </p>
        </div>
      )}

      <div
        style={{
          paddingTop: "1rem",
          borderTop: "1px solid var(--color-border)",
        }}
      >
        <p
          style={{
            margin: "0.25rem 0",
            color: "var(--color-text-muted)",
            fontSize: "0.75rem",
          }}
        >
          Creado: {formatDate(supplier.created_at)}
        </p>
        <p
          style={{
            margin: "0.25rem 0",
            color: "var(--color-text-muted)",
            fontSize: "0.75rem",
          }}
        >
          Actualizado: {formatDate(supplier.updated_at)}
        </p>
      </div>
    </div>
  );
}
