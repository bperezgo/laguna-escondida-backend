"use client";

import type { PurchaseEntry } from "@/types/purchaseEntry";

interface PurchaseEntryCardProps {
  entry: PurchaseEntry;
  onViewDetail: (entry: PurchaseEntry) => void;
}

export default function PurchaseEntryCard({
  entry,
  onViewDetail,
}: PurchaseEntryCardProps) {
  const formatCurrency = (amount: string) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(parseFloat(amount));
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("es-CO", {
      year: "numeric",
      month: "long",
      day: "numeric",
    });
  };

  const formatDateTime = (dateString: string) => {
    return new Date(dateString).toLocaleString("es-CO", {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
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
        cursor: "pointer",
      }}
      onClick={() => onViewDetail(entry)}
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-md)";
        e.currentTarget.style.borderColor = "var(--color-primary)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "var(--shadow-sm)";
        e.currentTarget.style.borderColor = "var(--color-border)";
      }}
    >
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "flex-start",
        }}
      >
        <div style={{ flex: 1 }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "1rem",
              marginBottom: "0.5rem",
            }}
          >
            <h3
              style={{
                margin: 0,
                fontSize: "1.1rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              {entry.supplier_name || "Proveedor desconocido"}
            </h3>
            {entry.invoice_reference && (
              <span
                style={{
                  padding: "0.25rem 0.5rem",
                  backgroundColor: "var(--color-bg)",
                  borderRadius: "var(--radius-sm)",
                  fontSize: "0.75rem",
                  fontFamily: "monospace",
                  color: "var(--color-text-secondary)",
                }}
              >
                Ref: {entry.invoice_reference}
              </span>
            )}
          </div>

          <p
            style={{
              margin: "0 0 0.5rem 0",
              color: "var(--color-text-secondary)",
              fontSize: "0.9rem",
            }}
          >
            <strong>Fecha de Entrada:</strong> {formatDate(entry.entry_date)}
          </p>

          {entry.notes && (
            <p
              style={{
                margin: 0,
                color: "var(--color-text-muted)",
                fontSize: "0.875rem",
                fontStyle: "italic",
              }}
            >
              {entry.notes}
            </p>
          )}
        </div>

        <div style={{ textAlign: "right" }}>
          <p
            style={{
              margin: "0 0 0.25rem 0",
              color: "var(--color-text-muted)",
              fontSize: "0.75rem",
            }}
          >
            Total
          </p>
          <p
            style={{
              margin: 0,
              fontSize: "1.5rem",
              fontWeight: "bold",
              color: "var(--color-success)",
            }}
          >
            {formatCurrency(entry.total_amount)}
          </p>
        </div>
      </div>

      <div
        style={{
          marginTop: "1rem",
          paddingTop: "1rem",
          borderTop: "1px solid var(--color-border)",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <p
          style={{
            margin: 0,
            color: "var(--color-text-muted)",
            fontSize: "0.75rem",
          }}
        >
          Registrado: {formatDateTime(entry.created_at)}
        </p>
        <span
          style={{
            color: "var(--color-primary)",
            fontSize: "0.875rem",
            fontWeight: "500",
          }}
        >
          Ver detalles →
        </span>
      </div>
    </div>
  );
}
