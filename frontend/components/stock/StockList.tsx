"use client";

import { useState } from "react";
import type { Stock } from "@/types/stock";
import StockCard from "./StockCard";

interface StockListProps {
  stocks: Stock[];
  products?: Array<{ id: string; name: string }>;
  onAdjust: (stock: Stock) => void;
  onDelete: (productId: string) => void;
  isLoading?: boolean;
}

export default function StockList({
  stocks,
  products = [],
  onAdjust,
  onDelete,
  isLoading = false,
}: StockListProps) {
  const [searchTerm, setSearchTerm] = useState("");

  // Create a map of product IDs to product names
  const productMap = new Map(products.map((p) => [p.id, p.name]));

  // Filter stocks based on search term
  const filteredStocks = stocks.filter((stock) => {
    const productName = productMap.get(stock.product_id) || "";
    return (
      productName.toLowerCase().includes(searchTerm.toLowerCase()) ||
      stock.product_id.toLowerCase().includes(searchTerm.toLowerCase())
    );
  });

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          Cargando inventarios...
        </p>
      </div>
    );
  }

  if (stocks.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          No se encontraron inventarios. ¡Crea tu primer inventario!
        </p>
      </div>
    );
  }

  return (
    <div>
      <div
        style={{
          marginBottom: "2rem",
          padding: "1.5rem",
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
        }}
      >
        <div style={{ flex: "1", minWidth: "200px" }}>
          <label
            style={{
              display: "block",
              marginBottom: "0.5rem",
              fontWeight: "500",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
            }}
          >
            Buscar Inventarios
          </label>
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Buscar por nombre de producto o ID..."
            style={{
              width: "100%",
              padding: "0.75rem",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              fontSize: "1rem",
              boxSizing: "border-box",
              backgroundColor: "var(--color-bg)",
              color: "var(--color-text-primary)",
            }}
          />
        </div>
      </div>

      <div style={{ marginBottom: "1rem", color: "var(--color-text-secondary)", fontSize: "0.9rem" }}>
        Mostrando {filteredStocks.length} de {stocks.length} inventario
        {stocks.length !== 1 ? "s" : ""}
      </div>

      {filteredStocks.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay inventarios que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <div>
          {filteredStocks.map((stock) => (
            <StockCard
              key={stock.id}
              stock={stock}
              productName={productMap.get(stock.product_id)}
              onAdjust={onAdjust}
              onDelete={onDelete}
            />
          ))}
        </div>
      )}
    </div>
  );
}
