"use client";

import { useState } from "react";
import type { Stock } from "@/types/stock";
import { Button, Input, Table, Badge } from "@/components/ui";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

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

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("es-CO", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
  };

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
      <div style={{ maxWidth: 400, marginBottom: "1.5rem" }}>
        <Input
          label="Buscar Inventarios"
          placeholder="Buscar por nombre de producto o ID..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
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
        <Table>
          <thead>
            <tr>
              <th>Producto</th>
              <th>ID del Producto</th>
              <th data-numeric>Inventario</th>
              <th>Actualizado</th>
              <th style={{ textAlign: "right" }}>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {filteredStocks.map((stock) => (
              <tr key={stock.product_id}>
                <td style={{ fontWeight: 600 }}>
                  {productMap.get(stock.product_id) || `Producto ${stock.product_id}`}
                </td>
                <td
                  style={{
                    fontFamily: "var(--font-mono)",
                    color: "var(--color-text-muted)",
                    fontSize: "0.75rem",
                  }}
                >
                  {stock.product_id}
                </td>
                <td data-numeric>
                  <Badge
                    tone={
                      stock.amount === 0
                        ? "danger"
                        : stock.amount < 10
                          ? "warning"
                          : "success"
                    }
                  >
                    {stock.amount}
                  </Badge>
                </td>
                <td>{formatDate(stock.updated_at)}</td>
                <td style={{ textAlign: "right" }}>
                  <div style={{ display: "flex", gap: "0.5rem", justifyContent: "flex-end" }}>
                    <PermissionGate permission={PERMISSIONS.STOCK_UPDATE}>
                      <Button variant="primary" size="sm" onClick={() => onAdjust(stock)}>
                        Ajustar
                      </Button>
                    </PermissionGate>
                    <PermissionGate permission={PERMISSIONS.STOCK_DELETE}>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => onDelete(stock.product_id)}
                      >
                        Eliminar
                      </Button>
                    </PermissionGate>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
