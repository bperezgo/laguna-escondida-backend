"use client";

import { useState } from "react";
import type { PurchaseEntry } from "@/types/purchaseEntry";
import type { Supplier } from "@/types/supplier";
import PurchaseEntryCard from "./PurchaseEntryCard";

interface PurchaseEntryListProps {
  entries: PurchaseEntry[];
  suppliers: Supplier[];
  onViewDetail: (entry: PurchaseEntry) => void;
  isLoading?: boolean;
}

export default function PurchaseEntryList({
  entries,
  suppliers,
  onViewDetail,
  isLoading = false,
}: PurchaseEntryListProps) {
  const [filterSupplier, setFilterSupplier] = useState("");
  const [searchTerm, setSearchTerm] = useState("");

  // Filter entries
  const filteredEntries = entries.filter((entry) => {
    const matchesSupplier =
      !filterSupplier || entry.supplier_id === filterSupplier;
    const matchesSearch =
      !searchTerm ||
      entry.supplier_name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
      entry.invoice_reference?.toLowerCase().includes(searchTerm.toLowerCase()) ||
      entry.notes?.toLowerCase().includes(searchTerm.toLowerCase());
    return matchesSupplier && matchesSearch;
  });

  if (isLoading) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          Cargando entradas de compra...
        </p>
      </div>
    );
  }

  if (entries.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "3rem" }}>
        <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
          No se encontraron entradas de compra. ¡Registra tu primera entrada!
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Filters */}
      <div
        style={{
          marginBottom: "2rem",
          padding: "1.5rem",
          backgroundColor: "var(--color-surface)",
          borderRadius: "var(--radius-md)",
          border: "1px solid var(--color-border)",
          display: "flex",
          gap: "1rem",
          flexWrap: "wrap",
          alignItems: "flex-end",
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
            Buscar
          </label>
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Buscar por proveedor, referencia o notas..."
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
            Filtrar por Proveedor
          </label>
          <select
            value={filterSupplier}
            onChange={(e) => setFilterSupplier(e.target.value)}
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
          >
            <option value="">Todos los Proveedores</option>
            {suppliers.map((supplier) => (
              <option key={supplier.id} value={supplier.id}>
                {supplier.name}
              </option>
            ))}
          </select>
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
        Mostrando {filteredEntries.length} de {entries.length} entrada
        {entries.length !== 1 ? "s" : ""}
      </div>

      {/* Entry List */}
      {filteredEntries.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay entradas que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <div>
          {filteredEntries.map((entry) => (
            <PurchaseEntryCard
              key={entry.id}
              entry={entry}
              onViewDetail={onViewDetail}
            />
          ))}
        </div>
      )}
    </div>
  );
}
