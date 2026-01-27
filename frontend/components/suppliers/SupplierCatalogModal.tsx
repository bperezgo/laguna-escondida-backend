"use client";

import { useState, useEffect } from "react";
import type { Supplier } from "@/types/supplier";
import type { SupplierCatalogItem, AddProductToSupplierRequest } from "@/types/supplierCatalog";
import type { Product } from "@/types/product";
import { supplierCatalogApi } from "@/lib/api/supplierCatalog";
import { productsApi } from "@/lib/api/products";

interface SupplierCatalogModalProps {
  supplier: Supplier;
  onClose: () => void;
}

export default function SupplierCatalogModal({
  supplier,
  onClose,
}: SupplierCatalogModalProps) {
  const [catalogItems, setCatalogItems] = useState<SupplierCatalogItem[]>([]);
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showAddForm, setShowAddForm] = useState(false);
  const [addFormData, setAddFormData] = useState({
    product_id: "",
    unit_cost: "",
    supplier_sku: "",
  });
  const [addFormLoading, setAddFormLoading] = useState(false);

  useEffect(() => {
    loadData();
  }, [supplier.id]);

  const loadData = async () => {
    try {
      setLoading(true);
      setError("");
      const [catalogData, productsData] = await Promise.all([
        supplierCatalogApi.getProductsFromSupplier(supplier.id),
        productsApi.getAll(),
      ]);
      setCatalogItems(catalogData);
      setProducts(productsData);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al cargar datos");
    } finally {
      setLoading(false);
    }
  };

  const handleAddProduct = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!addFormData.product_id || !addFormData.unit_cost) return;

    try {
      setAddFormLoading(true);
      setError("");
      const data: AddProductToSupplierRequest = {
        product_id: addFormData.product_id,
        unit_cost: addFormData.unit_cost,
      };
      if (addFormData.supplier_sku.trim()) {
        data.supplier_sku = addFormData.supplier_sku.trim();
      }
      await supplierCatalogApi.addProductToSupplier(supplier.id, data);
      await loadData();
      setShowAddForm(false);
      setAddFormData({ product_id: "", unit_cost: "", supplier_sku: "" });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al agregar producto");
    } finally {
      setAddFormLoading(false);
    }
  };

  const handleRemoveProduct = async (productId: string) => {
    if (!confirm("¿Estás seguro de eliminar este producto del catálogo?")) return;

    try {
      setError("");
      await supplierCatalogApi.removeProductFromSupplier(supplier.id, productId);
      await loadData();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error al eliminar producto");
    }
  };

  const formatCurrency = (amount: string) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(parseFloat(amount));
  };

  // Get products not already in catalog
  const availableProducts = products.filter(
    (p) => !catalogItems.some((c) => c.product_id === p.id)
  );

  const inputStyle = {
    width: "100%",
    padding: "0.75rem",
    border: "1px solid var(--color-border)",
    borderRadius: "var(--radius-sm)",
    fontSize: "1rem",
    boxSizing: "border-box" as const,
    backgroundColor: "var(--color-bg)",
    color: "var(--color-text-primary)",
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "var(--color-overlay)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
        padding: "1rem",
      }}
    >
      <div
        style={{
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          padding: "2rem",
          maxWidth: "700px",
          width: "100%",
          maxHeight: "90vh",
          overflowY: "auto",
          boxShadow: "var(--shadow-xl)",
          border: "1px solid var(--color-border)",
        }}
      >
        {/* Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: "1.5rem",
          }}
        >
          <div>
            <h2
              style={{
                margin: 0,
                fontSize: "1.5rem",
                fontWeight: "bold",
                color: "var(--color-text-primary)",
              }}
            >
              Catálogo de {supplier.name}
            </h2>
            <p
              style={{
                margin: "0.25rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "0.875rem",
              }}
            >
              Productos que ofrece este proveedor
            </p>
          </div>
          <button
            onClick={onClose}
            style={{
              padding: "0.5rem",
              backgroundColor: "transparent",
              border: "none",
              cursor: "pointer",
              color: "var(--color-text-secondary)",
              fontSize: "1.5rem",
              lineHeight: 1,
            }}
          >
            ×
          </button>
        </div>

        {/* Error */}
        {error && (
          <div
            style={{
              padding: "1rem",
              backgroundColor: "var(--color-danger-light)",
              color: "var(--color-danger)",
              border: "1px solid var(--color-danger)",
              borderRadius: "var(--radius-sm)",
              marginBottom: "1rem",
            }}
          >
            {error}
          </div>
        )}

        {/* Add Product Button */}
        {!showAddForm && (
          <button
            onClick={() => setShowAddForm(true)}
            disabled={availableProducts.length === 0}
            style={{
              marginBottom: "1.5rem",
              padding: "0.75rem 1.5rem",
              backgroundColor:
                availableProducts.length === 0
                  ? "var(--color-surface-hover)"
                  : "var(--color-success)",
              color:
                availableProducts.length === 0
                  ? "var(--color-text-muted)"
                  : "white",
              border: "none",
              borderRadius: "var(--radius-sm)",
              cursor:
                availableProducts.length === 0 ? "not-allowed" : "pointer",
              fontSize: "1rem",
              fontWeight: "500",
            }}
          >
            + Agregar Producto al Catálogo
          </button>
        )}

        {/* Add Form */}
        {showAddForm && (
          <form
            onSubmit={handleAddProduct}
            style={{
              marginBottom: "1.5rem",
              padding: "1rem",
              backgroundColor: "var(--color-bg)",
              borderRadius: "var(--radius-sm)",
              border: "1px solid var(--color-border)",
            }}
          >
            <h3
              style={{
                margin: "0 0 1rem 0",
                fontSize: "1rem",
                fontWeight: "600",
                color: "var(--color-text-primary)",
              }}
            >
              Agregar Producto
            </h3>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: "2fr 1fr 1fr",
                gap: "1rem",
                marginBottom: "1rem",
              }}
            >
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Producto *
                </label>
                <select
                  value={addFormData.product_id}
                  onChange={(e) =>
                    setAddFormData((prev) => ({
                      ...prev,
                      product_id: e.target.value,
                    }))
                  }
                  style={inputStyle}
                  required
                >
                  <option value="">Seleccionar producto</option>
                  {availableProducts.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.sku})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  Costo Unitario *
                </label>
                <input
                  type="number"
                  value={addFormData.unit_cost}
                  onChange={(e) =>
                    setAddFormData((prev) => ({
                      ...prev,
                      unit_cost: e.target.value,
                    }))
                  }
                  style={inputStyle}
                  placeholder="0"
                  min="0"
                  step="0.01"
                  required
                />
              </div>
              <div>
                <label
                  style={{
                    display: "block",
                    marginBottom: "0.5rem",
                    fontSize: "0.875rem",
                    color: "var(--color-text-primary)",
                  }}
                >
                  SKU Proveedor
                </label>
                <input
                  type="text"
                  value={addFormData.supplier_sku}
                  onChange={(e) =>
                    setAddFormData((prev) => ({
                      ...prev,
                      supplier_sku: e.target.value,
                    }))
                  }
                  style={inputStyle}
                  placeholder="SKU opcional"
                />
              </div>
            </div>
            <div style={{ display: "flex", gap: "0.5rem" }}>
              <button
                type="submit"
                disabled={addFormLoading}
                style={{
                  padding: "0.5rem 1rem",
                  backgroundColor: "var(--color-success)",
                  color: "white",
                  border: "none",
                  borderRadius: "var(--radius-sm)",
                  cursor: addFormLoading ? "not-allowed" : "pointer",
                  fontSize: "0.875rem",
                }}
              >
                {addFormLoading ? "Agregando..." : "Agregar"}
              </button>
              <button
                type="button"
                onClick={() => {
                  setShowAddForm(false);
                  setAddFormData({ product_id: "", unit_cost: "", supplier_sku: "" });
                }}
                style={{
                  padding: "0.5rem 1rem",
                  backgroundColor: "var(--color-surface-hover)",
                  color: "var(--color-text-primary)",
                  border: "1px solid var(--color-border)",
                  borderRadius: "var(--radius-sm)",
                  cursor: "pointer",
                  fontSize: "0.875rem",
                }}
              >
                Cancelar
              </button>
            </div>
          </form>
        )}

        {/* Catalog Items */}
        {loading ? (
          <div style={{ textAlign: "center", padding: "2rem" }}>
            <p style={{ color: "var(--color-text-secondary)" }}>
              Cargando catálogo...
            </p>
          </div>
        ) : catalogItems.length === 0 ? (
          <div style={{ textAlign: "center", padding: "2rem" }}>
            <p style={{ color: "var(--color-text-secondary)" }}>
              Este proveedor aún no tiene productos en su catálogo.
            </p>
          </div>
        ) : (
          <div>
            <table
              style={{
                width: "100%",
                borderCollapse: "collapse",
              }}
            >
              <thead>
                <tr
                  style={{
                    borderBottom: "2px solid var(--color-border)",
                  }}
                >
                  <th
                    style={{
                      textAlign: "left",
                      padding: "0.75rem",
                      color: "var(--color-text-secondary)",
                      fontSize: "0.875rem",
                      fontWeight: "600",
                    }}
                  >
                    Producto
                  </th>
                  <th
                    style={{
                      textAlign: "left",
                      padding: "0.75rem",
                      color: "var(--color-text-secondary)",
                      fontSize: "0.875rem",
                      fontWeight: "600",
                    }}
                  >
                    SKU Proveedor
                  </th>
                  <th
                    style={{
                      textAlign: "right",
                      padding: "0.75rem",
                      color: "var(--color-text-secondary)",
                      fontSize: "0.875rem",
                      fontWeight: "600",
                    }}
                  >
                    Costo Unitario
                  </th>
                  <th
                    style={{
                      textAlign: "center",
                      padding: "0.75rem",
                      color: "var(--color-text-secondary)",
                      fontSize: "0.875rem",
                      fontWeight: "600",
                    }}
                  >
                    Acciones
                  </th>
                </tr>
              </thead>
              <tbody>
                {catalogItems.map((item) => (
                  <tr
                    key={item.id}
                    style={{
                      borderBottom: "1px solid var(--color-border)",
                    }}
                  >
                    <td
                      style={{
                        padding: "0.75rem",
                        color: "var(--color-text-primary)",
                      }}
                    >
                      {item.product_name || item.product_id}
                    </td>
                    <td
                      style={{
                        padding: "0.75rem",
                        color: "var(--color-text-secondary)",
                        fontFamily: "monospace",
                        fontSize: "0.875rem",
                      }}
                    >
                      {item.supplier_sku || "-"}
                    </td>
                    <td
                      style={{
                        padding: "0.75rem",
                        color: "var(--color-text-primary)",
                        textAlign: "right",
                        fontWeight: "600",
                      }}
                    >
                      {formatCurrency(item.unit_cost)}
                    </td>
                    <td
                      style={{
                        padding: "0.75rem",
                        textAlign: "center",
                      }}
                    >
                      <button
                        onClick={() => handleRemoveProduct(item.product_id)}
                        style={{
                          padding: "0.25rem 0.5rem",
                          backgroundColor: "var(--color-danger)",
                          color: "white",
                          border: "none",
                          borderRadius: "var(--radius-sm)",
                          cursor: "pointer",
                          fontSize: "0.75rem",
                        }}
                      >
                        Eliminar
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Close button */}
        <div
          style={{
            marginTop: "1.5rem",
            display: "flex",
            justifyContent: "flex-end",
          }}
        >
          <button
            onClick={onClose}
            style={{
              padding: "0.75rem 1.5rem",
              backgroundColor: "var(--color-surface-hover)",
              color: "var(--color-text-primary)",
              border: "1px solid var(--color-border)",
              borderRadius: "var(--radius-sm)",
              cursor: "pointer",
              fontSize: "1rem",
              fontWeight: "500",
            }}
          >
            Cerrar
          </button>
        </div>
      </div>
    </div>
  );
}
