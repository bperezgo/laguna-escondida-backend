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
    if (amount === 0) return "#dc3545";
    if (amount < 10) return "#ffc107";
    return "#28a745";
  };

  return (
    <div
      style={{
        border: "1px solid #e0e0e0",
        borderRadius: "8px",
        padding: "1.5rem",
        marginBottom: "1rem",
        backgroundColor: "#fff",
        boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
        transition: "box-shadow 0.2s",
      }}
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = "0 4px 8px rgba(0,0,0,0.15)";
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "0 2px 4px rgba(0,0,0,0.1)";
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
              color: "#333",
            }}
          >
            {productName || `Product ID: ${stock.product_id}`}
          </h3>
          <p
            style={{
              margin: "0 0 0.5rem 0",
              color: "#666",
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
              backgroundColor: "#007bff",
              color: "white",
              border: "none",
              borderRadius: "4px",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#0056b3";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#007bff";
            }}
          >
            Ajustar
          </button>
          <button
            onClick={() => onDelete(stock.product_id)}
            style={{
              padding: "0.5rem 1rem",
              backgroundColor: "#dc3545",
              color: "white",
              border: "none",
              borderRadius: "4px",
              cursor: "pointer",
              fontSize: "0.875rem",
              fontWeight: "500",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#c82333";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#dc3545";
            }}
          >
            Eliminar
          </button>
        </div>
      </div>

      <div
        style={{
          padding: "1rem",
          backgroundColor: "#f8f9fa",
          borderRadius: "4px",
          textAlign: "center",
        }}
      >
        <p
          style={{
            margin: "0 0 0.25rem 0",
            color: "#666",
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
          borderTop: "1px solid #e0e0e0",
        }}
      >
        <p style={{ margin: "0.25rem 0", color: "#999", fontSize: "0.75rem" }}>
          Creado: {formatDate(stock.created_at)}
        </p>
        <p style={{ margin: "0.25rem 0", color: "#999", fontSize: "0.75rem" }}>
          Actualizado: {formatDate(stock.updated_at)}
        </p>
      </div>
    </div>
  );
}
