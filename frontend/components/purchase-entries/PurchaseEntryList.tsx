"use client";

import { useState, useMemo } from "react";
import type { PurchaseEntry } from "@/types/purchaseEntry";
import { Button, Input, Table } from "@/components/ui";

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

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString("es-CO", {
      year: "numeric",
      month: "short",
      day: "numeric",
    });
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
      <Input
        placeholder="Buscar por proveedor, referencia, notas..."
        value={searchTerm}
        onChange={(e) => setSearchTerm(e.target.value)}
        wrapperClassName=""
        style={{ marginBottom: "1.5rem" }}
        type="text"
      />

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

      {/* Entry Table */}
      {filteredEntries.length === 0 ? (
        <div style={{ textAlign: "center", padding: "3rem" }}>
          <p style={{ fontSize: "1.1rem", color: "var(--color-text-secondary)" }}>
            No hay entradas que coincidan con tu búsqueda.
          </p>
        </div>
      ) : (
        <Table>
          <thead>
            <tr>
              <th>Proveedor</th>
              <th>Fecha de Entrada</th>
              <th>Referencia</th>
              <th data-numeric>Total</th>
              <th style={{ textAlign: "right" }}>Acciones</th>
            </tr>
          </thead>
          <tbody>
            {filteredEntries.map((entry) => (
              <tr key={entry.id}>
                <td style={{ fontWeight: 600 }}>
                  {entry.supplier_name || "Proveedor desconocido"}
                </td>
                <td>{formatDate(entry.entry_date)}</td>
                <td>
                  {entry.invoice_reference ? (
                    <span
                      style={{
                        fontSize: "0.75rem",
                        fontFamily: "monospace",
                        color: "var(--color-text-muted)",
                      }}
                    >
                      {entry.invoice_reference}
                    </span>
                  ) : (
                    "—"
                  )}
                </td>
                <td data-numeric style={{ fontWeight: 600 }}>
                  {formatCurrency(parseFloat(entry.total_amount))}
                </td>
                <td style={{ textAlign: "right" }}>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => onViewDetail(entry)}
                  >
                    Ver detalles
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
}
