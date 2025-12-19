"use client";

import type { Stock } from "@/types/stock";

interface StockCardProps {
  stock: Stock;
  productName?: string;
  onAdjust: (stock: Stock) => void;
  onDelete: (productId: string) => void;
}

export default function StockCard({
  stock,
  productName,
  onAdjust,
  onDelete,
}: StockCardProps) {
  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("en-US", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

  const getAmountColor = (amount: number) => {
    if (amount === 0) return "var(--color-danger)";
    if (amount < 10) return "var(--color-warning)";
    return "var(--color-success)";
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
            {productName || `Product ID: ${stock.product_id}`}
          </h3>
          <p
            style={{
              margin: "0 0 0.5rem 0",
              color: "var(--color-text-secondary)",
              fontSize: "0.9rem",
            }}
          >
            <strong>ID del Producto:</strong> {stock.product_id}
          </p>
        </div>
        <div style={{ display: "flex", gap: "0.5rem" }}>
          <button
            onClick={() => onAdjust(stock)}
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
              e.currentTarget.style.backgroundColor = "var(--color-primary-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-primary)";
            }}
          >
            Ajustar
          </button>
          <button
            onClick={() => onDelete(stock.product_id)}
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
              e.currentTarget.style.backgroundColor = "var(--color-danger-hover)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "var(--color-danger)";
            }}
          >
            Eliminar
          </button>
        </div>
      </div>

      <div
        style={{
          padding: "1rem",
          backgroundColor: "var(--color-bg)",
          borderRadius: "var(--radius-sm)",
          textAlign: "center",
          border: "1px solid var(--color-border)",
        }}
      >
        <p
          style={{
            margin: "0 0 0.25rem 0",
            color: "var(--color-text-muted)",
            fontSize: "0.85rem",
          }}
        >
          Inventario Actual
        </p>
        <p
          style={{
            margin: 0,
            fontSize: "2rem",
            fontWeight: "bold",
            color: getAmountColor(stock.amount),
          }}
        >
          {stock.amount}
        </p>
      </div>

      <div
        style={{
          marginTop: "1rem",
          paddingTop: "1rem",
          borderTop: "1px solid var(--color-border)",
        }}
      >
        <p style={{ margin: "0.25rem 0", color: "var(--color-text-muted)", fontSize: "0.75rem" }}>
          Creado: {formatDate(stock.created_at)}
        </p>
        <p style={{ margin: "0.25rem 0", color: "var(--color-text-muted)", fontSize: "0.75rem" }}>
          Actualizado: {formatDate(stock.updated_at)}
        </p>
      </div>
    </div>
  );
}
