"use client";

import { useState, useEffect } from "react";
import type {
  Stock,
  CreateStockRequest,
  AddOrDecreaseStockRequest,
} from "@/types/stock";
import type { Product } from "@/types/product";

interface StockFormProps {
  stock?: Stock | null;
  products: Product[];
  onSubmit: (
    data: CreateStockRequest | AddOrDecreaseStockRequest
  ) => Promise<void>;
  onCancel: () => void;
  isLoading?: boolean;
  mode?: "create" | "adjust";
}

export default function StockForm({
  stock,
  products,
  onSubmit,
  onCancel,
  isLoading = false,
  mode = "create",
}: StockFormProps) {
  const [formData, setFormData] = useState({
    product_id: stock?.product_id || "",
    amount: stock?.amount?.toString() || "",
    change: "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (stock) {
      setFormData({
        product_id: stock.product_id || "",
        amount: stock.amount?.toString() || "",
        change: "",
      });
    }
  }, [stock]);

  const validate = (): boolean => {
    const newErrors: Record<string, string> = {};

    // Product ID: required, uuid
    if (!formData.product_id.trim()) {
      newErrors.product_id = "El producto es requerido";
    }

    if (mode === "create") {
      // Amount: required
      if (!formData.amount.trim()) {
        newErrors.amount = "La cantidad es requerida";
      } else {
        const amount = parseInt(formData.amount);
        if (isNaN(amount)) {
          newErrors.amount = "La cantidad debe ser un número válido";
        }
      }
    } else {
      // Change: required
      if (!formData.change.trim()) {
        newErrors.change = "Change amount is required";
      } else {
        const change = parseInt(formData.change);
        if (isNaN(change) || change === 0) {
          newErrors.change = "El cambio debe ser un número diferente de cero";
        }
      }
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validate()) {
      return;
    }

    if (mode === "create") {
      const submitData: CreateStockRequest = {
        product_id: formData.product_id.trim(),
        amount: parseInt(formData.amount),
      };
      await onSubmit(submitData);
    } else {
      const submitData: AddOrDecreaseStockRequest = {
        product_id: formData.product_id.trim(),
        change: parseInt(formData.change),
      };
      await onSubmit(submitData);
    }
  };

  const handleChange = (field: string, value: string) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    // Clear error when user starts typing
    if (errors[field]) {
      setErrors((prev) => {
        const newErrors = { ...prev };
        delete newErrors[field];
        return newErrors;
      });
    }
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "rgba(0, 0, 0, 0.5)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
        padding: "1rem",
      }}
    >
      <div
        style={{
          backgroundColor: "white",
          borderRadius: "8px",
          padding: "2rem",
          maxWidth: "500px",
          width: "100%",
          maxHeight: "90vh",
          overflowY: "auto",
          boxShadow: "0 4px 6px rgba(0, 0, 0, 0.1)",
        }}
      >
        <h2
          style={{
            marginTop: 0,
            marginBottom: "1.5rem",
            fontSize: "1.5rem",
            fontWeight: "bold",
          }}
        >
          {mode === "adjust"
            ? "Ajustar Inventario"
            : stock
            ? "Editar Inventario"
            : "Crear Nuevo Inventario"}
        </h2>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: "1.5rem" }}>
            <label
              style={{
                display: "block",
                marginBottom: "0.5rem",
                fontWeight: "500",
                color: "#333",
              }}
            >
              Producto *
            </label>
            <select
              value={formData.product_id}
              onChange={(e) => handleChange("product_id", e.target.value)}
              disabled={mode === "adjust"}
              style={{
                width: "100%",
                padding: "0.75rem",
                border: `1px solid ${
                  errors.product_id ? "#dc3545" : "#ced4da"
                }`,
                borderRadius: "4px",
                fontSize: "1rem",
                boxSizing: "border-box",
                backgroundColor: mode === "adjust" ? "#f8f9fa" : "white",
              }}
            >
              <option value="">Selecciona un producto</option>
              {products.map((product) => (
                <option key={product.id} value={product.id}>
                  {product.name} ({product.sku})
                </option>
              ))}
            </select>
            {errors.product_id && (
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "#dc3545",
                  fontSize: "0.875rem",
                }}
              >
                {errors.product_id}
              </p>
            )}
          </div>

          {mode === "create" ? (
            <div style={{ marginBottom: "1.5rem" }}>
              <label
                style={{
                  display: "block",
                  marginBottom: "0.5rem",
                  fontWeight: "500",
                  color: "#333",
                }}
              >
                Cantidad Inicial *
              </label>
              <input
                type="number"
                value={formData.amount}
                onChange={(e) => handleChange("amount", e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  border: `1px solid ${errors.amount ? "#dc3545" : "#ced4da"}`,
                  borderRadius: "4px",
                  fontSize: "1rem",
                  boxSizing: "border-box",
                }}
                placeholder="Ingresa la cantidad inicial de inventario"
              />
              {errors.amount && (
                <p
                  style={{
                    margin: "0.25rem 0 0 0",
                    color: "#dc3545",
                    fontSize: "0.875rem",
                  }}
                >
                  {errors.amount}
                </p>
              )}
            </div>
          ) : (
            <div style={{ marginBottom: "1.5rem" }}>
              <label
                style={{
                  display: "block",
                  marginBottom: "0.5rem",
                  fontWeight: "500",
                  color: "#333",
                }}
              >
                Cantidad de Cambio *
              </label>
              <input
                type="number"
                value={formData.change}
                onChange={(e) => handleChange("change", e.target.value)}
                style={{
                  width: "100%",
                  padding: "0.75rem",
                  border: `1px solid ${errors.change ? "#dc3545" : "#ced4da"}`,
                  borderRadius: "4px",
                  fontSize: "1rem",
                  boxSizing: "border-box",
                }}
                placeholder="Ingresa positivo para agregar, negativo para disminuir"
              />
              <p
                style={{
                  margin: "0.25rem 0 0 0",
                  color: "#666",
                  fontSize: "0.875rem",
                }}
              >
                Usa números positivos para agregar inventario, números negativos
                para disminuir
              </p>
              {errors.change && (
                <p
                  style={{
                    margin: "0.25rem 0 0 0",
                    color: "#dc3545",
                    fontSize: "0.875rem",
                  }}
                >
                  {errors.change}
                </p>
              )}
            </div>
          )}

          <div
            style={{ display: "flex", gap: "1rem", justifyContent: "flex-end" }}
          >
            <button
              type="button"
              onClick={onCancel}
              disabled={isLoading}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "#6c757d",
                color: "white",
                border: "none",
                borderRadius: "4px",
                cursor: isLoading ? "not-allowed" : "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              Cancelar
            </button>
            <button
              type="submit"
              disabled={isLoading}
              style={{
                padding: "0.75rem 1.5rem",
                backgroundColor: "#28a745",
                color: "white",
                border: "none",
                borderRadius: "4px",
                cursor: isLoading ? "not-allowed" : "pointer",
                fontSize: "1rem",
                fontWeight: "500",
                opacity: isLoading ? 0.6 : 1,
              }}
            >
              {isLoading
                ? "Guardando..."
                : mode === "adjust"
                ? "Ajustar"
                : stock
                ? "Actualizar"
                : "Crear"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
