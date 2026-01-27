"use client";

import { useState } from "react";
import type { Supplier } from "@/types/supplier";
import SupplierCard from "./SupplierCard";

interface SupplierListProps {
  suppliers: Supplier[];
  onEdit: (supplier: Supplier) => void;
  onDelete: (id: string) => void;
  onViewCatalog: (supplier: Supplier) => void;
  isLoading?: boolean;
}

export default function SupplierList({
  suppliers,
  onEdit,
  onDelete,
  onViewCatalog,
  isLoading = false,
}: SupplierListProps) {
  const [searchTerm, setSearchTerm] = useState("");

  // Filter suppliers based on search term
  const filteredSuppliers = suppliers.filter((supplier) => {
    const search = searchTerm.toLowerCase();
    return (
      supplier.name.toLowerCase().includes(search) ||
      supplier.contact_name?.toLowerCase().includes(search) ||
      supplier.email?.toLowerCase().includes(search) ||
      supplier.phone?.includes(search)
    );
  });

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          Cargando proveedores...
        </p>
      </div>
    );
  }

  if (suppliers.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          No se encontraron proveedores. ¡Crea tu primer proveedor!
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Search */}
      <div
        style={{
          marginBottom: "2rem",
          padding: "1.5rem",
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
        }}
      >
        <div style={{ maxWidth: "400px" }}>
          <label
            style={{
              display: "block",
              marginBottom: "0.5rem",
              fontWeight: "500",
              fontSize: "0.875rem",
              color: "var(--color-text-primary)",
            }}
          >
            Buscar Proveedores
          </label>
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Buscar por nombre, contacto, email o teléfono..."
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

      {/* Results count */}
      <div
        style={{
          marginBottom: "1rem",
          color: "var(--color-text-secondary)",
          fontSize: "0.9rem",
        }}
      >
        Mostrando {filteredSuppliers.length} de {suppliers.length} proveedor
        {suppliers.length !== 1 ? "es" : ""}
      </div>

      {/* Supplier List */}
      {filteredSuppliers.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay proveedores que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <div>
          {filteredSuppliers.map((supplier) => (
            <SupplierCard
              key={supplier.id}
              supplier={supplier}
              onEdit={onEdit}
              onDelete={onDelete}
              onViewCatalog={onViewCatalog}
            />
          ))}
        </div>
      )}
    </div>
  );
}
