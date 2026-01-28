"use client";

import { useState } from "react";
import { stockApi } from "@/lib/api/stock";
import type {
  Stock,
  CreateStockRequest,
  AddOrDecreaseStockRequest,
  BulkStockCreationOrUpdatingRequest,
} from "@/types/stock";
import type { Product } from "@/types/product";
import StockList from "@/components/stock/StockList";
import StockForm from "@/components/stock/StockForm";
import BulkStockTable from "@/components/stock/BulkStockTable";
import { PermissionGate } from "@/components/permissions";
import { PERMISSIONS } from "@/lib/permissions";

interface StockPageClientProps {
  initialStocks: Stock[];
  products: Product[];
}

export default function StockPageClient({
  initialStocks,
  products,
}: StockPageClientProps) {
  const [stocks, setStocks] = useState<Stock[]>(initialStocks);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [showForm, setShowForm] = useState<boolean>(false);
  const [adjustingStock, setAdjustingStock] = useState<Stock | null>(null);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [formMode, setFormMode] = useState<"create" | "adjust">("create");

  const loadStocks = async () => {
    try {
      setLoading(true);
      setError("");
      const fetchedStocks = await stockApi.getAll();
      setStocks(fetchedStocks);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to load stocks";
      setError(errorMessage);
      console.error("Error loading stocks:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setAdjustingStock(null);
    setFormMode("create");
    setShowForm(true);
  };

  const handleAdjust = (stock: Stock) => {
    setAdjustingStock(stock);
    setFormMode("adjust");
    setShowForm(true);
  };

  const handleFormSubmit = async (
    data: CreateStockRequest | AddOrDecreaseStockRequest
  ) => {
    try {
      setFormLoading(true);
      setError("");

      if (formMode === "adjust") {
        await stockApi.addOrDecrease(data as AddOrDecreaseStockRequest);
      } else {
        await stockApi.create(data as CreateStockRequest);
      }

      // Reload stocks list
      await loadStocks();
      setShowForm(false);
      setAdjustingStock(null);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to save stock";
      setError(errorMessage);
      console.error("Error saving stock:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleDelete = async (productId: string) => {
    if (!confirm("¿Estás seguro de que quieres eliminar este inventario?")) {
      return;
    }

    try {
      setError("");
      await stockApi.delete(productId);
      // Reload stocks list
      await loadStocks();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to delete stock";
      setError(errorMessage);
      console.error("Error deleting stock:", err);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setAdjustingStock(null);
    setError("");
  };

  const handleBulkSubmit = async (
    request: BulkStockCreationOrUpdatingRequest
  ) => {
    try {
      setError("");
      await stockApi.bulkCreateOrUpdate(request);
      // Reload stocks list
      await loadStocks();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Failed to update stocks";
      setError(errorMessage);
      throw err; // Re-throw so BulkStockTable can handle it
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        backgroundColor: "var(--color-bg)",
        padding: "2rem",
      }}
    >
      <div
        style={{
          maxWidth: "1200px",
          margin: "0 auto",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "2rem",
            flexWrap: "wrap",
            gap: "1rem",
          }}
        >
          <div>
            <h1
              style={{
                margin: 0,
                fontSize: "2rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Gestión de Inventario
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Administra los niveles de inventario
            </p>
          </div>
          <PermissionGate permission={PERMISSIONS.STOCK_CREATE}>
            <button
              onClick={handleCreate}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "var(--color-success)",
                color: "white",
                border: "none",
                borderRadius: "var(--radius-sm)",
                cursor: "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                boxShadow: "var(--shadow-sm)",
                transition: "background-color var(--transition-normal)",
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = "var(--color-success-hover)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = "var(--color-success)";
              }}
            >
              + Crear Inventario
            </button>
          </PermissionGate>
        </div>

        {/* Error Message */}
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

        {/* Stocks List */}
        <StockList
          stocks={stocks}
          products={products.map((p) => ({ id: p.id, name: p.name }))}
          onAdjust={handleAdjust}
          onDelete={handleDelete}
          isLoading={loading}
        />

        {/* Bulk Stock Update Table */}
        <BulkStockTable
          products={products}
          stocks={stocks}
          onSubmit={handleBulkSubmit}
          isLoading={loading}
        />

        {/* Stock Form Modal */}
        {showForm && (
          <StockForm
            stock={adjustingStock}
            products={products}
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
            mode={formMode}
          />
        )}
      </div>
    </div>
  );
}
