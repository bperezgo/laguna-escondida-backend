"use client";

import { useState, useMemo } from "react";
import type { PurchaseEntry } from "@/types/purchaseEntry";
import PurchaseEntryCard from "./PurchaseEntryCard";

interface PurchaseEntryListProps {
  entries: PurchaseEntry[];
  onViewDetail: (entry: PurchaseEntry) => void;
  isLoading?: boolean;
}

export default function PurchaseEntryList({
  entries,
  onViewDetail,
  isLoading = false,
}: PurchaseEntryListProps) {
  const [searchTerm, setSearchTerm] = useState("");

  const filteredEntries = useMemo(() => {
    if (!searchTerm) return entries;
    return entries.filter(
      (entry) =>
        entry.supplier_name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        entry.invoice_reference?.toLowerCase().includes(searchTerm.toLowerCase()) ||
        entry.notes?.toLowerCase().includes(searchTerm.toLowerCase())
    );
  }, [entries, searchTerm]);

  const totalAmount = useMemo(() => {
    return filteredEntries.reduce(
      (sum, entry) => sum + parseFloat(entry.total_amount),
      0
    );
  }, [filteredEntries]);

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat("es-CO", {
      style: "currency",
      currency: "COP",
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
  };

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
          No se encontraron entradas de compra para el período seleccionado.
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Search */}
      <div
        style={{
          marginBottom: "1.5rem",
        }}
      >
        <input
          type="text"
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          placeholder="Buscar por proveedor, referencia, notas..."
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

      {/* Results count and total */}
      <div
        style={{
          marginBottom: "1rem",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: "1rem",
        }}
      >
        <span
          style={{
            color: "var(--color-text-secondary)",
            fontSize: "0.9rem",
          }}
        >
          Mostrando {filteredEntries.length} de {entries.length} entrada
          {entries.length !== 1 ? "s" : ""}
        </span>
        <span
          style={{
            fontSize: "1.1rem",
            fontWeight: "bold",
            color: "var(--color-primary)",
          }}
        >
          Total: {formatCurrency(totalAmount)}
        </span>
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
