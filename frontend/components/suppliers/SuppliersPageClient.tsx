"use client";

import { useState } from "react";
import { suppliersApi } from "@/lib/api/suppliers";
import type {
  Supplier,
  CreateSupplierRequest,
  UpdateSupplierRequest,
} from "@/types/supplier";
import SupplierList from "./SupplierList";
import SupplierForm from "./SupplierForm";
import SupplierCatalogModal from "./SupplierCatalogModal";

interface SuppliersPageClientProps {
  initialSuppliers: Supplier[];
}

export default function SuppliersPageClient({
  initialSuppliers,
}: SuppliersPageClientProps) {
  const [suppliers, setSuppliers] = useState<Supplier[]>(initialSuppliers);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");
  const [showForm, setShowForm] = useState<boolean>(false);
  const [editingSupplier, setEditingSupplier] = useState<Supplier | null>(null);
  const [formLoading, setFormLoading] = useState<boolean>(false);
  const [catalogSupplier, setCatalogSupplier] = useState<Supplier | null>(null);

  const loadSuppliers = async () => {
    try {
      setLoading(true);
      setError("");
      const fetchedSuppliers = await suppliersApi.getAll();
      setSuppliers(fetchedSuppliers);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al cargar proveedores";
      setError(errorMessage);
      console.error("Error loading suppliers:", err);
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = () => {
    setEditingSupplier(null);
    setShowForm(true);
  };

  const handleEdit = (supplier: Supplier) => {
    setEditingSupplier(supplier);
    setShowForm(true);
  };

  const handleViewCatalog = (supplier: Supplier) => {
    setCatalogSupplier(supplier);
  };

  const handleFormSubmit = async (
    data: CreateSupplierRequest | UpdateSupplierRequest
  ) => {
    try {
      setFormLoading(true);
      setError("");

      if (editingSupplier) {
        await suppliersApi.update(
          editingSupplier.id,
          data as UpdateSupplierRequest
        );
      } else {
        await suppliersApi.create(data as CreateSupplierRequest);
      }

      await loadSuppliers();
      setShowForm(false);
      setEditingSupplier(null);
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al guardar proveedor";
      setError(errorMessage);
      console.error("Error saving supplier:", err);
    } finally {
      setFormLoading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("¿Estás seguro de que quieres eliminar este proveedor?")) {
      return;
    }

    try {
      setError("");
      await suppliersApi.delete(id);
      await loadSuppliers();
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : "Error al eliminar proveedor";
      setError(errorMessage);
      console.error("Error deleting supplier:", err);
    }
  };

  const handleCancel = () => {
    setShowForm(false);
    setEditingSupplier(null);
    setError("");
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
              Proveedores
            </h1>
            <p
              style={{
                margin: "0.5rem 0 0 0",
                color: "var(--color-text-secondary)",
                fontSize: "1rem",
              }}
            >
              Administra los proveedores y sus catálogos de productos
            </p>
          </div>
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
            + Nuevo Proveedor
          </button>
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

        {/* Suppliers List */}
        <SupplierList
          suppliers={suppliers}
          onEdit={handleEdit}
          onDelete={handleDelete}
          onViewCatalog={handleViewCatalog}
          isLoading={loading}
        />

        {/* Supplier Form Modal */}
        {showForm && (
          <SupplierForm
            supplier={editingSupplier}
            onSubmit={handleFormSubmit}
            onCancel={handleCancel}
            isLoading={formLoading}
          />
        )}

        {/* Supplier Catalog Modal */}
        {catalogSupplier && (
          <SupplierCatalogModal
            supplier={catalogSupplier}
            onClose={() => setCatalogSupplier(null)}
          />
        )}
      </div>
    </div>
  );
}
