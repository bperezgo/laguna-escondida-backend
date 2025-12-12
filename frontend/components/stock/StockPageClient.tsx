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
        backgroundColor: "#f5f5f5",
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
                color: "#333",
              }}
            >
              Gestión de Inventario
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "#666",
                fontSize: "1rem",
              }}
            >
              Administra los niveles de inventario
            </p>
          </div>
          <button
            onClick={handleCreate}
            style={{
              padding: "0.75rem 1.5rem",
              backgroundColor: "#28a745",
              color: "white",
              border: "none",
              borderRadius: "4px",
              cursor: "pointer",
              fontSize: "1rem",
              fontWeight: "500",
              boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "#218838";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "#28a745";
            }}
          >
            + Crear Inventario
          </button>
        </div>

        {/* Error Message */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "#f8d7da",
              color: "#721c24",
              border: "1px solid #f5c6cb",
              borderRadius: "4px",
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
