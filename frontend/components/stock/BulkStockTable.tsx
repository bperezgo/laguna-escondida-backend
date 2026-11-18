"use client";

import { useState, useEffect, useMemo } from "react";
import type { Product } from "@/types/product";
import type { Stock, BulkStockCreationOrUpdatingRequest } from "@/types/stock";
import { Button, Table } from "@/components/ui";

interface BulkStockTableProps {
  products: Product[];
  stocks: Stock[];
  onSubmit: (request: BulkStockCreationOrUpdatingRequest) => Promise<void>;
  isLoading?: boolean;
}

interface ProductStockRow {
  product_id: string;
  product_name: string;
  sku: string;
  currentAmount: number;
  newAmount: number;
}

export default function BulkStockTable({
  products,
  stocks,
  onSubmit,
  isLoading = false,
}: BulkStockTableProps) {
  // Create a map of product_id to stock amount using useMemo
  const stockMap = useMemo(() => {
    const map = new Map<string, number>();
    stocks.forEach((stock) => {
      map.set(stock.product_id, stock.amount);
    });
    return map;
  }, [stocks]);

  // Initialize rows with all products using useMemo
  const initialRows = useMemo((): ProductStockRow[] => {
    return products.map((product) => ({
      product_id: product.id,
      product_name: product.name,
      sku: product.sku,
      currentAmount: stockMap.get(product.id) || 0,
      newAmount: stockMap.get(product.id) || 0,
    }));
  }, [products, stockMap]);

  const [rows, setRows] = useState<ProductStockRow[]>(initialRows);
  const [originalRows, setOriginalRows] =
    useState<ProductStockRow[]>(initialRows);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>("");

  // Update rows when stocks or products change
  useEffect(() => {
    setRows(initialRows.map((row) => ({ ...row })));
    setOriginalRows(initialRows.map((row) => ({ ...row })));
  }, [initialRows]);

  const handleAmountChange = (productId: string, value: string) => {
    const numValue = value === "" ? 0 : parseInt(value, 10);
    if (isNaN(numValue) || numValue < 0) {
      return;
    }

    setRows((prevRows) =>
      prevRows.map((row) =>
        row.product_id === productId ? { ...row, newAmount: numValue } : row
      )
    );
    setError("");
  };

  const handleReset = () => {
    setRows(originalRows.map((row) => ({ ...row })));
    setError("");
  };

  const handleSubmit = async () => {
    try {
      setSubmitting(true);
      setError("");

      // Build bulk request with only items that have changed or have non-zero amounts
      const items = rows
        .filter(
          (row) => row.newAmount !== row.currentAmount || row.newAmount > 0
        )
        .map((row) => ({
          product_id: row.product_id,
          amount: row.newAmount,
        }));

      if (items.length === 0) {
        setError("No hay cambios para enviar");
        setSubmitting(false);
        return;
      }

      const request: BulkStockCreationOrUpdatingRequest = { items };
      await onSubmit(request);

      // Update original rows after successful submit
      setOriginalRows(rows.map((row) => ({ ...row })));
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to update stocks";
      setError(errorMessage);
      console.error("Error submitting bulk stock update:", err);
    } finally {
      setSubmitting(false);
    }
  };

  const hasChanges = rows.some((row) => row.newAmount !== row.currentAmount);

  return (
    <div
      style={{
        backgroundColor: "var(--color-surface)",
        borderRadius: "var(--radius-md)",
        padding: "2rem",
        boxShadow: "var(--shadow-sm)",
        marginTop: "2rem",
        border: "1px solid var(--color-border)",
      }}
    >
      <h2
        style={{
          margin: "0 0 1.5rem 0",
          fontSize: "1.5rem",
          fontWeight: "bold",
          color: "var(--color-text-primary)",
        }}
      >
        Actualización Masiva de Inventario
      </h2>
      <p
        style={{
          margin: "0 0 1.5rem 0",
          color: "var(--color-text-secondary)",
          fontSize: "0.9rem",
        }}
      >
        Actualiza las cantidades de inventario para todos los productos. Las
        cantidades actuales se muestran como referencia.
      </p>

      {error && (
        <div
          style={{
            padding: "1rem",
            backgroundColor: "var(--color-danger-light)",
            color: "var(--color-danger)",
            border: "1px solid var(--color-danger)",
            borderRadius: "var(--radius-sm)",
            marginBottom: "1.5rem",
          }}
        >
          <strong>Error:</strong> {error}
        </div>
      )}

      <div style={{ marginBottom: "1.5rem" }}>
        <Table style={{ minWidth: "600px" }}>
          <thead>
            <tr>
              <th>Nombre del Producto</th>
              <th>SKU</th>
              <th data-numeric>Cantidad Actual</th>
              <th data-numeric>Nueva Cantidad</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const hasChanged = row.newAmount !== row.currentAmount;
              return (
                <tr
                  key={row.product_id}
                  style={{
                    backgroundColor: hasChanged
                      ? "var(--color-warning-light)"
                      : undefined,
                  }}
                >
                  <td style={{ fontWeight: 500 }}>{row.product_name}</td>
                  <td
                    style={{
                      color: "var(--color-text-secondary)",
                      fontFamily: "var(--font-mono)",
                      fontSize: "0.85rem",
                    }}
                  >
                    {row.sku}
                  </td>
                  <td
                    data-numeric
                    style={{
                      color: "var(--color-text-secondary)",
                      fontWeight: 500,
                    }}
                  >
                    {row.currentAmount}
                  </td>
                  <td data-numeric>
                    <input
                      type="number"
                      min="0"
                      value={row.newAmount === 0 ? "" : row.newAmount}
                      onChange={(e) =>
                        handleAmountChange(row.product_id, e.target.value)
                      }
                      style={{
                        width: "120px",
                        maxWidth: "150px",
                        padding: "0.5rem",
                        border: `1px solid ${
                          hasChanged
                            ? "var(--color-warning)"
                            : "var(--color-border)"
                        }`,
                        borderRadius: "var(--radius-sm)",
                        fontSize: "0.95rem",
                        textAlign: "right",
                        fontFamily: "var(--font-mono)",
                        boxSizing: "border-box",
                        backgroundColor: hasChanged
                          ? "var(--color-surface)"
                          : "var(--color-bg)",
                        color: "var(--color-text-primary)",
                      }}
                      placeholder="0"
                    />
                  </td>
                </tr>
              );
            })}
          </tbody>
        </Table>
      </div>

      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          paddingTop: "1.5rem",
          borderTop: "1px solid var(--color-border)",
          gap: "1rem",
          flexWrap: "wrap",
        }}
      >
        <div
          style={{
            color: "var(--color-text-secondary)",
            fontSize: "0.9rem",
          }}
        >
          {hasChanges ? (
            <span style={{ color: "var(--color-warning)", fontWeight: "500" }}>
              Tienes cambios sin guardar
            </span>
          ) : (
            <span>No se realizaron cambios</span>
          )}
        </div>
        <div
          style={{
            display: "flex",
            gap: "1rem",
          }}
        >
          <Button
            variant="secondary"
            onClick={handleReset}
            disabled={submitting || !hasChanges}
          >
            Restablecer Valores
          </Button>
          <Button
            variant="primary"
            onClick={handleSubmit}
            disabled={submitting || !hasChanges || isLoading}
          >
            {submitting ? "Enviando..." : "Enviar Solicitud Masiva"}
          </Button>
        </div>
      </div>
    </div>
  );
}
